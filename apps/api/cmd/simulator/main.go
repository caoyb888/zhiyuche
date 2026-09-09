// Command simulator emulates VIG-100E gateways and driver behaviour against
// the REST API for demos and integration testing. As the tenant admin it
// creates (or reuses) vehicles, gateways, drivers, NFC cards and charge piles,
// then runs a per-vehicle state machine — idle → apply/approve → trip_start →
// drive a polyline route → trip_end → idle, charging when the SOC is low —
// posting telemetry through the device-authenticated /ingest endpoints.
// Standard library only; state (incl. device api keys) is kept in a JSON file.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Config holds the command line / environment settings.
type Config struct {
	API           string
	Username      string
	Password      string
	Vehicles      int
	Interval      time.Duration
	Speedup       float64
	Center        LngLat
	RadiusKm      float64
	StatePath     string
	TelemetryOnly bool
	Once          bool
	Verbose       bool
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
	fs.StringVar(&cfg.Username, "username", envOr("SIM_USERNAME", "chenhua_admin"), "tenant admin username [SIM_USERNAME]")
	fs.StringVar(&cfg.Password, "password", envOr("SIM_PASSWORD", "Admin@123456"), "tenant admin password [SIM_PASSWORD]")
	fs.IntVar(&cfg.Vehicles, "vehicles", envInt("SIM_VEHICLES", 6), "number of simulated vehicles [SIM_VEHICLES]")
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
	log.Info("simulator starting", "api", cfg.API, "vehicles", cfg.Vehicles, "interval", cfg.Interval,
		"speedup", cfg.Speedup, "center", fmt.Sprintf("%.4f,%.4f", cfg.Center.Lng, cfg.Center.Lat),
		"radius_km", cfg.RadiusKm, "state", cfg.StatePath, "telemetry_only", cfg.TelemetryOnly, "once", cfg.Once)

	st, err := loadState(cfg.StatePath)
	if err != nil {
		return fmt.Errorf("load state %s: %w", cfg.StatePath, err)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	c := newClient(cfg.API, cfg.Username, cfg.Password, log)
	if err := c.login(ctx); err != nil {
		return fmt.Errorf("login as %s: %w", cfg.Username, err)
	}
	log.Info("logged in", "username", cfg.Username)

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

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down, saving state", "path", cfg.StatePath)
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
