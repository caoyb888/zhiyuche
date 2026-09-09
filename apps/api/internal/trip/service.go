package trip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/notify"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/param"
	"github.com/caoyb888/zhiyuche/apps/api/internal/telemetry"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	app    *app.App
	store  *store
	vs     *vstatus.Store
	notify *notify.Service
}

func NewService(a *app.App) *Service {
	return &Service{app: a, store: &store{db: a.DB}, vs: vstatus.New(a), notify: notify.NewService(a)}
}

// ---- read

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, q ListQuery, pg pagination.Query) (pagination.Page[Trip], error) {
	f, err := s.filter(actor, q)
	if err != nil {
		return pagination.Page[Trip]{}, err
	}
	rows, total, err := s.store.list(ctx, tenantID, f, pg)
	if err != nil {
		return pagination.Page[Trip]{}, err
	}
	items := make([]Trip, 0, len(rows))
	for _, r := range rows {
		items = append(items, toTrip(r))
	}
	return pagination.NewPage(items, total, pg), nil
}

func (s *Service) filter(actor *auth.Principal, q ListQuery) (listFilter, error) {
	if q.Scope == "" {
		q.Scope = "mine"
	}
	if q.Scope == "all" && !actor.Has(PermManage) && !actor.Has(PermExport) {
		return listFilter{}, httpx.Forbidden("没有权限：" + PermManage + " / " + PermExport)
	}
	f := listFilter{ListQuery: q, me: actor.UserID}
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

// Get returns the detail (with events) when the caller may see it: driver,
// applicant / approvers of the linked approval, or trip:manage.
func (s *Service) Get(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID) (*Trip, error) {
	r, err := s.visible(ctx, tenantID, actor, id)
	if err != nil {
		return nil, err
	}
	t := toTrip(*r)
	if t.Events, err = s.store.events(ctx, id); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) visible(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID) (*row, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("行程不存在")
	}
	if actor.Has(PermManage) || (r.DriverID != nil && *r.DriverID == actor.UserID) ||
		(r.ApprovalApplicantID != nil && *r.ApprovalApplicantID == actor.UserID) {
		return r, nil
	}
	ids, err := s.store.approverIDs(ctx, r.ApprovalID)
	if err != nil {
		return nil, err
	}
	for _, a := range ids {
		if a == actor.UserID {
			return r, nil
		}
	}
	return nil, httpx.Forbidden("无权查看该行程")
}

func (s *Service) Track(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID, step int) (*Track, error) {
	r, err := s.visible(ctx, tenantID, actor, id)
	if err != nil {
		return nil, err
	}
	pts, err := s.store.points(ctx, id)
	if err != nil {
		return nil, err
	}
	tr := &Track{TripID: id, Points: Thin(pts, step)}
	if tr.PlannedRoute, err = s.store.plannedRouteFull(ctx, r.ApprovalID); err != nil {
		return nil, err
	}
	return tr, nil
}

func (s *Service) Events(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID) ([]Event, error) {
	if _, err := s.visible(ctx, tenantID, actor, id); err != nil {
		return nil, err
	}
	return s.store.events(ctx, id)
}

// Summary aggregates the trips that started on date (Asia/Shanghai).
func (s *Service) Summary(ctx context.Context, tenantID uuid.UUID, date string) (*Summary, error) {
	day := time.Now().In(approval.Shanghai)
	if strings.TrimSpace(date) != "" {
		d, err := time.ParseInLocation("2006-01-02", date, approval.Shanghai)
		if err != nil {
			return nil, httpx.BadRequest("date 须为 YYYY-MM-DD")
		}
		day = d
	}
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, approval.Shanghai)
	sm, err := s.store.summary(ctx, tenantID, from, from.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	sm.Date = from.Format("2006-01-02")
	sm.DistanceKm = round1(sm.DistanceKm)
	sm.EnergyKwh = round2(sm.EnergyKwh)
	return &sm, nil
}

