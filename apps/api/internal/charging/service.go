package charging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/notify"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

// Service is shared by the OCPP server (pile-side events) and the HTTP handlers.
type Service struct {
	app     *app.App
	store   *store
	vs      *vstatus.Store
	notify  *notify.Service
	billing BillingHook
	hb      time.Duration
}

// NewService wires the service; billing may be nil (sessions then always go to review).
func NewService(a *app.App, billing BillingHook) *Service {
	hb := DefaultHeartbeat
	if a.Cfg != nil && a.Cfg.OCPP.HeartbeatInterval > 0 {
		hb = a.Cfg.OCPP.HeartbeatInterval
	}
	return &Service{app: a, store: &store{db: a.DB}, vs: vstatus.New(a), notify: notify.NewService(a), billing: billing, hb: hb}
}

// HeartbeatInterval is what BootNotification tells the pile.
func (s *Service) HeartbeatInterval() time.Duration { return s.hb }

// OfflineAfter is how stale a heartbeat may be before the pile counts as offline.
func (s *Service) OfflineAfter() time.Duration { return 3 * s.hb }

// online = live OCPP connection + fresh heartbeat.
func (s *Service) online(p *PileLive) bool {
	gw := Gateway()
	if gw == nil || !gw.Online(p.ID) {
		return false
	}
	return p.LastHeartbeatAt != nil && time.Since(*p.LastHeartbeatAt) < s.OfflineAfter()
}

// ================================================================== pile side (called by internal/ocpp)

// PileByCode resolves the OCPP identity of a connecting pile (nil when unknown or disabled).
func (s *Service) PileByCode(ctx context.Context, code string) (*PileLive, error) {
	p, n, err := s.store.pileByCode(ctx, code)
	if err != nil || p == nil {
		return nil, err
	}
	if n > 1 {
		s.app.Log.Warn().Str("pile_code", code).Int("matches", n).Msg("pile code shared by several tenants; using the oldest archive")
	}
	return p, nil
}

// Boot handles BootNotification: vendor (if empty), heartbeat, status available.
func (s *Service) Boot(ctx context.Context, pileID uuid.UUID, vendor, model string) error {
	if err := s.store.boot(ctx, pileID, strings.TrimSpace(vendor)); err != nil {
		return err
	}
	s.app.Log.Info().Str("pile", pileID.String()).Str("vendor", vendor).Str("model", model).Msg("pile booted")
	return s.pushPile(ctx, pileID, "boot", nil, nil)
}

// Heartbeat stamps last_heartbeat_at; a pile that was offline becomes available again.
func (s *Service) Heartbeat(ctx context.Context, pileID uuid.UUID) error {
	if err := s.store.heartbeat(ctx, pileID); err != nil {
		return err
	}
	p, err := s.store.pileAny(ctx, pileID)
	if err != nil || p == nil {
		return err
	}
	if p.Status == PileOffline {
		conns, err := s.store.connectorsOf(ctx, pileID)
		if err != nil {
			return err
		}
		if err := s.store.setPileStatus(ctx, pileID, AggregatePileStatus(conns)); err != nil {
			return err
		}
		return s.pushPile(ctx, pileID, "heartbeat", nil, nil)
	}
	return nil
}

// StatusNotification upserts the connector and re-aggregates the pile status.
func (s *Service) StatusNotification(ctx context.Context, pileID uuid.UUID, connectorID int, status string, errCode, info *string) error {
	if !ValidConnectorStatus(status) {
		return fmt.Errorf("invalid connector status %q", status)
	}
	if errCode != nil && (*errCode == "" || *errCode == "NoError") {
		errCode = nil
	}
	if err := s.store.upsertConnector(ctx, pileID, connectorID, status, errCode, info); err != nil {
		return err
	}
	_ = s.store.heartbeat(ctx, pileID)
	conns, err := s.store.connectorsOf(ctx, pileID)
	if err != nil {
		return err
	}
	if err := s.store.setPileStatus(ctx, pileID, AggregatePileStatus(conns)); err != nil {
		return err
	}
	return s.pushPile(ctx, pileID, "status", &connectorID, &status)
}

// Disconnected marks the pile offline and every connector unavailable.
func (s *Service) Disconnected(ctx context.Context, pileID uuid.UUID) error {
	if err := s.store.setPileStatus(ctx, pileID, PileOffline); err != nil {
		return err
	}
	if err := s.store.setAllConnectors(ctx, pileID, ConnUnavailable); err != nil {
		return err
	}
	return s.pushPile(ctx, pileID, "offline", nil, nil)
}

