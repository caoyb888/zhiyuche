package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/internal/ocpp"
)

const (
	chargeEfficiency = 0.97 // the battery keeps 97 % of what the pile meters
	meterFaultFactor = 1.12 // a "drifting" meter over-registers → pile/BMS deviation > 5 % → review queue
	meterFaultRate   = 0.25 // share of sessions with a drifting meter
	chargeVoltage    = 380.0
	defaultPileKw    = 60.0
	defaultConnCount = 2
)

// simConnector is one gun of a simulated pile.
type simConnector struct {
	id           int
	vehicle      *simVehicle // charging or reserved by a remote start
	txID         int
	idTag        string
	meterWh      float64 // cumulative energy register
	meterFactor  float64 // 1.0 or meterFaultFactor for the running session
	stopReason   string  // set by RemoteStopTransaction; consumed on the next tick
	pendingIDTag string  // set by RemoteStartTransaction; the session starts on the next tick
	targetSOC    float64 // the session ends (reason Local) at this SOC: fullSOC when self-initiated, 100 when remotely started
}

func (c *simConnector) free() bool { return c.vehicle == nil }

// simPile is a charge point: its connectors and its OCPP connection.
type simPile struct {
	asset      *PileAsset
	powerKw    float64
	client     *ocppClient
	connectors []*simConnector
	log        *slog.Logger
}

func newSimPile(pa *PileAsset, log *slog.Logger) *simPile {
	p := &simPile{asset: pa, powerKw: pa.PowerKw, log: log.With("pile", pa.Code)}
	if p.powerKw <= 0 {
		p.powerKw = defaultPileKw
	}
	n := pa.ConnectorCount
	if n <= 0 {
		n = defaultConnCount
	}
	for i := 1; i <= n; i++ {
		c := &simConnector{id: i, meterFactor: 1}
		if len(pa.MeterWh) >= i {
			c.meterWh = pa.MeterWh[i-1]
		}
		p.connectors = append(p.connectors, c)
	}
	return p
}

func (p *simPile) online() bool { return p.client != nil && p.client.isConnected() }

func (p *simPile) freeConnector() *simConnector {
	for _, c := range p.connectors {
		if c.free() {
			return c
		}
	}
	return nil
}

func (p *simPile) connector(id int) *simConnector {
	for _, c := range p.connectors {
		if c.id == id {
			return c
		}
	}
	return nil
}

func (p *simPile) connectorByTx(txID int) *simConnector {
	for _, c := range p.connectors {
		if c.vehicle != nil && c.txID == txID {
			return c
		}
	}
	return nil
}

// persist copies the meters back into the state file struct.
func (p *simPile) persist() {
	p.asset.MeterWh = make([]float64, len(p.connectors))
	for i, c := range p.connectors {
		p.asset.MeterWh[i] = round(c.meterWh, 1)
	}
}

// status sends a StatusNotification (fire and forget).
func (p *simPile) status(ctx context.Context, connectorID int, status string) {
	if !p.online() {
		return
	}
	req := ocpp.StatusNotificationReq{ConnectorId: connectorID, ErrorCode: "NoError", Status: status, Timestamp: strp(ocpp.FormatTime(time.Now()))}
	if _, err := p.client.call(ctx, "StatusNotification", req); err != nil && ctx.Err() == nil {
		p.log.Warn("StatusNotification failed", "connector", connectorID, "status", status, "err", err)
	}
}

// sendStatuses announces every connector after (re)connecting.
func (p *simPile) sendStatuses(ctx context.Context) {
	p.status(ctx, 0, "Available")
	for _, c := range p.connectors {
		st := "Available"
		if c.vehicle != nil && c.txID != 0 {
			st = "Charging"
		}
		p.status(ctx, c.id, st)
	}
}

