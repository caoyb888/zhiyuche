package booking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type Service struct {
	app   *app.App
	store *store
	vs    *vstatus.Store
}

func NewService(a *app.App) *Service {
	return &Service{app: a, store: &store{db: a.DB}, vs: vstatus.New(a)}
}

// ---- read

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) (pagination.Page[Booking], error) {
	f := listFilter{ListQuery: q}
	var err error
	if f.from, err = parseTime(q.From, "from"); err != nil {
		return pagination.Page[Booking]{}, err
	}
	if f.to, err = parseTime(q.To, "to"); err != nil {
		return pagination.Page[Booking]{}, err
	}
	if q.DeptID != "" {
		id := uuid.MustParse(q.DeptID)
		f.deptID = &id
	}
	rows, total, err := s.store.list(ctx, tenantID, f, pg)
	if err != nil {
		return pagination.Page[Booking]{}, err
	}
	items := make([]Booking, 0, len(rows))
	for i := range rows {
		items = append(items, toAPI(rows[i]))
	}
	return pagination.NewPage(items, total, pg), nil
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

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Booking, error) {
	r, err := s.store.get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, httpx.NotFound("预约不存在")
	}
	b := toAPI(*r)
	return &b, nil
}

func (s *Service) AvailableVehicles(ctx context.Context, tenantID uuid.UUID, q AvailableQuery) ([]approval.VehicleBrief, error) {
	if err := ValidateWindow(q.Start, q.End, time.Now()); err != nil {
		return nil, httpx.BadRequest(err.Error())
	}
	exclude := uuid.Nil
	if q.ExcludeBookingID != "" {
		exclude = uuid.MustParse(q.ExcludeBookingID)
	}
	return s.store.availableVehicles(ctx, tenantID, q.Start, q.End, exclude)
}

// ---- write

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, actor *auth.Principal, req CreateRequest) (*Booking, error) {
	if err := ValidateWindow(req.ReserveStart, req.ReserveEnd, time.Now()); err != nil {
		return nil, httpx.BadRequest(err.Error())
	}
	deptID, err := s.passengerDept(ctx, tenantID, req.PassengerID)
	if err != nil {
		return nil, err
	}
	if req.VehicleID != nil {
		if err := s.checkVehicle(ctx, tenantID, *req.VehicleID, req.ReserveStart, req.ReserveEnd, uuid.Nil); err != nil {
			return nil, err
		}
	}
	id, _, err := s.store.insert(ctx, insertParams{tenantID: tenantID, req: req, deptID: deptID, createdBy: actor.UserID})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, id)
}

