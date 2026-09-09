package approval

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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

// Querier is what the SQL helpers need: a pool or a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type store struct{ db *pgxpool.Pool }

// row is the flat scan target of the approval list/detail query.
type row struct {
	ID            uuid.UUID  `db:"id"`
	TenantID      uuid.UUID  `db:"tenant_id"`
	ApplyNo       string     `db:"apply_no"`
	ApplicantID   uuid.UUID  `db:"applicant_id"`
	DeptID        *uuid.UUID `db:"dept_id"`
	TripType      string     `db:"trip_type"`
	PurposeCode   string     `db:"purpose_code"`
	PurposeDetail string     `db:"purpose_detail"`
	PlannedStart  time.Time  `db:"planned_start"`
	PlannedEnd    time.Time  `db:"planned_end"`
	Destination   string     `db:"destination"`
	DestLng       *float64   `db:"dest_lng"`
	DestLat       *float64   `db:"dest_lat"`
	PlannedRoute  []byte     `db:"planned_route"`
	PlannedKm     *float64   `db:"planned_km"`
	Passengers    []byte     `db:"passengers"`
	Attachments   []byte     `db:"attachments"`
	VehicleID     *uuid.UUID `db:"vehicle_id"`
	Urgency       string     `db:"urgency"`
	Status        string     `db:"status"`
	LevelRequired int        `db:"level_required"`
	CurrentStep   int        `db:"current_step"`
	RejectReason  *string    `db:"reject_reason"`
	CancelReason  *string    `db:"cancel_reason"`
	ApprovedAt    *time.Time `db:"approved_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`

	ApplicantName     string  `db:"applicant_name"`
	ApplicantUsername string  `db:"applicant_username"`
	ApplicantPhone    *string `db:"applicant_phone"`
	ApplicantDeptName *string `db:"applicant_dept_name"`
	DeptName          *string `db:"dept_name"`
	PurposeLabel      *string `db:"purpose_label"`

	VehiclePlate        *string    `db:"vehicle_plate"`
	VehicleBrand        *string    `db:"vehicle_brand"`
	VehicleModel        *string    `db:"vehicle_model"`
	VehicleStatus       *string    `db:"vehicle_status"`
	VehicleHomeDeptID   *uuid.UUID `db:"vehicle_home_dept_id"`
	VehicleHomeDeptName *string    `db:"vehicle_home_dept_name"`
	VehicleSOC          *float64   `db:"vehicle_soc"`
	VehicleRangeKm      *float64   `db:"vehicle_range_km"`
}

const baseSelect = `
	SELECT a.id, a.tenant_id, a.apply_no, a.applicant_id, a.dept_id, a.trip_type, a.purpose_code, a.purpose_detail,
	       a.planned_start, a.planned_end, a.destination, a.dest_lng, a.dest_lat, a.planned_route, a.planned_km,
	       a.passengers, a.attachments, a.vehicle_id, a.urgency, a.status, a.level_required, a.current_step,
	       a.reject_reason, a.cancel_reason, a.approved_at, a.created_at, a.updated_at,
	       u.name AS applicant_name, u.username AS applicant_username, u.phone AS applicant_phone, ud.name AS applicant_dept_name,
	       d.name AS dept_name,
	       (SELECT di.label FROM dict_items di JOIN dict_types dt ON dt.id = di.dict_type_id
	         WHERE dt.code = 'approval_purpose' AND di.value = a.purpose_code AND (dt.tenant_id = a.tenant_id OR dt.tenant_id IS NULL)
	         ORDER BY dt.tenant_id NULLS LAST LIMIT 1) AS purpose_label,
	       v.plate_no AS vehicle_plate, v.brand AS vehicle_brand, v.model AS vehicle_model, v.status AS vehicle_status,
	       v.home_dept_id AS vehicle_home_dept_id, vd.name AS vehicle_home_dept_name,
	       vs.soc AS vehicle_soc, vs.range_km AS vehicle_range_km
	FROM approvals a
	JOIN users u ON u.id = a.applicant_id
	LEFT JOIN departments ud ON ud.id = u.dept_id
	LEFT JOIN departments d ON d.id = a.dept_id
	LEFT JOIN vehicles v ON v.id = a.vehicle_id
	LEFT JOIN departments vd ON vd.id = v.home_dept_id
	LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
	WHERE a.tenant_id = $1`

var sortable = map[string]string{
	"created_at": "a.created_at", "planned_start": "a.planned_start", "planned_end": "a.planned_end",
	"status": "a.status", "apply_no": "a.apply_no", "updated_at": "a.updated_at",
}

