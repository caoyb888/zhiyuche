package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/internal/ocpp"
)

const (
	batteryKwh    = 60.0 // 比亚迪 e6 (matches the created vehicle)
	lowSOC        = 30.0 // below this an idle vehicle heads for a charger
	fullSOC       = 90.0 // charging stops here
	chargePowerKw = 60.0 // simulated fast pile
	idleReportGap = time.Minute
)

type phase int

const (
	phaseIdle phase = iota
	phaseDriving
	phaseToCharger
	phaseWaiting // parked at a pile, waiting for a free connector (or for a remote start to kick in)
	phaseCharging
)

func (p phase) String() string {
	switch p {
	case phaseIdle:
		return "idle"
	case phaseDriving:
		return "driving"
	case phaseToCharger:
		return "to_charger"
	case phaseWaiting:
		return "waiting"
	case phaseCharging:
		return "charging"
	}
	return "?"
}

var destNames = []string{
	"大明湖", "趵突泉", "千佛山", "济南西站", "山东大学中心校区", "泉城广场", "奥体中心", "遥墙机场",
	"高新区管委会", "历下区政府", "省立医院", "洪家楼", "济南东站", "华山湖", "章丘明水", "长清大学城",
}

// simVehicle is the per-vehicle state machine.
type simVehicle struct {
	asset *VehicleAsset
	phase phase

	pos     LngLat
	heading float64
	speed   float64 // km/h
	cruise  float64 // km/h target while driving
	soc     float64
	odo     float64

	route          *Route
	travelled      float64
	overspeedTicks int

	driver      *DriverAsset
	dest        string
	applyNo     string
	tripStarted bool
	signOn      bool

	lastIdleSent time.Time
	nextTripAt   time.Time

	// charging through OCPP: the pile the vehicle is parked at and the gun it uses
	pile *simPile
	conn *simConnector
}

// Sim drives all vehicles one tick at a time. mu serialises the tick loop and
// the OCPP callbacks (RemoteStart/RemoteStop arrive on the piles' reader goroutines).
type Sim struct {
	cfg      *Config
	c        *client
	st       *State
	log      *slog.Logger
	rng      *rand.Rand
	purposes []string

	mu        sync.Mutex
	vehicles  []*simVehicle
	piles     []*simPile
	driverIdx int
	legacyWarned bool
}

func newSim(cfg *Config, c *client, st *State, log *slog.Logger, rng *rand.Rand, purposes []string) *Sim {
	s := &Sim{cfg: cfg, c: c, st: st, log: log, rng: rng, purposes: purposes}
	now := time.Now()
	for i, va := range st.Vehicles {
		v := &simVehicle{asset: va, pos: LngLat{va.Lng, va.Lat}, soc: va.SOC, odo: va.OdometerKm, heading: rng.Float64() * 360}
		if i < cfg.LowSOC {
			v.soc = round(15+rng.Float64()*10, 1) // forced to need a charge right away (demo / e2e)
		}
		v.nextTripAt = now.Add(s.idleDelay())
		s.vehicles = append(s.vehicles, v)
	}
	for _, pa := range st.Piles {
		s.piles = append(s.piles, newSimPile(pa, log))
	}
	return s
}

// startOCPP connects every pile to the central system (background goroutines).
func (s *Sim) startOCPP(ctx context.Context, base string) {
	for _, p := range s.piles {
		p := p
		p.client = newOCPPClient(base, p.asset.Code, s.log)
		p.client.handle = func(action string, payload json.RawMessage) (any, *ocpp.CallErr) { return s.handleCall(p, action, payload) }
		p.client.onConnected = p.sendStatuses
		go p.client.run(ctx)
	}
}

// ocppEnabled reports whether piles talk OCPP at all (--no-ocpp disables it).
func (s *Sim) ocppEnabled() bool { return !s.cfg.NoOCPP && len(s.piles) > 0 && s.piles[0].client != nil }

// idleDelay is how long a parked vehicle waits before the next trip (real time, shortened by speedup).
func (s *Sim) idleDelay() time.Duration {
	d := 30*time.Second + time.Duration(s.rng.Int63n(int64(150*time.Second)))
	return time.Duration(float64(d) / s.cfg.Speedup)
}