// Overview builds the dashboard payload.
func (s *Service) Overview(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal) (*Overview, error) {
	var (
		ov  Overview
		err error
	)
	if ov.Vehicles, err = s.store.vehicleCounts(ctx, tenantID); err != nil {
		return nil, err
	}
	if ov.ApprovalsTodo, err = approval.TodoCount(ctx, s.app.DB, tenantID, actor.UserID); err != nil {
		return nil, err
	}
	if actor.Has(approval.PermManage) {
		if ov.ApprovalsPending, err = approval.PendingCount(ctx, s.app.DB, tenantID); err != nil {
			return nil, err
		}
	}
	today, err := s.Summary(ctx, tenantID, "")
	if err != nil {
		return nil, err
	}
	ov.Today = *today
	if ov.Devices, err = s.store.deviceCounts(ctx, tenantID); err != nil {
		return nil, err
	}
	if ov.RecentEvents, err = s.store.recentEvents(ctx, tenantID, 10); err != nil {
		return nil, err
	}
	return &ov, nil
}

// Export returns up to 5000 trips matching the filters, newest first.
func (s *Service) Export(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, q ListQuery) ([]Trip, error) {
	f, err := s.filter(actor, q)
	if err != nil {
		return nil, err
	}
	rows, _, err := s.store.list(ctx, tenantID, f, pagination.Query{Page: 1, PageSize: exportMaxRows, SortBy: "start_at", SortDesc: true})
	if err != nil {
		return nil, err
	}
	out := make([]Trip, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTrip(r))
	}
	return out, nil
}