// meterValues reports one sample of the running session.
func (p *simPile) meterValues(ctx context.Context, c *simConnector, now time.Time, soc, powerKw float64) {
	if !p.online() || c.txID == 0 {
		return
	}
	txID := c.txID
	req := ocpp.MeterValuesReq{ConnectorId: c.id, TransactionId: &txID, MeterValue: []ocpp.MeterValueEntry{{
		Timestamp: ocpp.FormatTime(now),
		SampledValue: []ocpp.SampledValue{
			{Value: strconv.FormatFloat(c.meterWh, 'f', 1, 64), Measurand: ocpp.MeasurandEnergy, Unit: "Wh"},
			{Value: strconv.FormatFloat(chargeVoltage, 'f', 1, 64), Measurand: ocpp.MeasurandVoltage, Unit: "V"},
			{Value: strconv.FormatFloat(powerKw*1000/chargeVoltage, 'f', 1, 64), Measurand: ocpp.MeasurandCurrent, Unit: "A"},
			{Value: strconv.FormatFloat(powerKw*1000, 'f', 0, 64), Measurand: ocpp.MeasurandPower, Unit: "W"},
			{Value: strconv.FormatFloat(soc, 'f', 1, 64), Measurand: ocpp.MeasurandSoC, Unit: "Percent"},
		},
	}}}
	if _, err := p.client.call(ctx, "MeterValues", req); err != nil && ctx.Err() == nil {
		p.log.Warn("MeterValues failed", "err", err)
	}
}

// handleCall answers the central system (RemoteStart/RemoteStop/Reset/ChangeAvailability/GetConfiguration).
// It runs on the OCPP reader's goroutine and therefore takes the simulator lock.
func (s *Sim) handleCall(p *simPile, action string, payload json.RawMessage) (any, *ocpp.CallErr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch action {
	case "RemoteStartTransaction":
		var req ocpp.RemoteStartTransactionReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &ocpp.CallErr{Code: ocpp.ErrFormationViolation, Description: err.Error()}
		}
		conn := p.freeConnector()
		if req.ConnectorId != nil {
			conn = p.connector(*req.ConnectorId)
			if conn != nil && !conn.free() {
				conn = nil
			}
		}
		if conn == nil {
			p.log.Info("RemoteStartTransaction rejected: no free connector")
			return ocpp.StatusConf{Status: "Rejected"}, nil
		}
		v := s.vehicleForRemoteStart(p)
		if v == nil {
			p.log.Info("RemoteStartTransaction rejected: no vehicle available")
			return ocpp.StatusConf{Status: "Rejected"}, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if v.phase == phaseDriving {
			s.endTrip(ctx, v, time.Now())
		}
		v.pos, v.route, v.speed, v.travelled = LngLat{p.asset.Lng, p.asset.Lat}, nil, 0, 0
		v.pile, v.conn, v.phase = p, conn, phaseWaiting
		conn.vehicle, conn.pendingIDTag, conn.targetSOC = v, req.IdTag, 100 // the operator asked: charge to full
		p.log.Info("RemoteStartTransaction accepted", "connector", conn.id, "id_tag", req.IdTag, "plate", v.asset.PlateNo)
		return ocpp.StatusConf{Status: "Accepted"}, nil

	case "RemoteStopTransaction":
		var req ocpp.RemoteStopTransactionReq
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &ocpp.CallErr{Code: ocpp.ErrFormationViolation, Description: err.Error()}
		}
		conn := p.connectorByTx(req.TransactionId)
		if conn == nil {
			p.log.Info("RemoteStopTransaction rejected: unknown transaction", "tx", req.TransactionId)
			return ocpp.StatusConf{Status: "Rejected"}, nil
		}
		conn.stopReason = "Remote"
		p.log.Info("RemoteStopTransaction accepted", "connector", conn.id, "tx", req.TransactionId)
		return ocpp.StatusConf{Status: "Accepted"}, nil

	case "Reset":
		p.log.Info("Reset accepted (no-op in the simulator)")
		return ocpp.StatusConf{Status: "Accepted"}, nil

	case "ChangeAvailability":
		return ocpp.StatusConf{Status: "Accepted"}, nil

	case "GetConfiguration":
		hb := "60"
		if p.client != nil {
			p.client.mu.Lock()
			if p.client.interval > 0 {
				hb = strconv.Itoa(int(p.client.interval.Seconds()))
			}
			p.client.mu.Unlock()
		}
		return ocpp.GetConfigurationConf{ConfigurationKey: []ocpp.KeyValue{
			{Key: "HeartbeatInterval", Readonly: false, Value: &hb},
			{Key: "MeterValueSampleInterval", Readonly: true, Value: strp(strconv.Itoa(int(s.cfg.Interval.Seconds())))},
			{Key: "NumberOfConnectors", Readonly: true, Value: strp(strconv.Itoa(len(p.connectors)))},
		}}, nil
	}
	return nil, &ocpp.CallErr{Code: ocpp.ErrNotImplemented, Description: action + " not implemented by the simulator"}
}