// persist copies the volatile state back into the state file structs.
func (s *Sim) persist() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.vehicles {
		v.asset.Lng, v.asset.Lat = round(v.pos.Lng, 6), round(v.pos.Lat, 6)
		v.asset.SOC, v.asset.OdometerKm = round(v.soc, 1), round(v.odo, 1)
	}
	for _, p := range s.piles {
		p.persist()
	}
}

// tick advances every vehicle by one reporting interval (scaled by speedup).
// force makes idle vehicles report immediately (used by --once).
func (s *Sim) tick(ctx context.Context, force bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	dt := time.Duration(float64(s.cfg.Interval) * s.cfg.Speedup)
	for _, v := range s.vehicles {
		if ctx.Err() != nil {
			return
		}
		s.step(ctx, v, now, dt, force)
	}
}

func (s *Sim) step(ctx context.Context, v *simVehicle, now time.Time, dt time.Duration, force bool) {
	hours := dt.Hours()
	switch v.phase {
	case phaseIdle:
		if !force && v.soc < lowSOC && len(s.piles) > 0 {
			s.goCharge(v)
			return
		}
		if !force && now.After(v.nextTripAt) {
			s.startTrip(ctx, v, now)
			return
		}
		if force || now.Sub(v.lastIdleSent) >= idleReportGap {
			s.send(ctx, v, s.point(v, now, false, false))
			v.lastIdleSent = now
		}
	case phaseDriving, phaseToCharger:
		arrived := s.advance(v, hours)
		s.send(ctx, v, s.point(v, now, true, false))
		if arrived {
			if v.phase == phaseDriving {
				s.endTrip(ctx, v, now)
			} else {
				s.arriveAtPile(ctx, v, now)
			}
		}
	case phaseWaiting:
		s.tryStart(ctx, v, now)
		if v.phase == phaseWaiting && now.Sub(v.lastIdleSent) >= idleReportGap {
			s.send(ctx, v, s.point(v, now, false, false))
			v.lastIdleSent = now
		}
	case phaseCharging:
		if v.conn != nil {
			s.chargeTick(ctx, v, now, hours)
			return
		}
		// legacy path: no OCPP, the vehicle only reports charging=true until full
		v.soc = math.Min(100, v.soc+chargeGainPct(chargePowerKw, hours, batteryKwh))
		charging := v.soc < fullSOC
		s.send(ctx, v, s.point(v, now, false, charging))
		if !charging {
			v.phase = phaseIdle
			v.pile = nil
			v.lastIdleSent = now
			v.nextTripAt = now.Add(s.idleDelay())
			s.log.Info("charging finished (telemetry only)", "plate", v.asset.PlateNo, "soc", round(v.soc, 1))
		}
	}
}

// arriveAtPile parks the vehicle and starts (or queues for) a session.
func (s *Sim) arriveAtPile(ctx context.Context, v *simVehicle, now time.Time) {
	s.log.Info("arrived at charger", "plate", v.asset.PlateNo, "pile", v.pile.asset.Code, "soc", round(v.soc, 1))
	v.phase = phaseWaiting
	v.lastIdleSent = now
	s.tryStart(ctx, v, now)
}

// tryStart starts an OCPP session when the pile is online and a gun is free;
// without OCPP it falls back to the telemetry-only charge.
func (s *Sim) tryStart(ctx context.Context, v *simVehicle, now time.Time) {
	p := v.pile
	if !s.ocppEnabled() || p == nil || !p.online() {
		if !s.legacyWarned {
			s.log.Warn("OCPP unavailable, charging with telemetry only", "plate", v.asset.PlateNo)
			s.legacyWarned = true
		}
		if v.conn != nil {
			v.conn.vehicle, v.conn.pendingIDTag = nil, ""
			v.conn = nil
		}
		v.phase = phaseCharging
		return
	}
	conn, idTag := v.conn, ""
	if conn != nil && conn.pendingIDTag != "" { // reserved by RemoteStartTransaction
		idTag = conn.pendingIDTag
	} else {
		conn = p.freeConnector()
		if conn == nil {
			if now.Sub(v.lastIdleSent) >= idleReportGap {
				s.log.Info("all connectors busy, waiting", "plate", v.asset.PlateNo, "pile", p.asset.Code)
			}
			return
		}
		if d := s.nextDriver(); d != nil {
			idTag = d.CardUID
		}
	}
	if idTag == "" {
		s.log.Warn("no driver card for charging; telemetry only", "plate", v.asset.PlateNo)
		v.phase = phaseCharging
		return
	}
	if !s.startSession(ctx, v, p, conn, idTag, now) {
		v.conn = nil
		v.phase = phaseCharging // central system refused: charge without a session
	}
}