func toTrip(r row) Trip {
	t := Trip{
		ID: r.ID, TenantID: r.TenantID, TripNo: r.TripNo,
		Vehicle: approval.VehicleBrief{
			ID: r.VehicleID, PlateNo: r.PlateNo, Brand: r.VehicleBrand, Model: r.VehicleModel, Status: r.VehicleStatus,
			SOC: r.VehicleSOC, RangeKm: r.VehicleRangeKm, HomeDeptID: r.VehicleHomeDeptID, HomeDeptName: r.VehicleHomeDeptName,
		},
		CardUID: r.CardUID, TripType: r.TripType, Purpose: r.Purpose, Source: r.Source, Status: r.Status,
		StartAt: r.StartAt, EndAt: r.EndAt, StartOdometer: r.StartOdometer, EndOdometer: r.EndOdometer,
		DistanceKm: r.DistanceKm, EnergyKwh: r.EnergyKwh, StartSOC: r.StartSOC, EndSOC: r.EndSOC,
		AvgSpeed: r.AvgSpeed, MaxSpeed: r.MaxSpeed, HarshAccel: r.HarshAccel, HarshBrake: r.HarshBrake, PointCount: r.PointCount,
		RoofSignStatus: r.RoofSignStatus, DeviationFlag: r.DeviationFlag, DeviationMaxM: r.DeviationMaxM,
		StartLng: r.StartLng, StartLat: r.StartLat, EndLng: r.EndLng, EndLat: r.EndLat, Cost: r.Cost, Remark: r.Remark,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if len(r.CostDetail) > 0 {
		t.CostDetail = json.RawMessage(r.CostDetail)
	}
	if r.DriverID != nil && r.DriverName != nil {
		t.Driver = &approval.UserBrief{ID: *r.DriverID, Name: *r.DriverName, Username: strDeref(r.DriverUsername), DeptName: r.DriverDeptName, Phone: r.DriverPhone}
	}
	if r.ApprovalID != nil && r.ApplyNo != nil {
		t.Approval = &ApprovalRef{ID: *r.ApprovalID, ApplyNo: *r.ApplyNo, PurposeDetail: strDeref(r.ApprovalPurpose), PlannedKm: r.ApprovalPlannedKm, Destination: strDeref(r.ApprovalDestination)}
	}
	end := time.Now()
	if r.EndAt != nil {
		end = *r.EndAt
	}
	d := round1(end.Sub(r.StartAt).Minutes())
	t.DurationMin = &d
	if r.DistanceKm != nil && r.EnergyKwh != nil && *r.DistanceKm > 0 {
		v := round2(*r.EnergyKwh / *r.DistanceKm * 100)
		t.EnergyPer100Km = &v
	}
	return t
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---- start

// Start opens a trip from the Web (dispatcher) for an approved request.
func (s *Service) Start(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, req StartRequest) (*Trip, error) {
	a, err := s.store.approvalByID(ctx, tenantID, req.ApprovalID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, httpx.NotFound("申请不存在")
	}
	if a.Status != approval.StatusApproved {
		return nil, httpx.Conflict("申请状态不是已批准：" + a.Status)
	}
	if a.VehicleID == nil {
		return nil, httpx.Conflict("申请尚未指派车辆")
	}
	v, err := s.store.vehicle(ctx, *a.VehicleID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, httpx.Conflict("车辆不存在")
	}
	if v.Status != vstatus.Idle && v.Status != vstatus.Charging {
		return nil, httpx.Conflict("车辆当前不可出车：" + v.Status)
	}
	driverID := a.ApplicantID
	if req.DriverID != nil {
		ok, err := s.store.userActive(ctx, tenantID, *req.DriverID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.BadRequest("驾驶员不存在或已停用")
		}
		driverID = *req.DriverID
	}
	id, err := s.open(ctx, openParams{
		tenantID: tenantID, approvalID: a.ID, vehicleID: v.ID, driverID: driverID, tripType: a.TripType, purpose: a.PurposeDetail,
		source: SourceWeb, startAt: time.Now(), remark: req.Remark,
	}, a.ApplyNo, v.PlateNo)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, actor, id)
}

// open is the common start path (Web and device): snapshot the vehicle's
// live numbers as start values, insert, flip vehicle/approval state, notify.
func (s *Service) open(ctx context.Context, p openParams, applyNo, plateNo string) (uuid.UUID, error) {
	live, err := s.vs.Get(ctx, p.vehicleID)
	if err != nil {
		return uuid.Nil, err
	}
	if live != nil {
		if p.odometer == nil {
			p.odometer = live.OdometerKm
		}
		if p.soc == nil {
			p.soc = live.SOC
		}
		if p.lng == nil && p.lat == nil {
			p.lng, p.lat = live.Lng, live.Lat
		}
	}
	id, tripNo, err := s.store.open(ctx, p)
	if err != nil {
		var pgErr *pgconn.PgError
		switch {
		case errors.Is(err, errApprovalNotApproved):
			return uuid.Nil, httpx.Conflict("申请状态不是已批准")
		case errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_trips_ongoing_vehicle":
			return uuid.Nil, httpx.Conflict("车辆已有进行中的行程")
		}
		return uuid.Nil, err
	}
	driver := p.driverID
	if _, err := s.vs.SetStatus(ctx, p.vehicleID, vstatus.InUse, &id, &driver); err != nil {
		return uuid.Nil, err
	}
	s.event(ctx, p.tenantID, id, p.vehicleID, EvStart, p.startAt, map[string]any{
		"lng": p.lng, "lat": p.lat, "odometer_km": p.odometer, "soc": p.soc, "source": p.source, "trip_no": tripNo,
	}, plateNo)
	s.send(ctx, p.tenantID, []uuid.UUID{p.driverID}, notify.TypeTripStarted, "行程已开始："+tripNo,
		fmt.Sprintf("车辆 %s 于 %s 出发（申请 %s）", plateNo, fmtTime(p.startAt), applyNo), id)
	s.app.Hub.Publish(p.tenantID, ws.Event{Type: ws.EvTripStarted, Data: map[string]any{"trip_id": id, "trip_no": tripNo, "vehicle_id": p.vehicleID, "driver_id": p.driverID, "plate_no": plateNo}})
	approval.PublishUpdated(s.app.Hub, p.tenantID, p.approvalID, approval.StatusInUse, applyNo)
	return id, nil
}

// ---- device hooks

// StartFromDevice implements telemetry.TripHooks.DeviceTripStart.
func (s *Service) StartFromDevice(ctx context.Context, dev telemetry.Device, ev telemetry.TripStartEvent) (*telemetry.TripStartResult, error) {
	if dev.VehicleID == nil {
		return nil, httpx.Conflict("设备未绑定车辆")
	}
	tenantID := dev.TenantID
	ts := ev.TS
	if ts.IsZero() {
		ts = time.Now()
	}

	// driver
	var (
		driverID uuid.UUID
		cardID   *uuid.UUID
	)
	switch {
	case strings.TrimSpace(ev.CardUID) != "":
		card, err := s.store.cardByUID(ctx, tenantID, strings.TrimSpace(ev.CardUID))
		if err != nil {
			return nil, err
		}
		switch {
		case card == nil:
			return nil, httpx.Forbidden("卡不存在")
		case card.Status == "lost":
			return nil, httpx.Forbidden("卡已挂失")
		case card.Status != "active":
			return nil, httpx.Forbidden("卡已停用")
		case card.UserID == nil:
			return nil, httpx.Forbidden("卡未绑定用户")
		}
		driverID, cardID = *card.UserID, &card.ID
	case ev.UserID != nil:
		driverID = *ev.UserID
	default:
		return nil, httpx.BadRequest("缺少驾驶员标识（card_uid 或 user_id）")
	}
	if ok, err := s.store.userActive(ctx, tenantID, driverID); err != nil {
		return nil, err
	} else if !ok {
		return nil, httpx.Forbidden("驾驶员不存在或已停用")
	}

	// vehicle
	v, err := s.store.vehicle(ctx, *dev.VehicleID)
	if err != nil {
		return nil, err
	}
	if v == nil || v.TenantID != tenantID {
		return nil, httpx.Conflict("设备绑定的车辆不存在")
	}
	switch v.Status {
	case vstatus.InUse:
		return nil, httpx.Conflict("车辆已在使用中")
	case vstatus.Maintenance, vstatus.Disabled:
		return nil, httpx.Conflict("车辆不可用：" + v.Status)
	}

	// approval
	var a *approvalLite
	if no := strings.TrimSpace(ev.ApprovalNo); no != "" {
		if a, err = s.store.approvalByNo(ctx, tenantID, no); err != nil {
			return nil, err
		}
		switch {
		case a == nil:
			return nil, httpx.Forbidden("申请不存在：" + no)
		case a.Status != approval.StatusApproved:
			return nil, httpx.Forbidden("申请未批准或已使用：" + a.Status)
		case a.VehicleID == nil || *a.VehicleID != v.ID:
			return nil, httpx.Forbidden("申请指派的车辆与本车不符")
		case a.ApplicantID != driverID && !a.hasPassenger(driverID):
			return nil, httpx.Forbidden("驾驶员不是该申请的申请人或同行人")
		}
	} else {
		if a, err = s.store.matchApproval(ctx, tenantID, v.ID, driverID, ts); err != nil {
			return nil, err
		}
		if a == nil {
			return nil, httpx.Forbidden("无有效用车申请")
		}
	}

	id, err := s.open(ctx, openParams{
		tenantID: tenantID, approvalID: a.ID, vehicleID: v.ID, driverID: driverID, cardID: cardID,
		tripType: a.TripType, purpose: a.PurposeDetail, source: SourceDevice, startAt: ts,
		odometer: ev.OdometerKm, soc: ev.SOC, lng: ev.Lng, lat: ev.Lat,
	}, a.ApplyNo, v.PlateNo)
	if err != nil {
		return nil, err
	}
	signOn := a.TripType == approval.TripTypeOfficial
	if err := s.store.setSignOn(ctx, v.ID, signOn); err != nil {
		return nil, err
	}
	if signOn {
		s.event(ctx, tenantID, id, v.ID, EvSignOn, ts, map[string]any{"sign_on": true, "source": "trip_start"}, v.PlateNo)
	}
	var tripNo string
	_ = s.app.DB.QueryRow(ctx, `SELECT trip_no FROM trips WHERE id = $1`, id).Scan(&tripNo)
	return &telemetry.TripStartResult{TripID: id, TripNo: tripNo, TripType: a.TripType, SignOn: signOn, DriverID: driverID}, nil
}

// EndFromDevice implements telemetry.TripHooks.DeviceTripEnd.
func (s *Service) EndFromDevice(ctx context.Context, dev telemetry.Device, ev telemetry.TripEndEvent) (uuid.UUID, error) {
	if dev.VehicleID == nil {
		return uuid.Nil, httpx.Conflict("设备未绑定车辆")
	}
	og, err := s.store.ongoingByVehicle(ctx, *dev.VehicleID)
	if err != nil {
		return uuid.Nil, err
	}
	if og == nil || og.TenantID != dev.TenantID {
		return uuid.Nil, httpx.Conflict("车辆没有进行中的行程")
	}
	ts := ev.TS
	if ts.IsZero() {
		ts = time.Now()
	}
	if err := s.End(ctx, og.ID, EndInput{EndAt: ts, EndOdometer: ev.OdometerKm, EndSOC: ev.SOC, Lng: ev.Lng, Lat: ev.Lat}); err != nil {
		return uuid.Nil, err
	}
	return og.ID, nil
}

// ---- end / cancel

// EndInput is what the Web handler or the device supplies when closing a trip.
type EndInput struct {
	EndAt       time.Time
	EndOdometer *float64
	EndSOC      *float64
	Lng, Lat    *float64
	Remark      *string
}

// End closes an ongoing trip: fills the end readings (request → live status →
// last track point), computes the statistics, releases the vehicle and
// completes the approval.
func (s *Service) End(ctx context.Context, id uuid.UUID, in EndInput) error {
	r, err := s.getAny(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return httpx.NotFound("行程不存在")
	}
	if r.Status != StatusOngoing {
		return httpx.Conflict("行程不在进行中：" + r.Status)
	}
	endAt := in.EndAt
	if endAt.IsZero() {
		endAt = time.Now()
	}
	if endAt.Before(r.StartAt) {
		endAt = r.StartAt
	}
	live, err := s.vs.Get(ctx, r.VehicleID)
	if err != nil {
		return err
	}
	pts, err := s.store.points(ctx, id)
	if err != nil {
		return err
	}
	endOdo, endSOC, endLng, endLat := in.EndOdometer, in.EndSOC, in.Lng, in.Lat
	if live != nil {
		if endOdo == nil {
			endOdo = live.OdometerKm
		}
		if endSOC == nil {
			endSOC = live.SOC
		}
		if endLng == nil && endLat == nil {
			endLng, endLat = live.Lng, live.Lat
		}
	}
	if n := len(pts); n > 0 {
		last := pts[n-1]
		if endSOC == nil {
			endSOC = last.SOC
		}
		if endLng == nil && endLat == nil {
			endLng, endLat = &last.Lng, &last.Lat
		}
	}
	var battery float64
	if v, err := s.store.vehicle(ctx, r.VehicleID); err != nil {
		return err
	} else if v != nil {
		battery = v.BatteryKwh
	}
	route, err := s.store.plannedRoute(ctx, r.ApprovalID)
	if err != nil {
		return err
	}
	st := Summarize(r.StartAt, endAt, r.StartOdometer, endOdo, r.StartSOC, endSOC, battery, pts, route)

	signOn, err := s.store.hasEvent(ctx, id, EvSignOn)
	if err != nil {
		return err
	}
	roof := "unknown"
	switch {
	case signOn:
		roof = "on"
	case r.TripType == approval.TripTypeOfficial:
		roof = "off"
	}
	if st.DeviationFlag {
		if has, err := s.store.hasEvent(ctx, id, EvDeviation); err != nil {
			return err
		} else if !has {
			s.event(ctx, r.TenantID, id, r.VehicleID, EvDeviation, endAt, map[string]any{"distance_m": *st.DeviationMaxM, "source": "summary"}, r.PlateNo)
		}
	}
	err = s.store.close(ctx, closeParams{
		id: id, endAt: endAt, endOdometer: endOdo, endSOC: endSOC, endLng: endLng, endLat: endLat, remark: in.Remark, stats: st, roofSign: roof,
	}, r.ApprovalID)
	if errors.Is(err, errNotOngoing) {
		return httpx.Conflict("行程不在进行中")
	}
	if err != nil {
		return err
	}
	if _, err := s.vs.SetStatus(ctx, r.VehicleID, vstatus.Idle, nil, nil); err != nil {
		return err
	}
	_ = s.store.setSignOn(ctx, r.VehicleID, false)
	s.event(ctx, r.TenantID, id, r.VehicleID, EvEnd, endAt, map[string]any{
		"lng": endLng, "lat": endLat, "odometer_km": endOdo, "soc": endSOC, "distance_km": st.DistanceKm, "energy_kwh": st.EnergyKwh,
	}, r.PlateNo)
	if r.DriverID != nil {
		s.send(ctx, r.TenantID, []uuid.UUID{*r.DriverID}, notify.TypeTripEnded, "行程已结束："+r.TripNo,
			fmt.Sprintf("车辆 %s 行程结束，里程 %s km，耗电 %s kWh", r.PlateNo, fmtNum(st.DistanceKm, 1), fmtNum(st.EnergyKwh, 2)), id)
	}
	s.app.Hub.Publish(r.TenantID, ws.Event{Type: ws.EvTripEnded, Data: map[string]any{"trip_id": id, "trip_no": r.TripNo, "vehicle_id": r.VehicleID, "distance_km": st.DistanceKm, "energy_kwh": st.EnergyKwh}})
	if r.ApprovalID != nil {
		approval.PublishUpdated(s.app.Hub, r.TenantID, *r.ApprovalID, approval.StatusCompleted, strDeref(r.ApplyNo))
	}
	// 计费：由 billing 模块在接线时注入；失败不影响行程结束，账单可由结算流程补算
	if CompletedHook != nil {
		if err := CompletedHook(ctx, r.TenantID, id); err != nil {
			s.app.Log.Warn().Err(err).Str("trip_id", id.String()).Msg("trip billing hook failed")
		}
	}
	return nil
}

// CompletedHook is called after a trip is closed and summarized (billing).
// Injected by the server wiring to avoid an import cycle (billing reads trips).
var CompletedHook func(ctx context.Context, tenantID, tripID uuid.UUID) error

// getAny loads a trip by id regardless of tenant (device path).
func (s *Service) getAny(ctx context.Context, id uuid.UUID) (*row, error) {
	var tenantID uuid.UUID
	err := s.app.DB.QueryRow(ctx, `SELECT tenant_id FROM trips WHERE id = $1`, id).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.store.get(ctx, tenantID, id)
}

// EndWeb is the /trips/{id}/end entry: tenant + permission checks then End.
func (s *Service) EndWeb(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID, req EndRequest) (*Trip, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("行程不存在")
	}
	if err := s.End(ctx, id, EndInput{EndAt: time.Now(), EndOdometer: req.EndOdometer, EndSOC: req.EndSOC, Remark: req.Remark}); err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, actor, id)
}

