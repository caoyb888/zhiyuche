// Command simulator emulates VIG-100E gateways and driver behaviour against
// the REST API for demos and integration testing. As the tenant admin it
// creates (or reuses) vehicles, gateways, drivers, NFC cards and charge piles,
// then runs a per-vehicle state machine — idle → apply/approve → trip_start →
// drive a polyline route → trip_end → idle, charging when the SOC is low —
// posting telemetry through the device-authenticated /ingest endpoints.
//
// The charge piles it provisions are also emulated as OCPP 1.6J charge points
// (gorilla/websocket clients of the api's OCPP listener): a low vehicle drives
// to the nearest free gun, the pile runs Preparing → Authorize →
// StartTransaction → MeterValues → StopTransaction → Available, and remote
// start/stop from the web work against it. Without OCPP (--no-ocpp or the
// listener being down) charging degrades to the old telemetry-only behaviour.
// State (incl. device api keys and pile meters) is kept in a JSON file.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Config holds the command line / environment settings.
type Config struct {
	API           string
	OCPP          string // ws://host:port of the OCPP listener ("" → derived from API)
	NoOCPP        bool
	Username      string
	Password      string
	Tenant        string
	Vehicles      int
	LowSOC        int // vehicles forced to start with a low SOC (need charging right away)
	Interval      time.Duration
	Speedup       float64
	Center        LngLat
	RadiusKm      float64
	StatePath     string
	TelemetryOnly bool
	Once          bool
	Verbose       bool
}

const defaultOCPPPort = "20081"