func (s *Sim) nextDriver() *DriverAsset {
	if len(s.st.Drivers) == 0 {
		return nil
	}
	d := s.st.Drivers[s.driverIdx%len(s.st.Drivers)]
	s.driverIdx++
	return d
}

// goCharge heads for the nearest pile with a free gun (any pile when all are busy).
func (s *Sim) goCharge(v *simVehicle) {
	var best *simPile
	bestD := math.Inf(1)
	pick := func(onlyFree bool) {
		for _, p := range s.piles {
			if onlyFree && (!p.online() || p.freeConnector() == nil) {
				continue
			}
			if d := haversineKm(v.pos, LngLat{p.asset.Lng, p.asset.Lat}); d < bestD {
				best, bestD = p, d
			}
		}
	}
	if s.ocppEnabled() {
		pick(true)
	}
	if best == nil {
		pick(false)
	}
	v.pile, v.conn = best, nil
	v.route = buildRoute(s.rng, v.pos, LngLat{best.asset.Lng, best.asset.Lat}, 2, 0.4)
	v.travelled, v.speed, v.cruise = 0, 0, 30+s.rng.Float64()*20
	v.signOn, v.driver = false, nil
	v.phase = phaseToCharger
	s.log.Info("low battery, heading to charger", "plate", v.asset.PlateNo, "soc", round(v.soc, 1), "pile", best.asset.Code, "km", round(v.route.TotalKm(), 1))
}

func (s *Sim) startTrip(ctx context.Context, v *simVehicle, now time.Time) {
	dest := randomPointWithin(s.rng, s.cfg.Center, s.cfg.RadiusKm)
	v.dest = destNames[s.rng.Intn(len(destNames))]
	v.route = buildRoute(s.rng, v.pos, dest, 3+s.rng.Intn(4), 0.8) // 3–6 途经点
	v.travelled, v.speed, v.cruise = 0, 0, 30+s.rng.Float64()*30
	v.driver = s.nextDriver()
	v.tripStarted, v.signOn, v.applyNo = false, false, ""
	mode := "telemetry-only"
	if !s.cfg.TelemetryOnly && v.driver != nil {
		if s.openTrip(ctx, v, now, dest) {
			mode = "trip"
		}
	}
	v.phase = phaseDriving
	driver := ""
	if v.driver != nil {
		driver = v.driver.Name
	}
	s.log.Info("trip started", "plate", v.asset.PlateNo, "driver", driver, "dest", v.dest,
		"route_km", round(v.route.TotalKm(), 1), "soc", round(v.soc, 1), "mode", mode, "apply_no", v.applyNo)
}