// vehicleForRemoteStart picks the vehicle that "drives up" to a remotely
// started pile: the idle one with the lowest SOC, else a driving one (its trip is ended first).
func (s *Sim) vehicleForRemoteStart(p *simPile) *simVehicle {
	var best *simVehicle
	for _, v := range s.vehicles {
		if v.phase != phaseIdle {
			continue
		}
		if best == nil || v.soc < best.soc {
			best = v
		}
	}
	if best != nil {
		return best
	}
	for _, v := range s.vehicles {
		if v.phase == phaseDriving && !v.tripStarted {
			return v
		}
	}
	for _, v := range s.vehicles {
		if v.phase == phaseDriving {
			return v
		}
	}
	return nil
}

// startSession runs Preparing → Authorize → StartTransaction → Charging for a
// vehicle parked at conn. Returns false (connector released) when the central
// system refuses; the caller then falls back to telemetry-only charging.
func (s *Sim) startSession(ctx context.Context, v *simVehicle, p *simPile, conn *simConnector, idTag string, now time.Time) bool {
	conn.vehicle = v
	p.status(ctx, conn.id, "Preparing")
	var auth ocpp.AuthorizeConf
	if err := p.client.callInto(ctx, "Authorize", ocpp.AuthorizeReq{IdTag: idTag}, &auth); err != nil {
		p.log.Warn("Authorize failed", "err", err)
		s.releaseConnector(ctx, p, conn)
		return false
	}
	if auth.IdTagInfo.Status != "Accepted" {
		p.log.Warn("Authorize refused", "id_tag", idTag, "status", auth.IdTagInfo.Status)
		s.releaseConnector(ctx, p, conn)
		return false
	}
	// a fresh position report lets the central system bind the vehicle by location
	v.pos = LngLat{p.asset.Lng, p.asset.Lat}
	s.send(ctx, v, s.point(v, now, false, false))
	var conf ocpp.StartTransactionConf
	req := ocpp.StartTransactionReq{ConnectorId: conn.id, IdTag: idTag, MeterStart: math.Round(conn.meterWh), Timestamp: ocpp.FormatTime(now)}
	if err := p.client.callInto(ctx, "StartTransaction", req, &conf); err != nil {
		p.log.Warn("StartTransaction failed", "err", err)
		s.releaseConnector(ctx, p, conn)
		return false
	}
	conn.txID, conn.idTag, conn.stopReason, conn.pendingIDTag = conf.TransactionId, idTag, "", ""
	if conn.targetSOC <= 0 {
		conn.targetSOC = fullSOC
	}
	conn.meterFactor = 1
	if s.rng.Float64() < meterFaultRate {
		conn.meterFactor = meterFaultFactor
	}
	if conf.IdTagInfo.Status != "Accepted" {
		p.log.Warn("StartTransaction not authorized, stopping", "status", conf.IdTagInfo.Status)
		s.stopSession(ctx, v, now, "DeAuthorized")
		return false
	}
	p.status(ctx, conn.id, "Charging")
	v.pile, v.conn, v.phase = p, conn, phaseCharging
	s.log.Info("charging session started", "plate", v.asset.PlateNo, "pile", p.asset.Code, "connector", conn.id,
		"tx", conf.TransactionId, "id_tag", idTag, "soc", round(v.soc, 1), "meter_wh", round(conn.meterWh, 0), "meter_fault", conn.meterFactor != 1)
	return true
}