// SweepStale flips piles without a heartbeat for OfflineAfter to offline and
// returns them so the server can drop their (dead) connections.
func (s *Service) SweepStale(ctx context.Context) ([]PileLive, error) {
	rows, err := s.store.sweepStale(ctx, time.Now().Add(-s.OfflineAfter()))
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		_ = s.store.setAllConnectors(ctx, p.ID, ConnUnavailable)
		_ = s.pushPile(ctx, p.ID, "offline", nil, nil)
	}
	return rows, nil
}

// Authorize maps an idTag onto the tenant's NFC cards: Accepted / Blocked / Invalid.
func (s *Service) Authorize(ctx context.Context, tenantID uuid.UUID, idTag string) (string, error) {
	card, err := s.store.cardByUID(ctx, tenantID, strings.ToUpper(strings.TrimSpace(idTag)))
	if err != nil {
		return "", err
	}
	return authorizeStatus(card), nil
}

// authorizeStatus is the OCPP AuthorizationStatus for a card row.
func authorizeStatus(c *cardRow) string {
	switch {
	case c == nil:
		return "Invalid"
	case c.Status == "lost", c.Status == "disabled":
		return "Blocked"
	case c.UserID == nil:
		return "Invalid"
	case c.Status == "active":
		return "Accepted"
	}
	return "Invalid"
}

// StartInput is what StartTransaction.req carries.
type StartInput struct {
	PileID      uuid.UUID
	ConnectorID int
	IDTag       string
	MeterStart  float64 // Wh
	TS          time.Time
}

// StartResult is what the pile gets back.
type StartResult struct {
	TxID        uuid.UUID
	TxNo        string
	OcppTxID    int
	IDTagStatus string
}

// StartTransaction creates the session, attributes it (card → user/dept) and
// binds a vehicle by priority (preset → location → recent trip → none).
func (s *Service) StartTransaction(ctx context.Context, in StartInput) (*StartResult, error) {
	pile, err := s.store.pileAny(ctx, in.PileID)
	if err != nil {
		return nil, err
	}
	if pile == nil {
		return nil, errors.New("unknown pile")
	}
	ts := in.TS
	if ts.IsZero() {
		ts = time.Now()
	}
	idTag := strings.ToUpper(strings.TrimSpace(in.IDTag))
	if in.ConnectorID < 1 {
		in.ConnectorID = 1
	}

	// who: the card holder
	card, err := s.store.cardByUID(ctx, pile.TenantID, idTag)
	if err != nil {
		return nil, err
	}
	tagStatus := authorizeStatus(card)
	var cardID, userID, deptID *uuid.UUID
	if tagStatus == "Accepted" {
		cardID, userID = &card.ID, card.UserID
	}
	preset, presetUser := TakePendingVehicle(in.PileID, in.ConnectorID)
	if userID == nil && presetUser != nil {
		userID = presetUser // remote start with a foreign id_tag: attribute to the operator
	}
	if userID != nil {
		u, err := s.store.user(ctx, pile.TenantID, *userID)
		if err != nil {
			return nil, err
		}
		if u == nil {
			userID = nil
		} else {
			deptID = u.DeptID
		}
	}

	// a connector can only run one session: close a stale one first
	if old, err := s.store.activeOn(ctx, in.PileID, in.ConnectorID); err != nil {
		return nil, err
	} else if old != nil {
		s.app.Log.Warn().Str("tx_no", old.TxNo).Msg("connector already has a charging transaction; closing it before starting a new one")
		s.finish(ctx, old, lastWhOr(old, old.MeterStart), ts, "Other")
	}

	// which vehicle
	cand := BindCandidates{UserKnown: userID != nil}
	if preset != nil {
		if v, err := s.store.vehicle(ctx, *preset); err != nil {
			return nil, err
		} else if v != nil && v.TenantID == pile.TenantID {
			cand.Preset = preset
		}
	}
	if pile.Lng != nil && pile.Lat != nil {
		if cand.Nearby, err = s.store.nearbyVehicles(ctx, pile.TenantID, *pile.Lng, *pile.Lat, ts.Add(-BindTelemetryWindow)); err != nil {
			return nil, err
		}
	}
	if userID != nil {
		if cand.RecentTrip, err = s.store.recentTripVehicle(ctx, pile.TenantID, *userID, ts.Add(-BindRecentTripWindow)); err != nil {
			return nil, err
		}
	}
	vehicleID, method := PickVehicle(cand, ts)

	var socStart *float64
	if vehicleID != nil {
		if live, err := s.vs.Get(ctx, *vehicleID); err != nil {
			return nil, err
		} else if live != nil {
			socStart = live.SOC
		}
	}
	p := insertParams{
		tenantID: pile.TenantID, pileID: in.PileID, connectorID: in.ConnectorID, idTag: idTag, cardID: cardID, userID: userID, deptID: deptID,
		vehicleID: vehicleID, bindMethod: method, startAt: ts, meterStart: in.MeterStart, socStart: socStart, review: ReviewNone,
	}
	if vehicleID == nil {
		note := "未能绑定车辆"
		p.review, p.note = ReviewPending, &note
	}
	id, txNo, ocppID, err := s.store.insert(ctx, p)
	if isUnique(err, "uq_charge_tx_active") {
		// lost a race with another StartTransaction on the same connector: close it and retry once
		if old, e := s.store.activeOn(ctx, in.PileID, in.ConnectorID); e == nil && old != nil {
			s.finish(ctx, old, lastWhOr(old, old.MeterStart), ts, "Other")
		}
		id, txNo, ocppID, err = s.store.insert(ctx, p)
	}
	if err != nil {
		return nil, err
	}
	if vehicleID != nil {
		on := true
		if _, err := s.vs.ApplyTelemetry(ctx, *vehicleID, 0, vstatus.Telemetry{Charging: &on}); err != nil {
			s.app.Log.Warn().Err(err).Str("vehicle", vehicleID.String()).Msg("set vehicle charging flag failed")
		}
	}
	r, _ := s.store.get(ctx, pile.TenantID, id)
	if r != nil {
		plate := "未知车辆"
		if r.PlateNo != nil {
			plate = *r.PlateNo
		}
		if userID != nil {
			s.send(ctx, pile.TenantID, []uuid.UUID{*userID}, NotifyStarted, "开始充电："+txNo,
				fmt.Sprintf("%s 于 %s 在 %s（%d 号枪）开始充电", plate, fmtTime(ts), pile.Name, in.ConnectorID), id)
		}
		s.pushTx(pile, r, "started", nil, nil, nil)
	}
	s.app.Log.Info().Str("tx_no", txNo).Str("pile", pile.PileCode).Str("bind", method).Str("id_tag_status", tagStatus).Msg("charging started")
	return &StartResult{TxID: id, TxNo: txNo, OcppTxID: ocppID, IDTagStatus: tagStatus}, nil
}

