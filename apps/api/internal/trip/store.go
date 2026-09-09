package trip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type store struct{ db *pgxpool.Pool }

// row is the flat scan target of the trip list/detail query.
type row struct {
	ID             uuid.UUID  `db:"id"`
	TenantID       uuid.UUID  `db:"tenant_id"`
	TripNo         string     `db:"trip_no"`
	VehicleID      uuid.UUID  `db:"vehicle_id"`
	DriverID       *uuid.UUID `db:"driver_id"`
	CardID         *uuid.UUID `db:"card_id"`
	ApprovalID     *uuid.UUID `db:"approval_id"`
	TripType       string     `db:"trip_type"`
	Purpose        *string    `db:"purpose"`
	Source         string     `db:"source"`
	Status         string     `db:"status"`
	StartAt        time.Time  `db:"start_at"`
	EndAt          *time.Time `db:"end_at"`
	StartOdometer  *float64   `db:"start_odometer"`
	EndOdometer    *float64   `db:"end_odometer"`
	DistanceKm     *float64   `db:"distance_km"`
	EnergyKwh      *float64   `db:"energy_kwh"`
	StartSOC       *float64   `db:"start_soc"`
	EndSOC         *float64   `db:"end_soc"`
	AvgSpeed       *float64   `db:"avg_speed"`
	MaxSpeed       *float64   `db:"max_speed"`
	HarshAccel     int        `db:"harsh_accel"`
	HarshBrake     int        `db:"harsh_brake"`
	PointCount     int        `db:"point_count"`
	RoofSignStatus string     `db:"roof_sign_status"`
	DeviationFlag  bool       `db:"deviation_flag"`
	DeviationMaxM  *float64   `db:"deviation_max_m"`
	StartLng       *float64   `db:"start_lng"`
	StartLat       *float64   `db:"start_lat"`
	EndLng         *float64   `db:"end_lng"`
	EndLat         *float64   `db:"end_lat"`
	Cost           *float64   `db:"cost"`
	CostDetail     []byte     `db:"cost_detail"`
	Remark         *string    `db:"remark"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	BillingStatus  string     `db:"billing_status"`
	AccountID      *uuid.UUID `db:"account_id"`
	AccountTxnID   *int64     `db:"account_txn_id"`
	BilledAt       *time.Time `db:"billed_at"`

	PlateNo             string     `db:"plate_no"`
	VehicleBrand        *string    `db:"vehicle_brand"`
	VehicleModel        *string    `db:"vehicle_model"`
	VehicleStatus       string     `db:"vehicle_status"`
	VehicleHomeDeptID   *uuid.UUID `db:"vehicle_home_dept_id"`
	VehicleHomeDeptName *string    `db:"vehicle_home_dept_name"`
	VehicleSOC          *float64   `db:"vehicle_soc"`
	VehicleRangeKm      *float64   `db:"vehicle_range_km"`

	DriverName     *string `db:"driver_name"`
	DriverUsername *string `db:"driver_username"`
	DriverPhone    *string `db:"driver_phone"`
	DriverDeptName *string `db:"driver_dept_name"`
	CardUID        *string `db:"card_uid"`

	ApplyNo             *string    `db:"apply_no"`
	ApprovalPurpose     *string    `db:"approval_purpose"`
	ApprovalPlannedKm   *float64   `db:"approval_planned_km"`
	ApprovalDestination *string    `db:"approval_destination"`
	ApprovalApplicantID *uuid.UUID `db:"approval_applicant_id"`
}

const baseSelect = `
	SELECT t.id, t.tenant_id, t.trip_no, t.vehicle_id, t.driver_id, t.card_id, t.approval_id, t.trip_type, t.purpose, t.source, t.status,
	       t.start_at, t.end_at, t.start_odometer, t.end_odometer, t.distance_km, t.energy_kwh, t.start_soc, t.end_soc,
	       t.avg_speed, t.max_speed, t.harsh_accel, t.harsh_brake, t.point_count, t.roof_sign_status, t.deviation_flag, t.deviation_max_m,
	       t.start_lng, t.start_lat, t.end_lng, t.end_lat, t.cost, t.cost_detail, t.remark, t.created_at, t.updated_at,
	       t.billing_status, t.account_id, t.account_txn_id, t.billed_at,
	       v.plate_no, v.brand AS vehicle_brand, v.model AS vehicle_model, v.status AS vehicle_status,
	       v.home_dept_id AS vehicle_home_dept_id, vd.name AS vehicle_home_dept_name, vs.soc AS vehicle_soc, vs.range_km AS vehicle_range_km,
	       u.name AS driver_name, u.username AS driver_username, u.phone AS driver_phone, ud.name AS driver_dept_name,
	       c.card_uid,
	       a.apply_no, a.purpose_detail AS approval_purpose, a.planned_km AS approval_planned_km, a.destination AS approval_destination,
	       a.applicant_id AS approval_applicant_id
	FROM trips t
	JOIN vehicles v ON v.id = t.vehicle_id
	LEFT JOIN departments vd ON vd.id = v.home_dept_id
	LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
	LEFT JOIN users u ON u.id = t.driver_id
	LEFT JOIN departments ud ON ud.id = u.dept_id
	LEFT JOIN nfc_cards c ON c.id = t.card_id
	LEFT JOIN approvals a ON a.id = t.approval_id
	WHERE t.tenant_id = $1`

var sortable = map[string]string{
	"start_at": "t.start_at", "distance_km": "t.distance_km", "energy_kwh": "t.energy_kwh", "max_speed": "t.max_speed",
}

type listFilter struct {
	ListQuery
	me   uuid.UUID
	from *time.Time
	to   *time.Time
}

func (s *store) listCond(f listFilter) (string, []any) {
	where := []string{}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)+1)) // $1 is tenant
	}
	if f.Scope != "all" {
		add("t.driver_id = $%d", f.me)
	}
	if f.Status != "" {
		add("t.status = $%d", f.Status)
	}
	if f.TripType != "" {
		add("t.trip_type = $%d", f.TripType)
	}
	if f.VehicleID != "" {
		add("t.vehicle_id = $%d", uuid.MustParse(f.VehicleID))
	}
	if f.DriverID != "" {
		add("t.driver_id = $%d", uuid.MustParse(f.DriverID))
	}
	if f.DeptID != "" {
		add("u.dept_id IN (SELECT id FROM departments WHERE deleted_at IS NULL AND path LIKE (SELECT path FROM departments WHERE id = $%d) || '%%')", uuid.MustParse(f.DeptID))
	}
	if f.from != nil {
		add("t.start_at >= $%d", *f.from)
	}
	if f.to != nil {
		add("t.start_at <= $%d", *f.to)
	}
	if f.Deviation != nil {
		add("t.deviation_flag = $%d", *f.Deviation)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		add("(t.trip_no ILIKE $%[1]d OR v.plate_no ILIKE $%[1]d OR u.name ILIKE $%[1]d OR t.purpose ILIKE $%[1]d OR a.purpose_detail ILIKE $%[1]d)", "%"+kw+"%")
	}
	if len(where) == 0 {
		return "", args
	}
	return " AND " + strings.Join(where, " AND "), args
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, f listFilter, pg pagination.Query) ([]row, int64, error) {
	cond, extra := s.listCond(f)
	args := append([]any{tenantID}, extra...)
	var total int64
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM trips t JOIN vehicles v ON v.id = t.vehicle_id
		LEFT JOIN users u ON u.id = t.driver_id LEFT JOIN approvals a ON a.id = t.approval_id
		WHERE t.tenant_id = $1`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "t.start_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
		order += " NULLS LAST"
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []row
	err := pgxscan.Select(ctx, s.db, &rows, fmt.Sprintf("%s%s ORDER BY %s, t.id LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*row, error) {
	var r row
	err := pgxscan.Get(ctx, s.db, &r, baseSelect+` AND t.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// approverIDs returns the approvers of the trip's approval (empty when none).
func (s *store) approverIDs(ctx context.Context, approvalID *uuid.UUID) ([]uuid.UUID, error) {
	if approvalID == nil {
		return nil, nil
	}
	var ids []uuid.UUID
	err := pgxscan.Select(ctx, s.db, &ids, `SELECT approver_id FROM approval_steps WHERE approval_id = $1 ORDER BY step_no`, *approvalID)
	return ids, err
}

// ---- events & points

func (s *store) events(ctx context.Context, tripID uuid.UUID) ([]Event, error) {
	var rows []Event
	err := pgxscan.Select(ctx, s.db, &rows, `SELECT id, tenant_id, trip_id, vehicle_id, type, ts, payload FROM trip_events WHERE trip_id = $1 ORDER BY ts, id`, tripID)
	if rows == nil {
		rows = []Event{}
	}
	return rows, err
}

func (s *store) insertEvent(ctx context.Context, tenantID, tripID, vehicleID uuid.UUID, typ string, ts time.Time, payload map[string]any) (*Event, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	b, _ := json.Marshal(payload)
	var e Event
	err := pgxscan.Get(ctx, s.db, &e, `
		INSERT INTO trip_events (tenant_id, trip_id, vehicle_id, type, ts, payload) VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, tenant_id, trip_id, vehicle_id, type, ts, payload`, tenantID, tripID, vehicleID, typ, ts, b)
	return &e, err
}

// lastEventTimes returns the latest timestamp per event type of a trip.
func (s *store) lastEventTimes(ctx context.Context, tripID uuid.UUID) (map[string]time.Time, error) {
	rows, err := s.db.Query(ctx, `SELECT type, max(ts) FROM trip_events WHERE trip_id = $1 GROUP BY type`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var typ string
		var ts time.Time
		if err := rows.Scan(&typ, &ts); err != nil {
			return nil, err
		}
		out[typ] = ts
	}
	return out, rows.Err()
}

func (s *store) hasEvent(ctx context.Context, tripID uuid.UUID, typ string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = $2`, tripID, typ).Scan(&n)
	return n > 0, err
}

func (s *store) points(ctx context.Context, tripID uuid.UUID) ([]TrackPoint, error) {
	var rows []TrackPoint
	err := pgxscan.Select(ctx, s.db, &rows, `SELECT ts, lng, lat, speed, heading, soc FROM trip_points WHERE trip_id = $1 ORDER BY ts`, tripID)
	if rows == nil {
		rows = []TrackPoint{}
	}
	return rows, err
}

func (s *store) insertPoints(ctx context.Context, tripID, vehicleID uuid.UUID, pts []TrackPoint) error {
	if len(pts) == 0 {
		return nil
	}
	rows := make([][]any, len(pts))
	for i, p := range pts {
		rows[i] = []any{p.TS, tripID, vehicleID, p.Lng, p.Lat, p.Speed, p.Heading, p.SOC}
	}
	if _, err := s.db.CopyFrom(ctx, pgx.Identifier{"trip_points"}, []string{"ts", "trip_id", "vehicle_id", "lng", "lat", "speed", "heading", "soc"}, pgx.CopyFromRows(rows)); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE trips SET point_count = point_count + $2 WHERE id = $1`, tripID, len(pts))
	return err
}

// ---- ongoing trip context for the device hooks

type ongoingRow struct {
	ID            uuid.UUID  `db:"id"`
	TenantID      uuid.UUID  `db:"tenant_id"`
	TripNo        string     `db:"trip_no"`
	VehicleID     uuid.UUID  `db:"vehicle_id"`
	DriverID      *uuid.UUID `db:"driver_id"`
	ApprovalID    *uuid.UUID `db:"approval_id"`
	TripType      string     `db:"trip_type"`
	DeviationFlag bool       `db:"deviation_flag"`
	DeviationMaxM *float64   `db:"deviation_max_m"`
	PlannedRoute  []byte     `db:"planned_route"`
	PlateNo       string     `db:"plate_no"`
	L1ApproverID  *uuid.UUID `db:"l1_approver_id"`
}

func (s *store) ongoingByVehicle(ctx context.Context, vehicleID uuid.UUID) (*ongoingRow, error) {
	var r ongoingRow
	err := pgxscan.Get(ctx, s.db, &r, `
		SELECT t.id, t.tenant_id, t.trip_no, t.vehicle_id, t.driver_id, t.approval_id, t.trip_type, t.deviation_flag, t.deviation_max_m,
		       a.planned_route, v.plate_no,
		       (SELECT approver_id FROM approval_steps s WHERE s.approval_id = t.approval_id AND s.step_no = 1) AS l1_approver_id
		FROM trips t
		JOIN vehicles v ON v.id = t.vehicle_id
		LEFT JOIN approvals a ON a.id = t.approval_id
		WHERE t.vehicle_id = $1 AND t.status = 'ongoing'`, vehicleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *ongoingRow) route() [][]float64 {
	if len(r.PlannedRoute) == 0 {
		return nil
	}
	var pr approval.PlannedRoute
	if json.Unmarshal(r.PlannedRoute, &pr) != nil {
		return nil
	}
	return pr.Points
}

// ---- vehicles / cards / users

type vehicleRow struct {
	ID         uuid.UUID  `db:"id"`
	TenantID   uuid.UUID  `db:"tenant_id"`
	PlateNo    string     `db:"plate_no"`
	Status     string     `db:"status"`
	BatteryKwh float64    `db:"battery_kwh"`
	HomeDeptID *uuid.UUID `db:"home_dept_id"`
}

func (s *store) vehicle(ctx context.Context, id uuid.UUID) (*vehicleRow, error) {
	var v vehicleRow
	err := pgxscan.Get(ctx, s.db, &v, `SELECT id, tenant_id, plate_no, status, battery_kwh, home_dept_id FROM vehicles WHERE id = $1 AND deleted_at IS NULL`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

type cardRow struct {
	ID     uuid.UUID  `db:"id"`
	UserID *uuid.UUID `db:"user_id"`
	Status string     `db:"status"`
}

func (s *store) cardByUID(ctx context.Context, tenantID uuid.UUID, uid string) (*cardRow, error) {
	var c cardRow
	err := pgxscan.Get(ctx, s.db, &c, `SELECT id, user_id, status FROM nfc_cards WHERE tenant_id = $1 AND upper(card_uid) = upper($2) AND deleted_at IS NULL`, tenantID, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *store) userActive(ctx context.Context, tenantID, id uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND id = $2 AND status = 'active' AND deleted_at IS NULL`, tenantID, id).Scan(&n)
	return n > 0, err
}

// ---- approvals (read-only here except the status field)

type approvalLite struct {
	ID            uuid.UUID  `db:"id"`
	TenantID      uuid.UUID  `db:"tenant_id"`
	ApplyNo       string     `db:"apply_no"`
	ApplicantID   uuid.UUID  `db:"applicant_id"`
	VehicleID     *uuid.UUID `db:"vehicle_id"`
	TripType      string     `db:"trip_type"`
	PurposeDetail string     `db:"purpose_detail"`
	Status        string     `db:"status"`
	PlannedStart  time.Time  `db:"planned_start"`
	PlannedEnd    time.Time  `db:"planned_end"`
	Passengers    []byte     `db:"passengers"`
}

const approvalLiteSelect = `SELECT id, tenant_id, apply_no, applicant_id, vehicle_id, trip_type, purpose_detail, status, planned_start, planned_end, passengers FROM approvals`

func (s *store) approvalByID(ctx context.Context, tenantID, id uuid.UUID) (*approvalLite, error) {
	var a approvalLite
	err := pgxscan.Get(ctx, s.db, &a, approvalLiteSelect+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *store) approvalByNo(ctx context.Context, tenantID uuid.UUID, applyNo string) (*approvalLite, error) {
	var a approvalLite
	err := pgxscan.Get(ctx, s.db, &a, approvalLiteSelect+` WHERE tenant_id = $1 AND apply_no = $2`, tenantID, applyNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// matchApproval finds the approved request of the vehicle whose window
// (±30 min) contains ts and whose applicant is the driver; nearest start wins.
func (s *store) matchApproval(ctx context.Context, tenantID, vehicleID, driverID uuid.UUID, ts time.Time) (*approvalLite, error) {
	var a approvalLite
	err := pgxscan.Get(ctx, s.db, &a, approvalLiteSelect+`
		WHERE tenant_id = $1 AND vehicle_id = $2 AND applicant_id = $3 AND status = 'approved'
		  AND planned_start - interval '30 minutes' <= $4 AND planned_end + interval '30 minutes' >= $4
		ORDER BY abs(extract(epoch from (planned_start - $4))) LIMIT 1`, tenantID, vehicleID, driverID, ts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (a *approvalLite) hasPassenger(id uuid.UUID) bool {
	if len(a.Passengers) == 0 {
		return false
	}
	var ps []struct {
		ID     *uuid.UUID `json:"id"`
		UserID *uuid.UUID `json:"user_id"`
	}
	if json.Unmarshal(a.Passengers, &ps) != nil {
		return false
	}
	for _, p := range ps {
		if (p.ID != nil && *p.ID == id) || (p.UserID != nil && *p.UserID == id) {
			return true
		}
	}
	return false
}

// ---- writes

type openParams struct {
	tenantID   uuid.UUID
	approvalID uuid.UUID
	vehicleID  uuid.UUID
	driverID   uuid.UUID
	cardID     *uuid.UUID
	tripType   string
	purpose    string
	source     string
	startAt    time.Time
	odometer   *float64
	soc        *float64
	lng, lat   *float64
	remark     *string
}

// open inserts the trip and flips the approval to in_use in one transaction.
// The approval row is locked and re-checked so two gateways/operators cannot
// both start it.
func (s *store) open(ctx context.Context, p openParams) (uuid.UUID, string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM approvals WHERE id = $1 FOR UPDATE`, p.approvalID).Scan(&status); err != nil {
		return uuid.Nil, "", err
	}
	if status != approval.StatusApproved {
		return uuid.Nil, "", errApprovalNotApproved
	}
	tripNo, err := approval.NextNo(ctx, tx, "trips", "trip_no", "T", p.startAt, 3)
	if err != nil {
		return uuid.Nil, "", err
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO trips (tenant_id, trip_no, vehicle_id, driver_id, card_id, approval_id, trip_type, purpose, source, status,
		  start_at, start_odometer, start_soc, start_lng, start_lat, remark)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ongoing',$10,$11,$12,$13,$14,$15) RETURNING id`,
		p.tenantID, tripNo, p.vehicleID, p.driverID, p.cardID, p.approvalID, p.tripType, nilIfEmpty(p.purpose), p.source,
		p.startAt, p.odometer, p.soc, p.lng, p.lat, p.remark).Scan(&id)
	if err != nil {
		return uuid.Nil, "", err
	}
	if err := approval.MarkStatus(ctx, tx, p.approvalID, approval.StatusInUse); err != nil {
		return uuid.Nil, "", err
	}
	return id, tripNo, tx.Commit(ctx)
}

var errApprovalNotApproved = errors.New("approval is not in approved status")

type closeParams struct {
	id             uuid.UUID
	endAt          time.Time
	endOdometer    *float64
	endSOC         *float64
	endLng, endLat *float64
	remark         *string
	stats          Stats
	roofSign       string
}

func (s *store) close(ctx context.Context, p closeParams, approvalID *uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE trips SET status = 'completed', end_at = $2, end_odometer = $3, end_soc = $4, end_lng = $5, end_lat = $6,
		  distance_km = $7, energy_kwh = $8, avg_speed = $9, max_speed = $10, harsh_accel = $11, harsh_brake = $12,
		  deviation_flag = deviation_flag OR $13, deviation_max_m = GREATEST(COALESCE(deviation_max_m, 0), COALESCE($14, 0)),
		  roof_sign_status = $15, remark = COALESCE($16, remark)
		WHERE id = $1 AND status = 'ongoing'`,
		p.id, p.endAt, p.endOdometer, p.endSOC, p.endLng, p.endLat,
		p.stats.DistanceKm, p.stats.EnergyKwh, p.stats.AvgSpeed, p.stats.MaxSpeed, p.stats.HarshAccel, p.stats.HarshBrake,
		p.stats.DeviationFlag, p.stats.DeviationMaxM, p.roofSign, p.remark)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotOngoing
	}
	if approvalID != nil {
		if err := approval.MarkStatus(ctx, tx, *approvalID, approval.StatusCompleted); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

var errNotOngoing = errors.New("trip is not ongoing")

func (s *store) cancel(ctx context.Context, id uuid.UUID, approvalID *uuid.UUID, reason *string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE trips SET status = 'cancelled', end_at = now(), remark = COALESCE($2, remark) WHERE id = $1 AND status = 'ongoing'`, id, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotOngoing
	}
	if approvalID != nil {
		if err := approval.MarkStatus(ctx, tx, *approvalID, approval.StatusApproved); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *store) markDeviation(ctx context.Context, id uuid.UUID, meters float64, flag bool) error {
	_, err := s.db.Exec(ctx, `UPDATE trips SET deviation_flag = deviation_flag OR $3, deviation_max_m = GREATEST(COALESCE(deviation_max_m, 0), $2) WHERE id = $1`, id, meters, flag)
	return err
}

// plannedRouteFull returns the approval's planned route (nil when absent).
func (s *store) plannedRouteFull(ctx context.Context, approvalID *uuid.UUID) (*approval.PlannedRoute, error) {
	if approvalID == nil {
		return nil, nil
	}
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT planned_route FROM approvals WHERE id = $1`, *approvalID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) || len(raw) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pr approval.PlannedRoute
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, nil
	}
	return &pr, nil
}

// plannedRoute returns just the vertices of the planned route.
func (s *store) plannedRoute(ctx context.Context, approvalID *uuid.UUID) ([][]float64, error) {
	pr, err := s.plannedRouteFull(ctx, approvalID)
	if err != nil || pr == nil {
		return nil, err
	}
	return pr.Points, nil
}

func (s *store) setSignOn(ctx context.Context, vehicleID uuid.UUID, on bool) error {
	_, err := s.db.Exec(ctx, `UPDATE vehicle_status SET sign_on = $2, updated_at = now() WHERE vehicle_id = $1`, vehicleID, on)
	return err
}

// ---- aggregates

func (s *store) summary(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (Summary, error) {
	var sm Summary
	err := s.db.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(distance_km), 0), COALESCE(sum(energy_kwh), 0),
		       count(*) FILTER (WHERE status = 'ongoing'), count(*) FILTER (WHERE trip_type = 'official'), count(*) FILTER (WHERE deviation_flag)
		FROM trips WHERE tenant_id = $1 AND status <> 'cancelled' AND start_at >= $2 AND start_at < $3`, tenantID, from, to).
		Scan(&sm.Trips, &sm.DistanceKm, &sm.EnergyKwh, &sm.Ongoing, &sm.OfficialTrips, &sm.DeviationTrips)
	return sm, err
}

func (s *store) vehicleCounts(ctx context.Context, tenantID uuid.UUID) (VehicleCounts, error) {
	var vc VehicleCounts
	err := s.db.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE v.status = 'idle'), count(*) FILTER (WHERE v.status = 'in_use'), count(*) FILTER (WHERE v.status = 'charging'),
		       count(*) FILTER (WHERE v.status = 'maintenance'), count(*) FILTER (WHERE v.status = 'disabled'),
		       count(*) FILTER (WHERE v.status <> 'disabled' AND (vs.last_telemetry_at IS NULL OR vs.last_telemetry_at < now() - interval '5 minutes'))
		FROM vehicles v LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
		WHERE v.tenant_id = $1 AND v.deleted_at IS NULL`, tenantID).
		Scan(&vc.Total, &vc.Idle, &vc.InUse, &vc.Charging, &vc.Maintenance, &vc.Disabled, &vc.Offline)
	return vc, err
}

func (s *store) deviceCounts(ctx context.Context, tenantID uuid.UUID) (DeviceCounts, error) {
	var dc DeviceCounts
	err := s.db.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE last_online_at >= now() - interval '5 minutes')
		FROM devices WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&dc.Total, &dc.Online)
	return dc, err
}

func (s *store) recentEvents(ctx context.Context, tenantID uuid.UUID, limit int) ([]EventWithVehicle, error) {
	var rows []EventWithVehicle
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT e.id, e.tenant_id, e.trip_id, e.vehicle_id, e.type, e.ts, e.payload, v.plate_no, u.name AS driver_name, t.trip_no
		FROM trip_events e
		JOIN trips t ON t.id = e.trip_id
		JOIN vehicles v ON v.id = e.vehicle_id
		LEFT JOIN users u ON u.id = t.driver_id
		WHERE e.tenant_id = $1 AND e.type IN ('deviation','overspeed','low_soc')
		ORDER BY e.ts DESC, e.id DESC LIMIT $2`, tenantID, limit)
	if rows == nil {
		rows = []EventWithVehicle{}
	}
	return rows, err
}

func nilIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