// chargeTick advances an OCPP session by hours of simulated time.
func (s *Sim) chargeTick(ctx context.Context, v *simVehicle, now time.Time, hours float64) {
	p, conn := v.pile, v.conn
	powerKw := p.powerKw * (0.97 + s.rng.Float64()*0.06)
	if v.soc > 80 { // taper near full
		powerKw *= 0.6
	}
	deliveredKwh := powerKw * hours
	maxKwh := (100 - v.soc) / 100 * batteryKwh / chargeEfficiency
	if deliveredKwh > maxKwh {
		deliveredKwh = maxKwh
	}
	v.soc = math.Min(100, v.soc+chargeGainPct(deliveredKwh*chargeEfficiency, 1, batteryKwh))
	conn.meterWh += deliveredKwh * 1000 * conn.meterFactor
	s.send(ctx, v, s.point(v, now, false, true))
	p.meterValues(ctx, conn, now, v.soc, powerKw)
	switch {
	case conn.stopReason != "":
		s.stopSession(ctx, v, now, conn.stopReason)
	case v.soc >= conn.targetSOC:
		s.stopSession(ctx, v, now, "Local")
	case !p.online():
		s.log.Warn("pile went offline during the session; continuing with telemetry only", "plate", v.asset.PlateNo, "pile", p.asset.Code)
		conn.vehicle, conn.txID = nil, 0
		v.pile, v.conn = nil, nil
	}
}

// stopSession ends the OCPP session and parks the vehicle.
func (s *Sim) stopSession(ctx context.Context, v *simVehicle, now time.Time, reason string) {
	p, conn := v.pile, v.conn
	if conn.txID != 0 {
		req := ocpp.StopTransactionReq{TransactionId: conn.txID, MeterStop: math.Round(conn.meterWh), Timestamp: ocpp.FormatTime(now), Reason: reason, IdTag: strp(conn.idTag)}
		if _, err := p.client.call(ctx, "StopTransaction", req); err != nil && ctx.Err() == nil {
			p.log.Warn("StopTransaction failed", "err", err)
		}
		p.status(ctx, conn.id, "Finishing")
	}
	s.log.Info("charging session ended", "plate", v.asset.PlateNo, "pile", p.asset.Code, "connector", conn.id, "tx", conn.txID,
		"reason", reason, "soc", round(v.soc, 1), "meter_wh", round(conn.meterWh, 0))
	s.releaseConnector(ctx, p, conn)
	v.pile, v.conn = nil, nil
	v.phase = phaseIdle
	v.lastIdleSent = now
	v.nextTripAt = now.Add(s.idleDelay())
	s.send(ctx, v, s.point(v, now, false, false))
}

func (s *Sim) releaseConnector(ctx context.Context, p *simPile, conn *simConnector) {
	conn.vehicle, conn.txID, conn.idTag, conn.stopReason, conn.pendingIDTag, conn.targetSOC = nil, 0, "", "", "", 0
	p.status(ctx, conn.id, "Available")
}

// shutdownSessions closes every running charging session and ends every
// ongoing trip before the simulator exits, so the next run does not hit
// "vehicle in use" / time-window conflicts.
func (s *Sim) shutdownSessions(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, v := range s.vehicles {
		if v.conn != nil && v.conn.txID != 0 {
			s.stopSession(ctx, v, now, "Other")
		}
		if v.tripStarted {
			s.endTrip(ctx, v, now)
		}
	}
	for _, p := range s.piles {
		p.persist()
	}
}

func strp(v string) *string { return &v }

// pileSummary is a log-friendly view.
func (p *simPile) String() string {
	return fmt.Sprintf("%s(online=%v,connectors=%d)", p.asset.Code, p.online(), len(p.connectors))
}