// MeterValues stores the samples of a session and pushes the live numbers.
// ocppTxID may be nil (some piles omit it): the connector's active session is used.
func (s *Service) MeterValues(ctx context.Context, pileID uuid.UUID, connectorID int, ocppTxID *int, samples []MeterSample) error {
	if len(samples) == 0 {
		return nil
	}
	var r *txRow
	var err error
	if ocppTxID != nil {
		r, err = s.store.byOcpp(ctx, pileID, *ocppTxID)
	} else {
		r, err = s.store.activeOn(ctx, pileID, connectorID)
	}
	if err != nil {
		return err
	}
	if r == nil || r.Status != StatusCharging {
		s.app.Log.Debug().Str("pile", pileID.String()).Msg("meter values for no active transaction, ignored")
		return nil
	}
	if err := s.store.insertMeterValues(ctx, r.ID, pileID, samples); err != nil {
		return err
	}
	_ = s.store.heartbeat(ctx, pileID)
	last := samples[len(samples)-1]
	var kwh *float64
	if last.Wh != nil {
		k := SessionKwh(r.MeterStart, *last.Wh)
		kwh = &k
	}
	pile, _ := s.store.pileAny(ctx, pileID)
	if pile != nil {
		s.pushTx(pile, r, "meter", kwh, last.PowerKw, last.SOC)
	}
	return nil
}