// openTrip files and approves the request as the admin, then sends
// trip_start as the gateway. Returns true when a real trip is open.
func (s *Sim) openTrip(ctx context.Context, v *simVehicle, now time.Time, dest LngLat) bool {
	tripType := "official"
	if s.rng.Float64() >= 0.7 {
		tripType = "daily"
	}
	start := now.Add(time.Minute)
	end := start.Add(time.Hour + time.Duration(s.rng.Int63n(int64(2*time.Hour))))
	body := map[string]any{
		"applicant_id": v.driver.ID, "trip_type": tripType,
		"purpose_code":   s.purposes[s.rng.Intn(len(s.purposes))],
		"purpose_detail": "模拟用车：前往" + v.dest,
		"planned_start":  start, "planned_end": end,
		"destination": v.dest, "dest_lng": round(dest.Lng, 6), "dest_lat": round(dest.Lat, 6),
		"planned_km": round(haversineKm(v.pos, dest)*1.3, 1), "vehicle_id": v.asset.ID,
	}
	var ap struct {
		ID      string `json:"id"`
		ApplyNo string `json:"apply_no"`
		Status  string `json:"status"`
	}
	if err := s.c.do(ctx, http.MethodPost, "/approvals", body, &ap); err != nil {
		s.degrade("create approval", err)
		return false
	}
	for i := 0; i < 2 && ap.Status != "approved"; i++ {
		var next struct {
			Status string `json:"status"`
		}
		if err := s.c.do(ctx, http.MethodPost, "/approvals/"+ap.ID+"/approve", map[string]string{"remark": "模拟器自动审批"}, &next); err != nil {
			s.degrade("approve", err)
			break
		}
		ap.Status = next.Status
	}
	ev := map[string]any{"type": "trip_start", "trip_start": map[string]any{
		"ts": now, "card_uid": v.driver.CardUID, "approval_no": ap.ApplyNo,
		"lng": round(v.pos.Lng, 6), "lat": round(v.pos.Lat, 6), "odometer_km": round(v.odo, 1), "soc": round(v.soc, 1),
	}}
	var res struct {
		TripID   string `json:"trip_id"`
		TripNo   string `json:"trip_no"`
		TripType string `json:"trip_type"`
		SignOn   bool   `json:"sign_on"`
	}
	if err := s.c.doDevice(ctx, v.asset.Serial, v.asset.APIKey, http.MethodPost, "/ingest/events", ev, &res); err != nil {
		s.degrade("trip_start", err)
		return false
	}
	v.tripStarted, v.signOn, v.applyNo = true, res.SignOn, ap.ApplyNo
	s.log.Info("trip opened on device", "plate", v.asset.PlateNo, "trip_no", res.TripNo, "trip_type", res.TripType, "sign_on", res.SignOn)
	return true
}

// degrade logs a failed approval/trip call and, when the module is simply not
// there yet (501 / route not found), switches the whole run to telemetry-only.
func (s *Sim) degrade(step string, err error) {
	var ae *apiError
	if errors.As(err, &ae) && ae.isUnavailable() {
		if !s.cfg.TelemetryOnly {
			s.log.Warn("approval/trip API not available, falling back to telemetry-only", "step", step, "err", err)
			s.cfg.TelemetryOnly = true
		}
		return
	}
	s.log.Warn("trip workflow call failed, driving without a trip", "step", step, "err", err)
}

func (s *Sim) endTrip(ctx context.Context, v *simVehicle, now time.Time) {
	if v.tripStarted {
		ev := map[string]any{"type": "trip_end", "trip_end": map[string]any{
			"ts": now, "lng": round(v.pos.Lng, 6), "lat": round(v.pos.Lat, 6), "odometer_km": round(v.odo, 1), "soc": round(v.soc, 1),
		}}
		if err := s.c.doDevice(ctx, v.asset.Serial, v.asset.APIKey, http.MethodPost, "/ingest/events", ev, nil); err != nil {
			s.log.Warn("trip_end rejected", "plate", v.asset.PlateNo, "err", err)
		}
	}
	s.log.Info("trip ended", "plate", v.asset.PlateNo, "dest", v.dest, "km", round(v.route.TotalKm(), 1), "soc", round(v.soc, 1), "odometer", round(v.odo, 1))
	v.tripStarted, v.signOn, v.driver, v.route, v.speed = false, false, nil, nil, 0
	v.phase = phaseIdle
	v.lastIdleSent = now
	v.nextTripAt = now.Add(s.idleDelay())
}