// Update edits a booking that has not left yet. Absent keys keep their value;
// clear_passenger / clear_vehicle set the field back to empty.
func (s *Service) Update(ctx context.Context, tenantID uuid.UUID, id uuid.UUID, req UpdateRequest) (*Booking, *Booking, error) {
	before, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if !Editable(before.Status) {
		return nil, nil, httpx.Conflict(StatusLabel(before.Status) + "的预约不可修改")
	}

	// effective window after the edit, for the conflict check
	start, end := before.ReserveStart, before.ReserveEnd
	if req.ReserveStart != nil {
		start = *req.ReserveStart
	}
	if req.ReserveEnd != nil {
		end = *req.ReserveEnd
	}
	windowChanged := req.ReserveStart != nil || req.ReserveEnd != nil
	if windowChanged {
		if err := ValidateWindow(start, end, time.Now()); err != nil {
			return nil, nil, httpx.BadRequest(err.Error())
		}
	}

	sets := []string{}
	args := []any{}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)+2)) // $1 tenant, $2 id
	}

	if req.Source != nil {
		set("source", *req.Source)
	}
	if req.ContactName != nil {
		set("contact_name", strings.TrimSpace(*req.ContactName))
	}
	if req.ContactPhone != nil {
		set("contact_phone", Trimmed(req.ContactPhone))
	}
	switch {
	case req.ClearPassenger:
		set("passenger_id", nil)
		set("dept_id", nil)
	case req.PassengerID != nil:
		deptID, err := s.passengerDept(ctx, tenantID, req.PassengerID)
		if err != nil {
			return nil, nil, err
		}
		set("passenger_id", *req.PassengerID)
		set("dept_id", deptID)
	}
	if req.ReserveStart != nil {
		set("reserve_start", start)
	}
	if req.ReserveEnd != nil {
		set("reserve_end", end)
	}

	// 车辆冲突：改派了车，或车不变但时段变了，都要重新校验
	vehicleID := before.vehicleID()
	switch {
	case req.ClearVehicle:
		set("vehicle_id", nil)
		vehicleID = nil
	case req.VehicleID != nil:
		vehicleID = req.VehicleID
		set("vehicle_id", *req.VehicleID)
	}
	if vehicleID != nil && (req.VehicleID != nil || windowChanged) {
		if err := s.checkVehicle(ctx, tenantID, *vehicleID, start, end, id); err != nil {
			return nil, nil, err
		}
	}

	if req.Origin != nil {
		set("origin", strings.TrimSpace(*req.Origin))
	}
	if req.OriginLng != nil {
		set("origin_lng", *req.OriginLng)
	}
	if req.OriginLat != nil {
		set("origin_lat", *req.OriginLat)
	}
	if req.Destination != nil {
		set("destination", strings.TrimSpace(*req.Destination))
	}
	if req.DestLng != nil {
		set("dest_lng", *req.DestLng)
	}
	if req.DestLat != nil {
		set("dest_lat", *req.DestLat)
	}
	if req.Purpose != nil {
		set("purpose", Trimmed(req.Purpose))
	}
	if req.Remark != nil {
		set("remark", Trimmed(req.Remark))
	}

	if err := s.store.update(ctx, tenantID, id, sets, args); err != nil {
		return nil, nil, err
	}
	after, err := s.Get(ctx, tenantID, id)
	return before, after, err
}

// Transition applies depart / complete / cancel. 出车占用车辆（vehicles.status
// → in_use），完成后再释放；取消只发生在出车前，不涉及车辆状态。
func (s *Service) Transition(ctx context.Context, tenantID, id uuid.UUID, event string, reason *string) (*Booking, error) {
	b, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	next, err := NextStatus(b.Status, event)
	if err != nil {
		return nil, httpx.Conflict(StatusLabel(b.Status) + "的预约不能" + eventLabel(event))
	}
	if next == StatusDeparted {
		if b.Vehicle == nil {
			return nil, httpx.Conflict("请先派车再确认出车")
		}
		if !CanDepart(b.Vehicle.Status) {
			return nil, httpx.Conflict("车辆 " + b.Vehicle.PlateNo + " 当前不可出车：" + VehicleStatusLabel(b.Vehicle.Status))
		}
	}
	if err := s.store.setStatus(ctx, tenantID, id, next, Trimmed(reason)); err != nil {
		return nil, err
	}
	switch next {
	case StatusDeparted:
		// driver_id 记用车人（可能为空），车辆快照与 WebSocket 推送由 vstatus 负责
		if _, err := s.vs.SetStatus(ctx, b.Vehicle.ID, vstatus.InUse, nil, passengerID(b)); err != nil {
			return nil, err
		}
	case StatusCompleted:
		if err := s.releaseVehicle(ctx, b); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, tenantID, id)
}

// releaseVehicle puts the vehicle back to idle after the booking finished —
// but only while it is still this booking that holds it. A trip started in the
// meantime (current_trip_id) owns the vehicle, and a car that went to charging,
// maintenance or disabled must keep that status.
func (s *Service) releaseVehicle(ctx context.Context, b *Booking) error {
	if b.Vehicle == nil {
		return nil
	}
	live, err := s.vs.Get(ctx, b.Vehicle.ID)
	if err != nil {
		return err
	}
	if live == nil || live.Status != vstatus.InUse || live.CurrentTripID != nil {
		return nil
	}
	_, err = s.vs.SetStatus(ctx, b.Vehicle.ID, vstatus.Idle, nil, nil)
	return err
}

// passengerID is the 用车人 recorded on the booking, used as the vehicle's
// driver in the live snapshot (nil when the booking has none).
func passengerID(b *Booking) *uuid.UUID {
	if b.Passenger == nil {
		return nil
	}
	id := b.Passenger.ID
	return &id
}