// StopTransaction closes the session named by the pile's transactionId.
// Unknown or already-closed transactions are ignored (the pile still gets Accepted).
func (s *Service) StopTransaction(ctx context.Context, pileID uuid.UUID, ocppTxID int, meterStop float64, ts time.Time, reason string) error {
	r, err := s.store.byOcpp(ctx, pileID, ocppTxID)
	if err != nil {
		return err
	}
	if r == nil {
		s.app.Log.Warn().Str("pile", pileID.String()).Int("ocpp_tx_id", ocppTxID).Msg("StopTransaction for unknown transaction")
		return nil
	}
	if r.Status != StatusCharging {
		return nil
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	s.finish(ctx, r, meterStop, ts, reason)
	return nil
}

// finish ends a charging transaction: energy, BMS cross check, verdict,
// billing (or the review queue), vehicle flag, notifications, push.
func (s *Service) finish(ctx context.Context, r *txRow, meterStop float64, endAt time.Time, reason string) {
	if endAt.Before(r.StartAt) {
		endAt = r.StartAt
	}
	kwh := SessionKwh(r.MeterStart, meterStop)
	var socEnd *float64
	var battery float64
	if r.VehicleID != nil {
		if live, err := s.vs.Get(ctx, *r.VehicleID); err == nil && live != nil {
			socEnd = live.SOC
		}
		if r.VehicleBatteryKwh != nil {
			battery = *r.VehicleBatteryKwh
		}
	}
	cc := Compare(kwh, r.BmsSocStart, socEnd, battery)
	needs, why := NeedsReview(r.VehicleID != nil, cc.DeviationPct)
	cp := closeParams{id: r.ID, endAt: endAt, meterStop: meterStop, kwh: kwh, reason: nilIfEmpty(reason), socEnd: socEnd, cc: cc, review: ReviewNone}
	if needs {
		cp.review, cp.reviewNote = ReviewPending, &why
	}
	if err := s.store.close(ctx, cp); err != nil {
		if !errors.Is(err, errNotCharging) {
			s.app.Log.Error().Err(err).Str("tx_no", r.TxNo).Msg("close charging transaction failed")
		}
		return
	}
	if r.VehicleID != nil {
		off := false
		if _, err := s.vs.ApplyTelemetry(ctx, *r.VehicleID, 0, vstatus.Telemetry{Charging: &off}); err != nil {
			s.app.Log.Warn().Err(err).Str("vehicle", r.VehicleID.String()).Msg("clear vehicle charging flag failed")
		}
	}
	pile, _ := s.store.pileAny(ctx, r.PileID)
	var result *BillingResult
	if needs {
		s.notifyReview(ctx, r, kwh, why)
	} else {
		var err error
		if result, err = s.bill(ctx, r, kwh, endAt, ReviewNone, nil, nil); err != nil {
			why = "计费失败：" + err.Error()
			_ = s.store.markPending(ctx, r.ID, why)
			s.app.Log.Warn().Err(err).Str("tx_no", r.TxNo).Msg("charging billing failed; queued for review")
			s.notifyReview(ctx, r, kwh, why)
		}
	}
	if r.UserID != nil {
		pileName := r.PileName
		if result != nil {
			s.send(ctx, r.TenantID, []uuid.UUID{*r.UserID}, NotifyEnded, "充电完成："+r.TxNo,
				fmt.Sprintf("%s 充电完成 %.3f kWh，费用 ¥%.2f", pileName, kwh, result.Cost), r.ID)
		} else {
			s.send(ctx, r.TenantID, []uuid.UUID{*r.UserID}, NotifyEnded, "充电完成："+r.TxNo,
				fmt.Sprintf("%s 充电完成 %.3f kWh，待复核（%s）", pileName, kwh, why), r.ID)
		}
	}
	if pile != nil {
		if fresh, _ := s.store.get(ctx, r.TenantID, r.ID); fresh != nil {
			s.pushTx(pile, fresh, "ended", &kwh, nil, socEnd)
		}
	}
	s.app.Log.Info().Str("tx_no", r.TxNo).Float64("kwh", kwh).Bool("review", needs || result == nil).Str("reason", reason).Msg("charging ended")
}

// bill prices and debits through the hook and stamps the result (status settled).
func (s *Service) bill(ctx context.Context, r *txRow, kwh float64, endAt time.Time, review string, reviewedBy *uuid.UUID, note *string) (*BillingResult, error) {
	if s.billing == nil {
		return nil, errors.New("billing hook not configured")
	}
	res, err := s.billing.PriceAndCharge(ctx, BillingInput{
		TenantID: r.TenantID, TxID: r.ID, TxNo: r.TxNo, Kwh: kwh, UserID: r.UserID, DeptID: r.DeptID, VehicleID: r.VehicleID,
		StartAt: r.StartAt, EndAt: endAt,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New("billing returned no result")
	}
	if err := s.store.settle(ctx, r.ID, res, review, reviewedBy, note); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) notifyReview(ctx context.Context, r *txRow, kwh float64, why string) {
	admins, err := s.store.tenantAdmins(ctx, r.TenantID)
	if err != nil || len(admins) == 0 {
		return
	}
	s.send(ctx, r.TenantID, admins, NotifyReview, "充电事务待复核："+r.TxNo,
		fmt.Sprintf("%s（%d 号枪）%.3f kWh：%s", r.PileName, r.ConnectorID, kwh, why), r.ID)
}

// ================================================================== HTTP side

// ListLive builds the contract PileLive view of every pile of the tenant.
func (s *Service) ListLive(ctx context.Context, tenantID uuid.UUID) ([]PileLive, error) {
	piles, err := s.store.pilesByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(piles))
	for i := range piles {
		ids[i] = piles[i].ID
	}
	conns, err := s.store.connectors(ctx, ids)
	if err != nil {
		return nil, err
	}
	active, err := s.store.activeByPiles(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	activeBy := map[uuid.UUID]map[int]*Transaction{}
	for i := range active {
		t := toTransaction(active[i])
		if activeBy[t.PileID] == nil {
			activeBy[t.PileID] = map[int]*Transaction{}
		}
		activeBy[t.PileID][t.ConnectorID] = &t
	}
	for i := range piles {
		p := &piles[i]
		p.Online = s.online(p)
		p.Connectors = fillConnectors(conns[p.ID], p.ConnectorCount, activeBy[p.ID])
	}
	return piles, nil
}

// fillConnectors returns one entry per physical connector 1..count (unknown ones
// as Unavailable), keeps connector 0 when reported, and attaches active sessions.
func fillConnectors(known []Connector, count int, active map[int]*Transaction) []Connector {
	byID := map[int]Connector{}
	for _, c := range known {
		byID[c.ConnectorID] = c
	}
	out := []Connector{}
	if c, ok := byID[0]; ok {
		out = append(out, c)
	}
	max := count
	for id := range byID {
		if id > max {
			max = id
		}
	}
	for id := 1; id <= max; id++ {
		c, ok := byID[id]
		if !ok {
			c = Connector{ConnectorID: id, Status: ConnUnavailable}
		}
		c.Transaction = active[id]
		out = append(out, c)
	}
	return out
}

func (s *Service) filter(q ListQuery) (listFilter, error) {
	f := listFilter{ListQuery: q}
	var err error
	if f.from, err = parseTime(q.From, "from"); err != nil {
		return f, err
	}
	if f.to, err = parseTime(q.To, "to"); err != nil {
		return f, err
	}
	return f, nil
}

func parseTime(s, name string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, httpx.BadRequest(name + " 须为 RFC3339 时间")
	}
	return &t, nil
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[Transaction], error) {
	f, err := s.filter(q)
	if err != nil {
		return pagination.Page[Transaction]{}, err
	}
	rows, total, err := s.store.list(ctx, tenantID, f, pg)
	if err != nil {
		return pagination.Page[Transaction]{}, err
	}
	items := make([]Transaction, 0, len(rows))
	for _, r := range rows {
		items = append(items, toTransaction(r))
	}
	return pagination.NewPage(items, total, pg), nil
}

const exportMaxRows = 5000

// Export returns up to exportMaxRows transactions matching the filters, newest first.
func (s *Service) Export(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Transaction, error) {
	f, err := s.filter(q)
	if err != nil {
		return nil, err
	}
	rows, _, err := s.store.list(ctx, tenantID, f, pagination.Query{Page: 1, PageSize: exportMaxRows, SortBy: "start_at", SortDesc: true})
	if err != nil {
		return nil, err
	}
	out := make([]Transaction, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTransaction(r))
	}
	return out, nil
}