// Cancel voids an ongoing trip (mis-trigger): vehicle back to idle, approval back to approved.
func (s *Service) Cancel(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, id uuid.UUID, reason *string) (*Trip, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("行程不存在")
	}
	if r.Status != StatusOngoing {
		return nil, httpx.Conflict("行程不在进行中：" + r.Status)
	}
	err = s.store.cancel(ctx, id, r.ApprovalID, reason)
	if errors.Is(err, errNotOngoing) {
		return nil, httpx.Conflict("行程不在进行中")
	}
	if err != nil {
		return nil, err
	}
	if _, err := s.vs.SetStatus(ctx, r.VehicleID, vstatus.Idle, nil, nil); err != nil {
		return nil, err
	}
	_ = s.store.setSignOn(ctx, r.VehicleID, false)
	s.event(ctx, tenantID, id, r.VehicleID, EvCancel, time.Now(), map[string]any{"reason": reason, "by": actor.UserID}, r.PlateNo)
	s.app.Hub.Publish(tenantID, ws.Event{Type: ws.EvTripEnded, Data: map[string]any{"trip_id": id, "trip_no": r.TripNo, "vehicle_id": r.VehicleID, "cancelled": true}})
	if r.ApprovalID != nil {
		approval.PublishUpdated(s.app.Hub, tenantID, *r.ApprovalID, approval.StatusApproved, strDeref(r.ApplyNo))
	}
	return s.Get(ctx, tenantID, actor, id)
}