func eventLabel(event string) string {
	switch event {
	case EvDepart:
		return "出车"
	case EvComplete:
		return "完成"
	case EvCancel:
		return "取消"
	}
	return event
}

// ---- helpers

// passengerDept resolves the department snapshot of the optional passenger.
func (s *Service) passengerDept(ctx context.Context, tenantID uuid.UUID, passengerID *uuid.UUID) (*uuid.UUID, error) {
	if passengerID == nil {
		return nil, nil
	}
	u, err := s.store.passenger(ctx, tenantID, *passengerID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, httpx.BadRequest("用车人不存在")
	}
	if u.Status != "active" {
		return nil, httpx.BadRequest("用车人已停用")
	}
	return u.DeptID, nil
}

// checkVehicle verifies the vehicle is usable and free in [start, end).
func (s *Service) checkVehicle(ctx context.Context, tenantID, vehicleID uuid.UUID, start, end time.Time, excludeBooking uuid.UUID) error {
	v, err := s.store.vehicle(ctx, tenantID, vehicleID)
	if err != nil {
		return err
	}
	if v == nil || v.Deleted {
		return httpx.BadRequest("车辆不存在")
	}
	switch v.Status {
	case "maintenance":
		return httpx.Conflict("车辆 " + v.PlateNo + " 维保中，不能派车")
	case "disabled":
		return httpx.Conflict("车辆 " + v.PlateNo + " 已停用，不能派车")
	}
	cs, err := s.store.conflicts(ctx, tenantID, vehicleID, start, end, excludeBooking)
	if err != nil {
		return err
	}
	if len(cs) > 0 {
		return httpx.Conflict(ConflictMessage(v.PlateNo, cs))
	}
	return nil
}

// vehicleID is the assigned vehicle's id, nil when still 待派车.
func (b *Booking) vehicleID() *uuid.UUID {
	if b.Vehicle == nil {
		return nil
	}
	id := b.Vehicle.ID
	return &id
}

// toAPI flattens a joined row into the API shape.
func toAPI(r row) Booking {
	b := Booking{
		ID:           r.ID,
		TenantID:     r.TenantID,
		BookingNo:    r.BookingNo,
		Source:       r.Source,
		ContactName:  r.ContactName,
		ContactPhone: r.ContactPhone,
		DeptID:       r.DeptID,
		DeptName:     r.DeptName,
		ReserveStart: r.ReserveStart,
		ReserveEnd:   r.ReserveEnd,
		Origin:       r.Origin,
		OriginLng:    r.OriginLng,
		OriginLat:    r.OriginLat,
		Destination:  r.Destination,
		DestLng:      r.DestLng,
		DestLat:      r.DestLat,
		Purpose:      r.Purpose,
		Remark:       r.Remark,
		Status:       r.Status,
		CancelReason: r.CancelReason,
		DepartedAt:   r.DepartedAt,
		CompletedAt:  r.CompletedAt,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if r.PassengerID != nil && r.PassengerName != nil {
		b.Passenger = &approval.UserBrief{
			ID:       *r.PassengerID,
			Name:     *r.PassengerName,
			Username: strOr(r.PassengerUsername),
			DeptName: r.PassengerDeptName,
			Phone:    r.PassengerPhone,
		}
	}
	if r.CreatedByID != nil && r.CreatorName != nil {
		b.CreatedBy = &approval.UserBrief{
			ID:       *r.CreatedByID,
			Name:     *r.CreatorName,
			Username: strOr(r.CreatorUsername),
			DeptName: r.CreatorDeptName,
			Phone:    r.CreatorPhone,
		}
	}
	if r.VehicleID != nil && r.VehiclePlate != nil {
		b.Vehicle = &approval.VehicleBrief{
			ID:           *r.VehicleID,
			PlateNo:      *r.VehiclePlate,
			Brand:        r.VehicleBrand,
			Model:        r.VehicleModel,
			Status:       strOr(r.VehicleStatus),
			SOC:          r.VehicleSOC,
			RangeKm:      r.VehicleRangeKm,
			HomeDeptID:   r.VehicleHomeDeptID,
			HomeDeptName: r.VehicleHomeDeptName,
		}
	}
	return b
}

func strOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