// Get returns the detail with a sampled meter curve (≤ DetailMeterPoints).
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Transaction, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("充电事务不存在")
	}
	t := toTransaction(*r)
	pts, err := s.store.meterValues(ctx, id)
	if err != nil {
		return nil, err
	}
	t.MeterValues = Thin(WithKwh(pts, r.MeterStart), DetailMeterPoints)
	return &t, nil
}

// MeterValuesOf returns the whole curve.
func (s *Service) MeterValuesOf(ctx context.Context, tenantID, id uuid.UUID) ([]MeterValue, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("充电事务不存在")
	}
	pts, err := s.store.meterValues(ctx, id)
	if err != nil {
		return nil, err
	}
	return WithKwh(pts, r.MeterStart), nil
}

// Summary aggregates the day (Asia/Shanghai) and its month.
func (s *Service) Summary(ctx context.Context, tenantID uuid.UUID, date string) (*Summary, error) {
	day := time.Now().In(approval.Shanghai)
	if strings.TrimSpace(date) != "" {
		d, err := time.ParseInLocation("2006-01-02", date, approval.Shanghai)
		if err != nil {
			return nil, httpx.BadRequest("date 须为 YYYY-MM-DD")
		}
		day = d
	}
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, approval.Shanghai)
	monthStart := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, approval.Shanghai)
	sm := &Summary{Date: dayStart.Format("2006-01-02")}
	var err error
	if sm.Today, err = s.store.bucket(ctx, tenantID, dayStart, dayStart.AddDate(0, 0, 1)); err != nil {
		return nil, err
	}
	if sm.Month, err = s.store.bucket(ctx, tenantID, monthStart, monthStart.AddDate(0, 1, 0)); err != nil {
		return nil, err
	}
	if sm.Ongoing, sm.PendingReview, err = s.store.counts(ctx, tenantID); err != nil {
		return nil, err
	}
	piles, err := s.store.pilesByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range piles {
		sm.Piles.Total++
		if s.online(&piles[i]) {
			sm.Piles.Online++
		}
		switch piles[i].Status {
		case PileCharging:
			sm.Piles.Charging++
		case PileFaulted:
			sm.Piles.Faulted++
		}
	}
	if sm.ByPile, err = s.store.byPile(ctx, tenantID, monthStart, monthStart.AddDate(0, 1, 0)); err != nil {
		return nil, err
	}
	return sm, nil
}