// ---- telemetry

// OnTelemetry implements telemetry.TripHooks.OnTelemetry: appends the points
// to the vehicle's ongoing trip and evaluates overspeed / low SOC / route
// deviation / roof-sign changes. At most one event per type per batch, and
// never within EventCooldown of the previous event of that type.
func (s *Service) OnTelemetry(ctx context.Context, tenantID, vehicleID uuid.UUID, pts []telemetry.Point) error {
	og, err := s.store.ongoingByVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	if og == nil || (tenantID != uuid.Nil && og.TenantID != tenantID) || len(pts) == 0 {
		return nil
	}
	sorted := append([]telemetry.Point(nil), pts...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].TS.Before(sorted[j].TS) })

	track := make([]TrackPoint, 0, len(sorted))
	for _, p := range sorted {
		if p.Lng == nil || p.Lat == nil {
			continue
		}
		track = append(track, TrackPoint{TS: p.TS, Lng: *p.Lng, Lat: *p.Lat, Speed: p.Speed, Heading: p.Heading, SOC: p.SOC})
	}
	if err := s.store.insertPoints(ctx, og.ID, og.VehicleID, track); err != nil {
		return err
	}

	last, err := s.store.lastEventTimes(ctx, og.ID)
	if err != nil {
		return err
	}
	fired := map[string]bool{}
	canFire := func(typ string, ts time.Time) bool {
		if fired[typ] {
			return false
		}
		if prev, ok := last[typ]; ok && ts.Sub(prev) < EventCooldown {
			return false
		}
		return true
	}
	fire := func(typ string, ts time.Time, payload map[string]any, alert bool) {
		fired[typ] = true
		last[typ] = ts
		e := s.event(ctx, og.TenantID, og.ID, og.VehicleID, typ, ts, payload, og.PlateNo)
		if alert && e != nil {
			s.alert(ctx, og, e)
		}
	}

	limit := float64(param.GetInt(ctx, s.app.DB, og.TenantID, ParamOverspeed, DefaultOverspeed))
	lowSOC := float64(param.GetInt(ctx, s.app.DB, og.TenantID, param.KeyLowSocLock, DefaultLowSOCLock) + 10)
	route := og.route()
	signOn := last[EvSignOn].After(last[EvSignOff]) // known roof-sign state from the event log
	var maxDev float64

	for _, p := range sorted {
		if p.Speed != nil && *p.Speed > limit && canFire(EvOverspeed, p.TS) {
			fire(EvOverspeed, p.TS, map[string]any{"speed": *p.Speed, "limit": limit, "lng": p.Lng, "lat": p.Lat}, true)
		}
		if p.SOC != nil && *p.SOC < lowSOC && canFire(EvLowSOC, p.TS) {
			fire(EvLowSOC, p.TS, map[string]any{"soc": *p.SOC, "threshold": lowSOC, "lng": p.Lng, "lat": p.Lat}, true)
		}
		if len(route) >= 2 && p.Lng != nil && p.Lat != nil {
			if d := DistanceToPolyline(*p.Lng, *p.Lat, route); !math.IsInf(d, 1) {
				maxDev = math.Max(maxDev, d)
				if d > DeviationMeters && canFire(EvDeviation, p.TS) {
					fire(EvDeviation, p.TS, map[string]any{"distance_m": round1(d), "lng": *p.Lng, "lat": *p.Lat}, true)
				}
			}
		}
		if p.SignOn != nil && *p.SignOn != signOn {
			typ := EvSignOff
			if *p.SignOn {
				typ = EvSignOn
			}
			if canFire(typ, p.TS) {
				fire(typ, p.TS, map[string]any{"sign_on": *p.SignOn, "lng": p.Lng, "lat": p.Lat}, false)
				signOn = *p.SignOn
			}
		}
	}
	if maxDev > 0 {
		// keep the running maximum; the flag is only raised beyond the threshold
		if err := s.store.markDeviation(ctx, og.ID, round1(maxDev), maxDev > DeviationMeters); err != nil {
			return err
		}
	}
	return nil
}