// advance moves the vehicle along its route for hours of simulated time and
// returns true on arrival. Speed follows the cruise target with occasional
// harsh accelerations/brakes and short bursts to 90 km/h.
func (s *Sim) advance(v *simVehicle, hours float64) bool {
	switch {
	case v.overspeedTicks > 0:
		v.overspeedTicks--
		v.speed = 90
	case s.rng.Float64() < 0.02:
		v.overspeedTicks = 2
		v.speed = 90
		s.log.Debug("overspeed burst", "plate", v.asset.PlateNo)
	case s.rng.Float64() < 0.05:
		if s.rng.Intn(2) == 0 {
			v.speed = math.Min(v.speed+25, 85)
			s.log.Debug("harsh acceleration", "plate", v.asset.PlateNo, "speed", round(v.speed, 1))
		} else {
			v.speed = math.Max(v.speed-25, 5)
			s.log.Debug("harsh brake", "plate", v.asset.PlateNo, "speed", round(v.speed, 1))
		}
	default:
		if v.speed < v.cruise {
			v.speed = math.Min(v.speed+8, v.cruise)
		} else {
			v.speed = math.Max(v.speed-8, v.cruise)
		}
	}
	remaining := v.route.TotalKm() - v.travelled
	dist := math.Min(v.speed*hours, remaining)
	v.travelled += dist
	pos, heading, done := v.route.At(v.travelled)
	v.pos, v.heading = pos, heading
	v.odo += dist
	v.soc = math.Max(0, v.soc-socDropPct(dist, batteryKwh))
	if done {
		v.speed = 0
	}
	return done
}

// telemetryPoint mirrors the contract TelemetryPoint.
type telemetryPoint struct {
	TS         time.Time `json:"ts"`
	Lng        float64   `json:"lng"`
	Lat        float64   `json:"lat"`
	Speed      float64   `json:"speed"`
	Heading    float64   `json:"heading"`
	SOC        float64   `json:"soc"`
	SOH        float64   `json:"soh"`
	CellTemp   float64   `json:"cell_temp"`
	MotorTemp  float64   `json:"motor_temp"`
	OdometerKm float64   `json:"odometer_km"`
	AccOn      bool      `json:"acc_on"`
	Locked     bool      `json:"locked"`
	SignOn     bool      `json:"sign_on"`
	Charging   bool      `json:"charging"`
}

func (s *Sim) point(v *simVehicle, now time.Time, moving, charging bool) telemetryPoint {
	cell := 26 + s.rng.Float64()*4
	if charging {
		cell += 6
	}
	return telemetryPoint{
		TS: now, Lng: round(v.pos.Lng, 6), Lat: round(v.pos.Lat, 6),
		Speed: round(v.speed, 1), Heading: round(v.heading, 1),
		SOC: round(v.soc, 1), SOH: 98.5, CellTemp: round(cell, 1), MotorTemp: round(38+v.speed/2, 1),
		OdometerKm: round(v.odo, 1), AccOn: moving, Locked: !moving,
		SignOn: v.signOn && moving && v.tripStarted, Charging: charging,
	}
}

// send posts one point as the vehicle's gateway; a 401 means the stored key
// is stale, so the key is rotated through the admin API and saved.
func (s *Sim) send(ctx context.Context, v *simVehicle, p telemetryPoint) {
	body := map[string]any{"points": []telemetryPoint{p}}
	var res struct {
		Accepted int     `json:"accepted"`
		TripID   *string `json:"trip_id"`
	}
	err := s.c.doDevice(ctx, v.asset.Serial, v.asset.APIKey, http.MethodPost, "/ingest/telemetry", body, &res)
	if err == nil {
		s.log.Debug("telemetry sent", "plate", v.asset.PlateNo, "phase", v.phase.String(), "speed", p.Speed, "soc", p.SOC)
		return
	}
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized && v.asset.DeviceID != "" {
		var out apiDevice
		if rerr := s.c.do(ctx, http.MethodPost, "/assets/devices/"+v.asset.DeviceID+"/rotate-key", nil, &out); rerr == nil && out.APIKey != "" {
			v.asset.APIKey = out.APIKey
			_ = s.st.save(s.cfg.StatePath)
			s.log.Warn("device key was rejected, rotated a new one", "serial", v.asset.Serial)
			return
		}
	}
	msg := err.Error()
	if strings.Contains(msg, "context canceled") {
		return
	}
	s.log.Warn("telemetry rejected", "plate", v.asset.PlateNo, "err", msg)
}