// RemoteStart asks the pile to start a session on a connector.
func (s *Service) RemoteStart(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, pileID uuid.UUID, req RemoteStartRequest) (*RemoteResult, error) {
	pile, err := s.store.pile(ctx, tenantID, pileID)
	if err != nil {
		return nil, err
	}
	if pile == nil {
		return nil, httpx.NotFound("充电桩不存在")
	}
	if pile.Status == PileDisabled {
		return nil, httpx.Conflict("充电桩已停用")
	}
	gw := Gateway()
	if gw == nil || !s.online(pile) {
		return nil, httpx.Conflict("充电桩不在线")
	}
	connectorID := 1
	if req.ConnectorID != nil {
		connectorID = *req.ConnectorID
	}
	conns, err := s.store.connectorsOf(ctx, pileID)
	if err != nil {
		return nil, err
	}
	var conn *Connector
	for i := range conns {
		if conns[i].ConnectorID == connectorID {
			conn = &conns[i]
		}
	}
	if conn == nil {
		return nil, httpx.Conflict(fmt.Sprintf("%d 号枪状态未知", connectorID))
	}
	if !CanRemoteStart(conn.Status) {
		return nil, httpx.Conflict(fmt.Sprintf("%d 号枪当前不可用：%s", connectorID, conn.Status))
	}
	idTag := ""
	if req.IDTag != nil {
		idTag = strings.ToUpper(strings.TrimSpace(*req.IDTag))
	}
	if idTag == "" {
		if idTag, err = s.store.firstActiveCard(ctx, tenantID, actor.UserID); err != nil {
			return nil, err
		}
		if idTag == "" {
			return nil, httpx.BadRequest("当前用户没有有效的 NFC 卡，请指定 id_tag")
		}
		idTag = strings.ToUpper(idTag)
	}
	if req.VehicleID != nil {
		v, err := s.store.vehicle(ctx, *req.VehicleID)
		if err != nil {
			return nil, err
		}
		if v == nil || v.TenantID != tenantID {
			return nil, httpx.BadRequest("车辆不存在")
		}
		SetPendingVehicle(pileID, connectorID, v.ID, &actor.UserID)
	}
	status, err := gw.RemoteStart(ctx, pileID, connectorID, idTag)
	if err != nil {
		ClearPendingVehicle(pileID, connectorID)
		return nil, httpx.Conflict("充电桩无应答：" + err.Error())
	}
	if status != "Accepted" {
		ClearPendingVehicle(pileID, connectorID)
	}
	return &RemoteResult{Status: status}, nil
}

// RemoteStop asks the pile to stop a charging transaction.
func (s *Service) RemoteStop(ctx context.Context, tenantID, pileID uuid.UUID, req RemoteStopRequest) (*RemoteResult, error) {
	pile, err := s.store.pile(ctx, tenantID, pileID)
	if err != nil {
		return nil, err
	}
	if pile == nil {
		return nil, httpx.NotFound("充电桩不存在")
	}
	r, err := s.store.get(ctx, tenantID, req.TransactionID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("充电事务不存在")
	}
	if r.PileID != pileID {
		return nil, httpx.BadRequest("事务不属于该充电桩")
	}
	if r.Status != StatusCharging {
		return nil, httpx.Conflict("事务不在充电中：" + r.Status)
	}
	gw := Gateway()
	if gw == nil || !s.online(pile) {
		return nil, httpx.Conflict("充电桩不在线")
	}
	status, err := gw.RemoteStop(ctx, pileID, r.OcppTxID)
	if err != nil {
		return nil, httpx.Conflict("充电桩无应答：" + err.Error())
	}
	return &RemoteResult{Status: status}, nil
}

