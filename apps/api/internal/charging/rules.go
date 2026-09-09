package charging

import (
	"context"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---- pile status aggregation

// AggregatePileStatus derives charge_piles.status from the connector states
// (connector 0 = the whole pile, counted like any other):
//
//	any Faulted                                        → faulted
//	else any Charging / SuspendedEV / SuspendedEVSE / Finishing → charging
//	else any Available / Preparing / Reserved          → available
//	else (only Unavailable, or no connectors)          → offline
func AggregatePileStatus(connectors []Connector) string {
	anyCharging, anyAvailable := false, false
	for _, c := range connectors {
		switch c.Status {
		case ConnFaulted:
			return PileFaulted
		case ConnCharging, ConnSuspendedEV, ConnSuspendedEVSE, ConnFinishing:
			anyCharging = true
		case ConnAvailable, ConnPreparing, ConnReserved:
			anyAvailable = true
		}
	}
	switch {
	case anyCharging:
		return PileCharging
	case anyAvailable:
		return PileAvailable
	}
	return PileOffline
}

// ValidConnectorStatus reports whether s is an OCPP 1.6 ChargePointStatus.
func ValidConnectorStatus(s string) bool {
	switch s {
	case ConnAvailable, ConnPreparing, ConnCharging, ConnSuspendedEV, ConnSuspendedEVSE,
		ConnFinishing, ConnReserved, ConnUnavailable, ConnFaulted:
		return true
	}
	return false
}

// CanRemoteStart reports whether a connector may accept a RemoteStartTransaction.
func CanRemoteStart(status string) bool { return status == ConnAvailable || status == ConnPreparing }

// ---- pile-side metering vs BMS cross check

// CrossCheck is the result of comparing the pile's meter with the vehicle's BMS.
type CrossCheck struct {
	BmsKwhEst    *float64 // (soc_end − soc_start) / 100 × battery_kwh, clamped ≥ 0; nil when a SOC is missing
	DeviationPct *float64 // |kwh − est| / kwh × 100; nil when kwh ≤ 0 or est is nil
}

// Compare implements the cross check. kwh is the pile-side energy of the session.
func Compare(kwh float64, socStart, socEnd *float64, batteryKwh float64) CrossCheck {
	var cc CrossCheck
	if socStart == nil || socEnd == nil || batteryKwh <= 0 {
		return cc
	}
	est := math.Max(0, (*socEnd-*socStart)/100*batteryKwh)
	est = round3(est)
	cc.BmsKwhEst = &est
	if kwh > 0 {
		d := round2(math.Abs(kwh-est) / kwh * 100)
		cc.DeviationPct = &d
	}
	return cc
}

// NeedsReview is the verdict at StopTransaction: a session without a bound
// vehicle, or whose pile/BMS deviation exceeds DeviationThresholdPct, goes to
// the review queue instead of being billed automatically.
func NeedsReview(vehicleBound bool, deviationPct *float64) (bool, string) {
	if !vehicleBound {
		return true, "未能绑定车辆"
	}
	if deviationPct != nil && *deviationPct > DeviationThresholdPct {
		return true, "桩侧计量与 BMS 估算偏差 " + trimFloat(*deviationPct) + "% 超过 " + trimFloat(DeviationThresholdPct) + "%"
	}
	return false, ""
}

// SessionKwh converts meter readings (Wh) to the session energy in kWh (≥ 0).
func SessionKwh(meterStart, meterStop float64) float64 {
	return round3(math.Max(0, (meterStop-meterStart)/1000))
}

// ---- vehicle binding

// NearbyVehicle is a candidate for location binding.
type NearbyVehicle struct {
	VehicleID uuid.UUID
	DistanceM float64
	LastSeen  time.Time
}

// BindCandidates gathers everything the binding rule may use.
type BindCandidates struct {
	Preset     *uuid.UUID      // vehicle preset by remote start (highest priority)
	Nearby     []NearbyVehicle // vehicles reporting recently; the rule filters by radius/window itself
	RecentTrip *uuid.UUID      // vehicle of the card holder's most recent trip that ended within the window
	UserKnown  bool            // card resolved to a user (decides card vs none when no vehicle)
}

// PickVehicle applies the binding priority:
//
//	① preset vehicle (remote start)                     → manual
//	② nearest vehicle seen within BindTelemetryWindow and ≤ BindRadiusM → location
//	③ vehicle of the user's trip ended within BindRecentTripWindow      → recent_trip
//	④ none: "card" when the card holder is known, else "none"
func PickVehicle(c BindCandidates, now time.Time) (*uuid.UUID, string) {
	if c.Preset != nil && *c.Preset != uuid.Nil {
		v := *c.Preset
		return &v, BindManual
	}
	var best *NearbyVehicle
	for i := range c.Nearby {
		n := &c.Nearby[i]
		if n.DistanceM > BindRadiusM || now.Sub(n.LastSeen) > BindTelemetryWindow {
			continue
		}
		if best == nil || n.DistanceM < best.DistanceM || (n.DistanceM == best.DistanceM && n.LastSeen.After(best.LastSeen)) {
			best = n
		}
	}
	if best != nil {
		v := best.VehicleID
		return &v, BindLocation
	}
	if c.RecentTrip != nil && *c.RecentTrip != uuid.Nil {
		v := *c.RecentTrip
		return &v, BindRecentTrip
	}
	if c.UserKnown {
		return nil, BindCard
	}
	return nil, BindNone
}

// DistanceM is the haversine distance in metres.
func DistanceM(lng1, lat1, lng2, lat2 float64) float64 {
	const r = 6371000.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

// ---- meter curve sampling

// Thin keeps at most max points, always including the first and the last.
func Thin(pts []MeterValue, max int) []MeterValue {
	n := len(pts)
	if max < 2 || n <= max {
		return pts
	}
	out := make([]MeterValue, 0, max)
	step := float64(n-1) / float64(max-1)
	last := -1
	for i := 0; i < max; i++ {
		idx := int(math.Round(float64(i) * step))
		if idx >= n {
			idx = n - 1
		}
		if idx == last {
			continue
		}
		out = append(out, pts[idx])
		last = idx
	}
	return out
}

// WithKwh fills MeterValue.Kwh relative to meterStart (Wh).
func WithKwh(pts []MeterValue, meterStart float64) []MeterValue {
	for i := range pts {
		if pts[i].Wh != nil {
			k := round3(math.Max(0, (*pts[i].Wh-meterStart)/1000))
			pts[i].Kwh = &k
		}
	}
	return pts
}

// ---- pile gateway (implemented by internal/ocpp, injected at wiring time)

// PileGateway is what the HTTP side needs from the OCPP server.
type PileGateway interface {
	// Online reports whether the pile has a live OCPP connection.
	Online(pileID uuid.UUID) bool
	// RemoteStart sends RemoteStartTransaction and returns the pile's status (Accepted|Rejected).
	RemoteStart(ctx context.Context, pileID uuid.UUID, connectorID int, idTag string) (string, error)
	// RemoteStop sends RemoteStopTransaction for the pile's transactionId.
	RemoteStop(ctx context.Context, pileID uuid.UUID, ocppTxID int) (string, error)
}

var (
	gatewayMu sync.RWMutex
	gateway   PileGateway
)

// SetGateway registers the OCPP server (ocpp.New calls it; tests may set a fake).
func SetGateway(g PileGateway) {
	gatewayMu.Lock()
	gateway = g
	gatewayMu.Unlock()
}

// Gateway returns the registered gateway (nil when the OCPP server is not running).
func Gateway() PileGateway {
	gatewayMu.RLock()
	defer gatewayMu.RUnlock()
	return gateway
}

// ---- remote-start presets (vehicle chosen by the operator, consumed by StartTransaction)

type pendingKey struct {
	pileID      uuid.UUID
	connectorID int
}

type pendingStart struct {
	vehicleID uuid.UUID
	userID    *uuid.UUID
	expires   time.Time
}

var (
	pendingMu     sync.Mutex
	pendingStarts = map[pendingKey]pendingStart{}
)

// SetPendingVehicle remembers the vehicle (and the operator) a remote start is for.
func SetPendingVehicle(pileID uuid.UUID, connectorID int, vehicleID uuid.UUID, userID *uuid.UUID) {
	pendingMu.Lock()
	pendingStarts[pendingKey{pileID, connectorID}] = pendingStart{vehicleID: vehicleID, userID: userID, expires: time.Now().Add(PendingVehicleTTL)}
	pendingMu.Unlock()
}

// ClearPendingVehicle drops a preset (rejected remote start).
func ClearPendingVehicle(pileID uuid.UUID, connectorID int) {
	pendingMu.Lock()
	delete(pendingStarts, pendingKey{pileID, connectorID})
	pendingMu.Unlock()
}

// TakePendingVehicle consumes the preset for the connector, if any and not expired.
func TakePendingVehicle(pileID uuid.UUID, connectorID int) (vehicleID *uuid.UUID, userID *uuid.UUID) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	k := pendingKey{pileID, connectorID}
	p, ok := pendingStarts[k]
	if !ok {
		return nil, nil
	}
	delete(pendingStarts, k)
	if time.Now().After(p.expires) {
		return nil, nil
	}
	v := p.vehicleID
	return &v, p.userID
}

// ---- small helpers

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// trimFloat renders v with at most two decimals and no trailing zeros.
func trimFloat(v float64) string { return strconv.FormatFloat(round2(v), 'f', -1, 64) }