// expireStale lazily marks pending/approved requests whose window ended more
// than 2 hours ago as expired (and skips their pending steps). Returns the ids.
func (s *store) expireStale(ctx context.Context, tenantID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := pgxscan.Select(ctx, s.db, &ids, `
		WITH e AS (
		  UPDATE approvals SET status = 'expired'
		  WHERE tenant_id = $1 AND status IN ('pending_l1','pending_l2','approved') AND planned_end + interval '2 hours' < now()
		  RETURNING id
		), st AS (
		  UPDATE approval_steps SET action = 'skipped' WHERE approval_id IN (SELECT id FROM e) AND action = 'pending'
		)
		SELECT id FROM e`, tenantID)
	return ids, err
}

type listFilter struct {
	ListQuery
	me     uuid.UUID
	from   *time.Time
	to     *time.Time
	deptID *uuid.UUID
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, f listFilter, pg pagination.Query) ([]row, int64, error) {
	where := []string{}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	switch f.Scope {
	case "todo":
		add(`a.status IN ('pending_l1','pending_l2') AND EXISTS (SELECT 1 FROM approval_steps s WHERE s.approval_id = a.id AND s.step_no = a.current_step AND s.action = 'pending' AND s.approver_id = $%d)`, f.me)
	case "all":
	default:
		add("a.applicant_id = $%d", f.me)
	}
	if f.Status != "" {
		add("a.status = $%d", f.Status)
	}
	if f.TripType != "" {
		add("a.trip_type = $%d", f.TripType)
	}
	if f.deptID != nil {
		add("a.dept_id IN (SELECT id FROM departments WHERE deleted_at IS NULL AND path LIKE (SELECT path FROM departments WHERE id = $%d) || '%%')", *f.deptID)
	}
	if f.VehicleID != "" {
		add("a.vehicle_id = $%d", uuid.MustParse(f.VehicleID))
	}
	if f.ApplicantID != "" {
		add("a.applicant_id = $%d", uuid.MustParse(f.ApplicantID))
	}
	if f.from != nil {
		add("a.planned_start >= $%d", *f.from)
	}
	if f.to != nil {
		add("a.planned_start <= $%d", *f.to)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		add("(a.apply_no ILIKE $%[1]d OR a.purpose_detail ILIKE $%[1]d OR a.destination ILIKE $%[1]d OR u.name ILIKE $%[1]d)", "%"+kw+"%")
	}
	cond := ""
	if len(where) > 0 {
		cond = " AND " + strings.Join(where, " AND ")
	}
	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM approvals a JOIN users u ON u.id = a.applicant_id WHERE a.tenant_id = $1`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "a.created_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []row
	err := pgxscan.Select(ctx, s.db, &rows, fmt.Sprintf("%s%s ORDER BY %s, a.id LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*row, error) {
	var r row
	err := pgxscan.Get(ctx, s.db, &r, baseSelect+` AND a.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// stepRow is one approval_steps row joined with its approver.
type stepRow struct {
	ApprovalID uuid.UUID  `db:"approval_id"`
	StepNo     int        `db:"step_no"`
	ApproverID uuid.UUID  `db:"approver_id"`
	Action     string     `db:"action"`
	Remark     *string    `db:"remark"`
	ActedAt    *time.Time `db:"acted_at"`
	Name       string     `db:"name"`
	Username   string     `db:"username"`
	Phone      *string    `db:"phone"`
	DeptName   *string    `db:"dept_name"`
}

func (s *store) steps(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]stepRow, error) {
	out := map[uuid.UUID][]stepRow{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []stepRow
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT s.approval_id, s.step_no, s.approver_id, s.action, s.remark, s.acted_at,
		       u.name, u.username, u.phone, d.name AS dept_name
		FROM approval_steps s
		JOIN users u ON u.id = s.approver_id
		LEFT JOIN departments d ON d.id = u.dept_id
		WHERE s.approval_id = ANY($1) ORDER BY s.approval_id, s.step_no`, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ApprovalID] = append(out[r.ApprovalID], r)
	}
	return out, nil
}

type tripBriefRow struct {
	ApprovalID uuid.UUID `db:"approval_id"`
	TripBrief
}

// trips returns the latest trip of each approval.
func (s *store) trips(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*TripBrief, error) {
	out := map[uuid.UUID]*TripBrief{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []tripBriefRow
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT DISTINCT ON (approval_id) approval_id, id, trip_no, status, start_at, end_at, distance_km
		FROM trips WHERE approval_id = ANY($1)
		ORDER BY approval_id, (status = 'ongoing') DESC, start_at DESC`, ids)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		b := rows[i].TripBrief
		out[rows[i].ApprovalID] = &b
	}
	return out, nil
}