// alert notifies the driver and the first-level approver about an abnormal event.
func (s *Service) alert(ctx context.Context, og *ongoingRow, e *Event) {
	users := []uuid.UUID{}
	if og.DriverID != nil {
		users = append(users, *og.DriverID)
	}
	if og.L1ApproverID != nil {
		users = append(users, *og.L1ApproverID)
	}
	var title, content string
	var payload map[string]any
	_ = json.Unmarshal(e.Payload, &payload)
	switch e.Type {
	case EvOverspeed:
		title = "超速提醒：" + og.PlateNo
		content = fmt.Sprintf("行程 %s 车速 %v km/h，超过限值 %v km/h", og.TripNo, payload["speed"], payload["limit"])
	case EvLowSOC:
		title = "低电量提醒：" + og.PlateNo
		content = fmt.Sprintf("行程 %s 电量 %v%%，低于 %v%%", og.TripNo, payload["soc"], payload["threshold"])
	case EvDeviation:
		title = "路线偏离提醒：" + og.PlateNo
		content = fmt.Sprintf("行程 %s 偏离计划路线 %v 米", og.TripNo, payload["distance_m"])
	default:
		return
	}
	s.send(ctx, og.TenantID, users, notify.TypeTripEvent, title, content, og.ID)
}

// ---- side effects

func (s *Service) event(ctx context.Context, tenantID, tripID, vehicleID uuid.UUID, typ string, ts time.Time, payload map[string]any, plateNo string) *Event {
	e, err := s.store.insertEvent(ctx, tenantID, tripID, vehicleID, typ, ts, payload)
	if err != nil {
		s.app.Log.Error().Err(err).Str("trip", tripID.String()).Str("type", typ).Msg("trip event insert failed")
		return nil
	}
	s.app.Hub.Publish(tenantID, ws.Event{Type: ws.EvTripEvent, Data: EventWithVehicle{Event: *e, PlateNo: plateNo}})
	return e
}

func (s *Service) send(ctx context.Context, tenantID uuid.UUID, users []uuid.UUID, typ, title, content string, tripID uuid.UUID) {
	if err := s.notify.Send(ctx, tenantID, users, notify.Message{Type: typ, Title: title, Content: content, RefType: "trip", RefID: tripID.String()}); err != nil {
		s.app.Log.Error().Err(err).Str("trip", tripID.String()).Msg("trip notify failed")
	}
}

func fmtTime(t time.Time) string { return t.In(approval.Shanghai).Format("01-02 15:04") }

func fmtNum(v *float64, prec int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.*f", prec, *v)
}