// Review approves (optionally fixing user/vehicle, then billing) or rejects a pending transaction.
func (s *Service) Review(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID, req ReviewRequest) (*Transaction, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("充电事务不存在")
	}
	if r.ReviewStatus != ReviewPending {
		return nil, httpx.Conflict("事务不在待复核状态：" + r.ReviewStatus)
	}
	if r.Status == StatusCharging {
		return nil, httpx.Conflict("事务仍在充电中")
	}
	if req.Action == "reject" {
		if err := s.store.reject(ctx, id, actor.UserID, req.Note); err != nil {
			return nil, err
		}
		return s.Get(ctx, tenantID, id)
	}

	// approve: apply corrections
	userID, deptID, vehicleID := r.UserID, r.DeptID, r.VehicleID
	method := BindNone
	if r.BindMethod != nil {
		method = *r.BindMethod
	}
	if req.UserID != nil {
		u, err := s.store.user(ctx, tenantID, *req.UserID)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, httpx.BadRequest("用户不存在")
		}
		userID, deptID = &u.ID, u.DeptID
	}
	socStart, socEnd := r.BmsSocStart, r.BmsSocEnd
	var battery float64
	if r.VehicleBatteryKwh != nil {
		battery = *r.VehicleBatteryKwh
	}
	if req.VehicleID != nil && (vehicleID == nil || *vehicleID != *req.VehicleID) {
		v, err := s.store.vehicle(ctx, *req.VehicleID)
		if err != nil {
			return nil, err
		}
		if v == nil || v.TenantID != tenantID {
			return nil, httpx.BadRequest("车辆不存在")
		}
		vehicleID, method, battery = &v.ID, BindManual, v.BatteryKwh
		// re-read the BMS from the vehicle's own telemetry around the session
		if socStart, err = s.store.telemetrySOCAt(ctx, v.ID, r.StartAt); err != nil {
			return nil, err
		}
		end := time.Now()
		if r.EndAt != nil {
			end = *r.EndAt
		}
		if socEnd, err = s.store.telemetrySOCAt(ctx, v.ID, end); err != nil {
			return nil, err
		}
	} else if vehicleID != nil && method == BindNone {
		method = BindManual
	}
	if vehicleID == nil && userID == nil {
		return nil, httpx.Conflict("无法确认归属：请指定 user_id 或 vehicle_id")
	}
	if vehicleID == nil && method != BindCard {
		method = BindCard
	}
	kwh := 0.0
	if r.Kwh != nil {
		kwh = *r.Kwh
	}
	cc := Compare(kwh, socStart, socEnd, battery)
	if err := s.store.setAttribution(ctx, attributionParams{id: id, userID: userID, deptID: deptID, vehicleID: vehicleID, bindMethod: method, socStart: socStart, socEnd: socEnd, cc: cc}); err != nil {
		return nil, err
	}
	r.UserID, r.DeptID, r.VehicleID = userID, deptID, vehicleID
	endAt := time.Now()
	if r.EndAt != nil {
		endAt = *r.EndAt
	}
	if _, err := s.bill(ctx, r, kwh, endAt, ReviewApproved, &actor.UserID, req.Note); err != nil {
		_ = s.store.markPending(ctx, id, "计费失败："+err.Error())
		var ae *httpx.AppError
		if errors.As(err, &ae) {
			return nil, &httpx.AppError{Code: ae.Code, Status: ae.Status, Message: "计费失败：" + ae.Message, Err: ae.Err}
		}
		return nil, httpx.Conflict("计费失败：" + err.Error())
	}
	if t, err := s.store.get(ctx, tenantID, id); err == nil && t != nil && t.UserID != nil && t.Cost != nil {
		s.send(ctx, tenantID, []uuid.UUID{*t.UserID}, NotifyEnded, "充电已结算："+t.TxNo,
			fmt.Sprintf("%s 充电 %.3f kWh，费用 ¥%.2f（复核通过）", t.PileName, kwh, *t.Cost), id)
	}
	return s.Get(ctx, tenantID, id)
}

// ================================================================== mapping & side effects