// ---- users / depts

type userRow struct {
	UserBrief
	DeptID *uuid.UUID `db:"dept_id"`
	Status string     `db:"status"`
}

const userSelect = `
	SELECT u.id, u.name, u.username, d.name AS dept_name, u.phone, u.dept_id, u.status
	FROM users u LEFT JOIN departments d ON d.id = u.dept_id
	WHERE u.deleted_at IS NULL`

func (s *store) user(ctx context.Context, tenantID, id uuid.UUID) (*userRow, error) {
	var u userRow
	err := pgxscan.Get(ctx, s.db, &u, userSelect+` AND u.tenant_id = $1 AND u.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *store) users(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]userRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []userRow
	err := pgxscan.Select(ctx, s.db, &rows, userSelect+` AND u.tenant_id = $1 AND u.id = ANY($2) ORDER BY u.name`, tenantID, ids)
	return rows, err
}

// UserBriefByID loads a user brief (nil if missing). Exported for the trip module.
func UserBriefByID(ctx context.Context, q Querier, id uuid.UUID) (*UserBrief, error) {
	var u UserBrief
	err := pgxscan.Get(ctx, q, &u, `
		SELECT u.id, u.name, u.username, d.name AS dept_name, u.phone
		FROM users u LEFT JOIN departments d ON d.id = u.dept_id WHERE u.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

type deptRow struct {
	ID           uuid.UUID  `db:"id"`
	ParentID     *uuid.UUID `db:"parent_id"`
	LeaderUserID *uuid.UUID `db:"leader_user_id"`
}

func (s *store) dept(ctx context.Context, tenantID, id uuid.UUID) (*deptRow, error) {
	var d deptRow
	err := pgxscan.Get(ctx, s.db, &d, `SELECT id, parent_id, leader_user_id FROM departments WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *store) userActive(ctx context.Context, tenantID, id uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND id = $2 AND status = 'active' AND deleted_at IS NULL`, tenantID, id).Scan(&n)
	return n > 0, err
}

// firstUserWithRole returns the earliest-created active user of the tenant
// holding role code, excluding the given ids (uuid.Nil when none found).
func (s *store) firstUserWithRole(ctx context.Context, tenantID uuid.UUID, roleCode string, exclude []uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT u.id FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
		WHERE u.tenant_id = $1 AND r.code = $2 AND u.status = 'active' AND u.deleted_at IS NULL AND NOT (u.id = ANY($3))
		ORDER BY u.created_at, u.id LIMIT 1`, tenantID, roleCode, exclude).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return id, err
}

// ---- vehicles

type vehicleRow struct {
	VehicleBrief
	Deleted bool `db:"deleted"`
}

const vehicleSelect = `
	SELECT v.id, v.plate_no, v.brand, v.model, v.status, vs.soc, vs.range_km, v.home_dept_id, d.name AS home_dept_name,
	       (v.deleted_at IS NOT NULL) AS deleted
	FROM vehicles v
	LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
	LEFT JOIN departments d ON d.id = v.home_dept_id
	WHERE v.tenant_id = $1`

func (s *store) vehicle(ctx context.Context, tenantID, id uuid.UUID) (*vehicleRow, error) {
	var v vehicleRow
	err := pgxscan.Get(ctx, s.db, &v, vehicleSelect+` AND v.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

const conflictWindow = `a.planned_start < $3::timestamptz + interval '15 minutes' AND a.planned_end > $2::timestamptz - interval '15 minutes'`

func (s *store) conflicts(ctx context.Context, q Querier, tenantID, vehicleID uuid.UUID, start, end time.Time, exclude uuid.UUID) ([]Conflict, error) {
	var rows []Conflict
	err := pgxscan.Select(ctx, q, &rows, `
		SELECT a.apply_no, u.name AS applicant_name, a.planned_start, a.planned_end, a.status
		FROM approvals a JOIN users u ON u.id = a.applicant_id
		WHERE a.tenant_id = $1 AND a.vehicle_id = $4 AND a.id <> $5
		  AND a.status IN ('pending_l1','pending_l2','approved','in_use') AND `+conflictWindow+`
		ORDER BY a.planned_start`, tenantID, start, end, vehicleID, exclude)
	if rows == nil {
		rows = []Conflict{}
	}
	return rows, err
}

func (s *store) availableVehicles(ctx context.Context, tenantID uuid.UUID, start, end time.Time, exclude uuid.UUID) ([]VehicleBrief, error) {
	var rows []VehicleBrief
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT v.id, v.plate_no, v.brand, v.model, v.status, vs.soc, vs.range_km, v.home_dept_id, d.name AS home_dept_name
		FROM vehicles v
		LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
		LEFT JOIN departments d ON d.id = v.home_dept_id
		WHERE v.tenant_id = $1 AND v.deleted_at IS NULL AND v.status IN ('idle','charging')
		  AND NOT EXISTS (
		    SELECT 1 FROM approvals a WHERE a.vehicle_id = v.id AND a.id <> $4
		      AND a.status IN ('pending_l1','pending_l2','approved','in_use') AND `+conflictWindow+`)
		ORDER BY v.plate_no`, tenantID, start, end, exclude)
	if rows == nil {
		rows = []VehicleBrief{}
	}
	return rows, err
}

// ---- writes

type insertParams struct {
	tenantID   uuid.UUID
	applicant  userRow
	req        CreateRequest
	passengers []UserBrief
	level      int
	approvers  []uuid.UUID
	createdBy  uuid.UUID
}

func (s *store) insert(ctx context.Context, p insertParams) (uuid.UUID, string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer tx.Rollback(ctx)
	applyNo, err := NextNo(ctx, tx, "approvals", "apply_no", "ZY", time.Now(), 4)
	if err != nil {
		return uuid.Nil, "", err
	}
	route, err := jsonOrNull(p.req.PlannedRoute)
	if err != nil {
		return uuid.Nil, "", err
	}
	pass, _ := json.Marshal(p.passengers)
	att := p.req.Attachments
	if att == nil {
		att = []Attachment{}
	}
	attJSON, _ := json.Marshal(att)
	urgency := p.req.Urgency
	if urgency == "" {
		urgency = "normal"
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO approvals (tenant_id, apply_no, applicant_id, dept_id, trip_type, purpose_code, purpose_detail,
		  planned_start, planned_end, destination, dest_lng, dest_lat, planned_route, planned_km, passengers, attachments,
		  vehicle_id, urgency, status, level_required, current_step, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,'pending_l1',$19,1,$20)
		RETURNING id`,
		p.tenantID, applyNo, p.applicant.ID, p.applicant.DeptID, p.req.TripType, p.req.PurposeCode, p.req.PurposeDetail,
		p.req.PlannedStart, p.req.PlannedEnd, p.req.Destination, p.req.DestLng, p.req.DestLat, route, p.req.PlannedKm, pass, attJSON,
		p.req.VehicleID, urgency, p.level, p.createdBy).Scan(&id)
	if err != nil {
		return uuid.Nil, "", err
	}
	for i, a := range p.approvers {
		if _, err := tx.Exec(ctx, `INSERT INTO approval_steps (approval_id, step_no, approver_id) VALUES ($1,$2,$3)`, id, i+1, a); err != nil {
			return uuid.Nil, "", err
		}
	}
	return id, applyNo, tx.Commit(ctx)
}

func jsonOrNull(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	if r, ok := v.(*PlannedRoute); ok && r == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// lockRow is the subset of an approval read FOR UPDATE inside a workflow transaction.
type lockRow struct {
	ID            uuid.UUID  `db:"id"`
	TenantID      uuid.UUID  `db:"tenant_id"`
	ApplyNo       string     `db:"apply_no"`
	ApplicantID   uuid.UUID  `db:"applicant_id"`
	VehicleID     *uuid.UUID `db:"vehicle_id"`
	Status        string     `db:"status"`
	LevelRequired int        `db:"level_required"`
	CurrentStep   int        `db:"current_step"`
	PlannedStart  time.Time  `db:"planned_start"`
	PlannedEnd    time.Time  `db:"planned_end"`
}

func lockApproval(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (*lockRow, error) {
	var r lockRow
	err := pgxscan.Get(ctx, tx, &r, `
		SELECT id, tenant_id, apply_no, applicant_id, vehicle_id, status, level_required, current_step, planned_start, planned_end
		FROM approvals WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

type pendingStep struct {
	StepNo     int       `db:"step_no"`
	ApproverID uuid.UUID `db:"approver_id"`
}

func currentStep(ctx context.Context, tx pgx.Tx, id uuid.UUID, stepNo int) (*pendingStep, error) {
	var st pendingStep
	err := pgxscan.Get(ctx, tx, &st, `SELECT step_no, approver_id FROM approval_steps WHERE approval_id = $1 AND step_no = $2`, id, stepNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *store) stepApproverIDs(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := pgxscan.Select(ctx, s.db, &ids, `SELECT approver_id FROM approval_steps WHERE approval_id = $1 ORDER BY step_no`, id)
	return ids, err
}

func (s *store) hasOngoingTrip(ctx context.Context, q Querier, approvalID uuid.UUID) (bool, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM trips WHERE approval_id = $1 AND status = 'ongoing'`, approvalID).Scan(&n)
	return n > 0, err
}

// MarkStatus sets an approval status (used by the trip module on start/end/cancel).
func MarkStatus(ctx context.Context, q Querier, id uuid.UUID, status string) error {
	_, err := q.Exec(ctx, `UPDATE approvals SET status = $2 WHERE id = $1`, id, status)
	return err
}

// ---- rules

type rulesRow struct {
	Enabled              bool       `db:"enabled"`
	Level2Km             *float64   `db:"level2_km"`
	Level2Night          bool       `db:"level2_night"`
	NightStart           string     `db:"night_start"`
	NightEnd             string     `db:"night_end"`
	Level2CrossDept      bool       `db:"level2_cross_dept"`
	Level2TripTypes      []string   `db:"level2_trip_types"`
	Level2ApproverID     *uuid.UUID `db:"level2_approver_id"`
	FallbackApproverRole string     `db:"fallback_approver_role"`
	OverdueAlertMinutes  int        `db:"overdue_alert_minutes"`
	UpdatedAt            time.Time  `db:"updated_at"`
}

const rulesSelect = `
	SELECT enabled, level2_km, level2_night, to_char(night_start, 'HH24:MI') AS night_start, to_char(night_end, 'HH24:MI') AS night_end,
	       level2_cross_dept, level2_trip_types, level2_approver_id, fallback_approver_role, overdue_alert_minutes, updated_at
	FROM approval_rules WHERE tenant_id = $1`

func (s *store) rules(ctx context.Context, tenantID uuid.UUID) (*rulesRow, error) {
	var r rulesRow
	err := pgxscan.Get(ctx, s.db, &r, rulesSelect, tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *store) upsertRules(ctx context.Context, tenantID uuid.UUID, r Rules, updatedBy uuid.UUID) error {
	types := r.Level2TripTypes
	if types == nil {
		types = []string{}
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO approval_rules (tenant_id, enabled, level2_km, level2_night, night_start, night_end, level2_cross_dept,
		  level2_trip_types, level2_approver_id, fallback_approver_role, overdue_alert_minutes, updated_by)
		VALUES ($1,$2,$3,$4,$5::time,$6::time,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (tenant_id) DO UPDATE SET
		  enabled = EXCLUDED.enabled, level2_km = EXCLUDED.level2_km, level2_night = EXCLUDED.level2_night,
		  night_start = EXCLUDED.night_start, night_end = EXCLUDED.night_end, level2_cross_dept = EXCLUDED.level2_cross_dept,
		  level2_trip_types = EXCLUDED.level2_trip_types, level2_approver_id = EXCLUDED.level2_approver_id,
		  fallback_approver_role = EXCLUDED.fallback_approver_role, overdue_alert_minutes = EXCLUDED.overdue_alert_minutes,
		  updated_by = EXCLUDED.updated_by, updated_at = now()`,
		tenantID, r.Enabled, r.Level2Km, r.Level2Night, r.NightStart, r.NightEnd, r.Level2CrossDept,
		types, r.Level2ApproverID, r.FallbackApproverRole, r.OverdueAlertMinutes, updatedBy)
	return err
}

func (s *store) todoCount(ctx context.Context, tenantID, me uuid.UUID) (int64, error) {
	var n int64
	err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM approvals a
		WHERE a.tenant_id = $1 AND a.status IN ('pending_l1','pending_l2')
		  AND EXISTS (SELECT 1 FROM approval_steps s WHERE s.approval_id = a.id AND s.step_no = a.current_step AND s.action = 'pending' AND s.approver_id = $2)`,
		tenantID, me).Scan(&n)
	return n, err
}

// TodoCount is the number of steps waiting on user (exported for the dashboard).
func TodoCount(ctx context.Context, db *pgxpool.Pool, tenantID, me uuid.UUID) (int64, error) {
	return (&store{db: db}).todoCount(ctx, tenantID, me)
}

// PendingCount is the number of pending requests in the tenant (exported for the dashboard).
func PendingCount(ctx context.Context, db *pgxpool.Pool, tenantID uuid.UUID) (int64, error) {
	var n int64
	err := db.QueryRow(ctx, `SELECT count(*) FROM approvals WHERE tenant_id = $1 AND status IN ('pending_l1','pending_l2')`, tenantID).Scan(&n)
	return n, err
}