// deriveOCPP turns the API base URL into the OCPP listener URL (same host, port 20081).
func deriveOCPP(api string) string {
	u, err := url.Parse(api)
	if err != nil || u.Host == "" {
		return "ws://localhost:" + defaultOCPPPort
	}
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	return "ws://" + net.JoinHostPort(host, defaultOCPPPort)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// parseFlags reads flags with environment-variable defaults (SIM_*).
func parseFlags(args []string) (*Config, error) {
	fs := flag.NewFlagSet("zhiyuche-simulator", flag.ContinueOnError)
	cfg := &Config{}
	var center string
	fs.StringVar(&cfg.API, "api", envOr("SIM_API", "http://localhost:20080/api/v1"), "API base URL [SIM_API]")
	fs.StringVar(&cfg.OCPP, "ocpp", envOr("SIM_OCPP", ""), "OCPP listener ws://host:port (default: host of --api, port 20081) [SIM_OCPP]")
	fs.BoolVar(&cfg.NoOCPP, "no-ocpp", envBool("SIM_NO_OCPP", false), "do not emulate piles over OCPP; charge with telemetry only [SIM_NO_OCPP]")
	fs.StringVar(&cfg.Username, "username", envOr("SIM_USERNAME", "chenhua_admin"), "tenant admin username [SIM_USERNAME]")
	fs.StringVar(&cfg.Password, "password", envOr("SIM_PASSWORD", "Admin@123456"), "tenant admin password [SIM_PASSWORD]")
	fs.StringVar(&cfg.Tenant, "tenant", envOr("SIM_TENANT", "chenhua"), "tenant code sent with the login (empty = let the API resolve it) [SIM_TENANT]")
	fs.IntVar(&cfg.Vehicles, "vehicles", envInt("SIM_VEHICLES", 6), "number of simulated vehicles [SIM_VEHICLES]")
	fs.IntVar(&cfg.LowSOC, "low-soc", envInt("SIM_LOW_SOC", 0), "force the first N vehicles to start with 15-25% SOC so they charge right away [SIM_LOW_SOC]")
	fs.DurationVar(&cfg.Interval, "interval", envDuration("SIM_INTERVAL", 5*time.Second), "telemetry reporting interval [SIM_INTERVAL]")
	fs.Float64Var(&cfg.Speedup, "speedup", envFloat("SIM_SPEEDUP", 1), "simulated-time multiplier [SIM_SPEEDUP]")
	fs.StringVar(&center, "center", envOr("SIM_CENTER", "117.12,36.65"), "map center lng,lat (default 济南) [SIM_CENTER]")
	fs.Float64Var(&cfg.RadiusKm, "radius-km", envFloat("SIM_RADIUS_KM", 8), "activity radius in km [SIM_RADIUS_KM]")
	fs.StringVar(&cfg.StatePath, "state", envOr("SIM_STATE", "./simulator-state.json"), "state file [SIM_STATE]")
	fs.BoolVar(&cfg.TelemetryOnly, "telemetry-only", envBool("SIM_TELEMETRY_ONLY", false), "no approvals/trips, telemetry only [SIM_TELEMETRY_ONLY]")
	fs.BoolVar(&cfg.Once, "once", envBool("SIM_ONCE", false), "provision, report one round and exit [SIM_ONCE]")
	fs.BoolVar(&cfg.Verbose, "verbose", envBool("SIM_VERBOSE", false), "debug logging (every telemetry point) [SIM_VERBOSE]")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	c, err := parseLngLat(center)
	if err != nil {
		return nil, fmt.Errorf("--center: %w", err)
	}
	cfg.Center = c
	if cfg.Vehicles < 1 || cfg.Vehicles > 500 {
		return nil, fmt.Errorf("--vehicles must be 1..500")
	}
	if cfg.Interval < 200*time.Millisecond {
		return nil, fmt.Errorf("--interval must be at least 200ms")
	}
	if cfg.Speedup <= 0 {
		return nil, fmt.Errorf("--speedup must be positive")
	}
	if cfg.RadiusKm <= 0 {
		return nil, fmt.Errorf("--radius-km must be positive")
	}
	if cfg.LowSOC < 0 || cfg.LowSOC > cfg.Vehicles {
		return nil, fmt.Errorf("--low-soc must be 0..%d", cfg.Vehicles)
	}
	if cfg.OCPP == "" {
		cfg.OCPP = deriveOCPP(cfg.API)
	}
	cfg.OCPP = strings.TrimRight(cfg.OCPP, "/")
	if !strings.HasPrefix(cfg.OCPP, "ws://") && !strings.HasPrefix(cfg.OCPP, "wss://") {
		return nil, fmt.Errorf("--ocpp must start with ws:// or wss://")
	}
	return cfg, nil
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(2)
	}
	level := slog.LevelInfo
	if cfg.Verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, log); err != nil {
		log.Error("simulator failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *Config, log *slog.Logger) error {
	log.Info("simulator starting", "api", cfg.API, "ocpp", cfg.OCPP, "no_ocpp", cfg.NoOCPP, "vehicles", cfg.Vehicles, "low_soc", cfg.LowSOC,
		"interval", cfg.Interval, "speedup", cfg.Speedup, "center", fmt.Sprintf("%.4f,%.4f", cfg.Center.Lng, cfg.Center.Lat),
		"radius_km", cfg.RadiusKm, "state", cfg.StatePath, "telemetry_only", cfg.TelemetryOnly, "once", cfg.Once)

	st, err := loadState(cfg.StatePath)
	if err != nil {
		return fmt.Errorf("load state %s: %w", cfg.StatePath, err)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	c := newClient(cfg.API, cfg.Username, cfg.Password, cfg.Tenant, log)
	if err := c.login(ctx); err != nil {
		return fmt.Errorf("login as %s: %w", cfg.Username, err)
	}
	log.Info("logged in", "username", cfg.Username, "tenant", cfg.Tenant)

	prov := &provisioner{cfg: cfg, c: c, st: st, log: log, rng: rng}
	if err := prov.ensureAll(ctx); err != nil {
		return fmt.Errorf("provision: %w", err)
	}
	if err := st.save(cfg.StatePath); err != nil {
		log.Warn("save state failed", "err", err)
	}
	log.Info("assets ready", "vehicles", len(st.Vehicles), "drivers", len(st.Drivers), "piles", len(st.Piles), "purposes", len(prov.purposes))

	sim := newSim(cfg, c, st, log, rng, prov.purposes)
	if cfg.Once {
		sim.tick(ctx, true)
		sim.persist()
		log.Info("one round done")
		return st.save(cfg.StatePath)
	}

	// piles go online as OCPP charge points; their context outlives the main
	// one so running sessions can be stopped cleanly on shutdown
	ocppCtx, stopOCPP := context.WithCancel(context.Background())
	defer stopOCPP()
	if cfg.NoOCPP {
		log.Warn("OCPP disabled (--no-ocpp): piles stay offline, charging is telemetry-only")
	} else if len(st.Piles) == 0 {
		log.Warn("no charge piles: OCPP not started")
	} else {
		sim.startOCPP(ocppCtx, cfg.OCPP)
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down: closing charging sessions, saving state", "path", cfg.StatePath)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			sim.shutdownSessions(shutdownCtx)
			cancel()
			stopOCPP()
			time.Sleep(300 * time.Millisecond) // let the close frames go out
			sim.persist()
			return st.save(cfg.StatePath)
		case <-ticker.C:
			sim.tick(ctx, false)
			n++
			if n%12 == 0 {
				sim.persist()
				if err := st.save(cfg.StatePath); err != nil {
					log.Warn("save state failed", "err", err)
				}
			}
		}
	}
}