func toTransaction(r txRow) Transaction {
	t := Transaction{
		ID: r.ID, TenantID: r.TenantID, TxNo: r.TxNo, PileID: r.PileID, PileCode: r.PileCode, PileName: r.PileName,
		ConnectorID: r.ConnectorID, OcppTxID: r.OcppTxID, IDTag: r.IDTag, DeptID: r.DeptID, DeptName: r.DeptName, BindMethod: r.BindMethod,
		Status: r.Status, StartAt: r.StartAt, EndAt: r.EndAt, MeterStart: r.MeterStart, MeterStop: r.MeterStop, Kwh: r.Kwh,
		UnitPrice: r.UnitPrice, Cost: r.Cost, StopReason: r.StopReason, BmsSocStart: r.BmsSocStart, BmsSocEnd: r.BmsSocEnd,
		BmsKwhEst: r.BmsKwhEst, DeviationPct: r.DeviationPct, ReviewStatus: r.ReviewStatus, ReviewNote: r.ReviewNote,
		ReviewedBy: r.ReviewedBy, ReviewedByName: r.ReviewedByName, ReviewedAt: r.ReviewedAt, Attribution: r.Attribution,
		AccountID: r.AccountID, AccountTxnID: r.AccountTxnID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if t.DeptName == nil {
		t.DeptName = r.UserDeptName
	}
	if r.UserID != nil && r.UserName != nil {
		t.User = &approval.UserBrief{ID: *r.UserID, Name: *r.UserName, Username: deref(r.UserUsername), DeptName: r.UserDeptName, Phone: r.UserPhone}
	}
	if r.VehicleID != nil && r.PlateNo != nil {
		t.Vehicle = &approval.VehicleBrief{
			ID: *r.VehicleID, PlateNo: *r.PlateNo, Brand: r.VehicleBrand, Model: r.VehicleModel, Status: deref(r.VehicleStatus),
			SOC: r.VehicleSOC, RangeKm: r.VehicleRangeKm, HomeDeptID: r.VehicleHomeDeptID, HomeDeptName: r.VehicleHomeDeptName,
		}
	}
	end := time.Now()
	if r.EndAt != nil {
		end = *r.EndAt
	}
	d := round2(end.Sub(r.StartAt).Minutes())
	t.DurationMin = &d
	if r.Status == StatusCharging {
		if r.LastWh != nil {
			k := SessionKwh(r.MeterStart, *r.LastWh)
			t.Kwh = &k
		}
		t.PowerKw = r.LastPowerKw
	}
	return t
}

// pushPile publishes charging.updated for a pile-level change.
func (s *Service) pushPile(ctx context.Context, pileID uuid.UUID, event string, connectorID *int, connStatus *string) error {
	p, err := s.store.pileAny(ctx, pileID)
	if err != nil || p == nil {
		return err
	}
	s.app.Hub.Publish(p.TenantID, ws.Event{Type: ws.EvChargingUpdated, Data: ChargingUpdate{
		PileID: p.ID, PileCode: p.PileCode, PileStatus: p.Status, Online: s.online(p), ConnectorID: connectorID, Connector: connStatus, Event: event,
	}})
	return nil
}

// pushTx publishes charging.updated for a transaction change.
func (s *Service) pushTx(p *PileLive, r *txRow, event string, kwh, power, soc *float64) {
	if kwh == nil {
		if r.Status == StatusCharging && r.LastWh != nil {
			k := SessionKwh(r.MeterStart, *r.LastWh)
			kwh = &k
		} else if r.Kwh != nil {
			kwh = r.Kwh
		}
	}
	if power == nil && r.Status == StatusCharging {
		power = r.LastPowerKw
	}
	status := r.Status
	s.app.Hub.Publish(p.TenantID, ws.Event{Type: ws.EvChargingUpdated, Data: ChargingUpdate{
		PileID: p.ID, PileCode: p.PileCode, PileStatus: p.Status, Online: s.online(p), ConnectorID: &r.ConnectorID,
		TxID: &r.ID, TxNo: &r.TxNo, TxStatus: &status, Kwh: kwh, PowerKw: power, SOC: soc, Event: event,
	}})
}

func (s *Service) send(ctx context.Context, tenantID uuid.UUID, users []uuid.UUID, typ, title, content string, txID uuid.UUID) {
	if err := s.notify.Send(ctx, tenantID, users, notify.Message{Type: typ, Title: title, Content: content, RefType: "charge_transaction", RefID: txID.String()}); err != nil {
		s.app.Log.Error().Err(err).Str("tx", txID.String()).Msg("charging notify failed")
	}
}

func lastWhOr(r *txRow, fallback float64) float64 {
	if r.LastWh != nil && *r.LastWh > fallback {
		return *r.LastWh
	}
	return fallback
}

func isUnique(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

func nilIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func fmtTime(t time.Time) string { return t.In(approval.Shanghai).Format("01-02 15:04") }
