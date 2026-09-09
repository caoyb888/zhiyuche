package booking

import (
	"context"
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

// row is the flat scan target of the list/detail query.
type row struct {
	ID           uuid.UUID  `db:"id"`
	TenantID     uuid.UUID  `db:"tenant_id"`
	BookingNo    string     `db:"booking_no"`
	Source       string     `db:"source"`
	ContactName  string     `db:"contact_name"`
	ContactPhone *string    `db:"contact_phone"`
	PassengerID  *uuid.UUID `db:"passenger_id"`
	DeptID       *uuid.UUID `db:"dept_id"`
	ReserveStart time.Time  `db:"reserve_start"`
	ReserveEnd   time.Time  `db:"reserve_end"`
	VehicleID    *uuid.UUID `db:"vehicle_id"`
	Origin       string     `db:"origin"`
	OriginLng    *float64   `db:"origin_lng"`
	OriginLat    *float64   `db:"origin_lat"`
	Destination  string     `db:"destination"`
	DestLng      *float64   `db:"dest_lng"`
	DestLat      *float64   `db:"dest_lat"`
	Purpose      *string    `db:"purpose"`
	Remark       *string    `db:"remark"`
	Status       string     `db:"status"`
	CancelReason *string    `db:"cancel_reason"`
	DepartedAt   *time.Time `db:"departed_at"`
	CompletedAt  *time.Time `db:"completed_at"`
	CreatedByID  *uuid.UUID `db:"created_by"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`

	PassengerName     *string `db:"passenger_name"`
	PassengerUsername *string `db:"passenger_username"`
	PassengerPhone    *string `db:"passenger_phone"`
	PassengerDeptName *string `db:"passenger_dept_name"`
	DeptName          *string `db:"dept_name"`

	CreatorName     *string `db:"creator_name"`
	CreatorUsername *string `db:"creator_username"`
	CreatorPhone    *string `db:"creator_phone"`
	CreatorDeptName *string `db:"creator_dept_name"`

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
	SELECT b.id, b.tenant_id, b.booking_no, b.source, b.contact_name, b.contact_phone, b.passenger_id, b.dept_id,
	       b.reserve_start, b.reserve_end, b.vehicle_id, b.origin, b.origin_lng, b.origin_lat,
	       b.destination, b.dest_lng, b.dest_lat, b.purpose, b.remark, b.status, b.cancel_reason,
	       b.departed_at, b.completed_at, b.created_by, b.created_at, b.updated_at,
	       p.name AS passenger_name, p.username AS passenger_username, p.phone AS passenger_phone, pd.name AS passenger_dept_name,
	       d.name AS dept_name,
	       cu.name AS creator_name, cu.username AS creator_username, cu.phone AS creator_phone, cd.name AS creator_dept_name,
	       v.plate_no AS vehicle_plate, v.brand AS vehicle_brand, v.model AS vehicle_model, v.status AS vehicle_status,
	       v.home_dept_id AS vehicle_home_dept_id, vd.name AS vehicle_home_dept_name,
	       vs.soc AS vehicle_soc, vs.range_km AS vehicle_range_km
	FROM bookings b
	LEFT JOIN users p ON p.id = b.passenger_id
	LEFT JOIN departments pd ON pd.id = p.dept_id
	LEFT JOIN departments d ON d.id = b.dept_id
	LEFT JOIN users cu ON cu.id = b.created_by
	LEFT JOIN departments cd ON cd.id = cu.dept_id
	LEFT JOIN vehicles v ON v.id = b.vehicle_id
	LEFT JOIN departments vd ON vd.id = v.home_dept_id
	LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
	WHERE b.tenant_id = $1`

var sortable = map[string]string{
	"created_at": "b.created_at", "reserve_start": "b.reserve_start", "reserve_end": "b.reserve_end",
	"status": "b.status", "booking_no": "b.booking_no", "source": "b.source", "updated_at": "b.updated_at",
}

type listFilter struct {
	ListQuery
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
	if f.Source != "" {
		add("b.source = $%d", f.Source)
	}
	if f.Status != "" {
		add("b.status = $%d", f.Status)
	}
	if f.VehicleID != "" {
		add("b.vehicle_id = $%d", uuid.MustParse(f.VehicleID))
	}
	if f.deptID != nil {
		add("b.dept_id IN (SELECT id FROM departments WHERE deleted_at IS NULL AND path LIKE (SELECT path FROM departments WHERE id = $%d) || '%%')", *f.deptID)
	}
	if f.from != nil {
		add("b.reserve_start >= $%d", *f.from)
	}
	if f.to != nil {
		add("b.reserve_start <= $%d", *f.to)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		add(`(b.booking_no ILIKE $%[1]d OR b.contact_name ILIKE $%[1]d OR b.contact_phone ILIKE $%[1]d
		      OR b.origin ILIKE $%[1]d OR b.destination ILIKE $%[1]d OR b.purpose ILIKE $%[1]d)`, "%"+kw+"%")
	}
	cond := ""
	if len(where) > 0 {
		cond = " AND " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM bookings b WHERE b.tenant_id = $1`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "b.created_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []row
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s, b.id LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*row, error) {
	var r row
	err := pgxscan.Get(ctx, s.db, &r, baseSelect+` AND b.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ---- vehicles / users

type vehicleRow struct {
	approval.VehicleBrief
	Deleted bool `db:"deleted"`
}

func (s *store) vehicle(ctx context.Context, tenantID, id uuid.UUID) (*vehicleRow, error) {
	var v vehicleRow
	err := pgxscan.Get(ctx, s.db, &v, `
		SELECT v.id, v.plate_no, v.brand, v.model, v.status, vs.soc, vs.range_km, v.home_dept_id, d.name AS home_dept_name,
		       (v.deleted_at IS NOT NULL) AS deleted
		FROM vehicles v
		LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
		LEFT JOIN departments d ON d.id = v.home_dept_id
		WHERE v.tenant_id = $1 AND v.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

type passengerRow struct {
	ID     uuid.UUID  `db:"id"`
	DeptID *uuid.UUID `db:"dept_id"`
	Status string     `db:"status"`
}

func (s *store) passenger(ctx context.Context, tenantID, id uuid.UUID) (*passengerRow, error) {
	var u passengerRow
	err := pgxscan.Get(ctx, s.db, &u, `
		SELECT u.id, u.dept_id, u.status FROM users u
		WHERE u.tenant_id = $1 AND u.id = $2 AND u.deleted_at IS NULL`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ---- vehicle window conflicts (bookings and approvals share the fleet)

// The ±15 min buffer mirrors internal/approval so both modules agree on "free".
const bookingWindow = `b.reserve_start < $3::timestamptz + interval '15 minutes' AND b.reserve_end > $2::timestamptz - interval '15 minutes'`

const approvalWindow = `a.planned_start < $3::timestamptz + interval '15 minutes' AND a.planned_end > $2::timestamptz - interval '15 minutes'`

const activeBooking = `b.status IN ('reserved','departed')`

const activeApproval = `a.status IN ('pending_l1','pending_l2','approved','in_use')`

// conflicts lists the other holds on vehicleID overlapping [start, end):
// bookings still due or under way, and approvals not yet finished.
func (s *store) conflicts(ctx context.Context, tenantID, vehicleID uuid.UUID, start, end time.Time, exclude uuid.UUID) ([]Conflict, error) {
	var rows []Conflict
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT 'booking' AS kind, b.booking_no AS no, b.contact_name AS who, b.reserve_start AS start, b.reserve_end AS "end", b.status
		FROM bookings b
		WHERE b.tenant_id = $1 AND b.vehicle_id = $4 AND b.id <> $5 AND `+activeBooking+` AND `+bookingWindow+`
		UNION ALL
		SELECT 'approval' AS kind, a.apply_no AS no, u.name AS who, a.planned_start AS start, a.planned_end AS "end", a.status
		FROM approvals a JOIN users u ON u.id = a.applicant_id
		WHERE a.tenant_id = $1 AND a.vehicle_id = $4 AND `+activeApproval+` AND `+approvalWindow+`
		ORDER BY start`, tenantID, start, end, vehicleID, exclude)
	if rows == nil {
		rows = []Conflict{}
	}
	return rows, err
}

func (s *store) availableVehicles(ctx context.Context, tenantID uuid.UUID, start, end time.Time, exclude uuid.UUID) ([]approval.VehicleBrief, error) {
	var rows []approval.VehicleBrief
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT v.id, v.plate_no, v.brand, v.model, v.status, vs.soc, vs.range_km, v.home_dept_id, d.name AS home_dept_name
		FROM vehicles v
		LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
		LEFT JOIN departments d ON d.id = v.home_dept_id
		WHERE v.tenant_id = $1 AND v.deleted_at IS NULL AND v.status IN ('idle','charging')
		  AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.vehicle_id = v.id AND b.id <> $4 AND `+activeBooking+` AND `+bookingWindow+`)
		  AND NOT EXISTS (SELECT 1 FROM approvals a WHERE a.vehicle_id = v.id AND `+activeApproval+` AND `+approvalWindow+`)
		ORDER BY v.plate_no`, tenantID, start, end, exclude)
	if rows == nil {
		rows = []approval.VehicleBrief{}
	}
	return rows, err
}

// ---- writes

type insertParams struct {
	tenantID  uuid.UUID
	req       CreateRequest
	deptID    *uuid.UUID
	createdBy uuid.UUID
}

func (s *store) insert(ctx context.Context, p insertParams) (uuid.UUID, string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer tx.Rollback(ctx)

	no, err := approval.NextNo(ctx, tx, "bookings", "booking_no", "YY", time.Now(), 4)
	if err != nil {
		return uuid.Nil, "", err
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO bookings (tenant_id, booking_no, source, contact_name, contact_phone, passenger_id, dept_id,
		  reserve_start, reserve_end, vehicle_id, origin, origin_lng, origin_lat, destination, dest_lng, dest_lat,
		  purpose, remark, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,'reserved',$19)
		RETURNING id`,
		p.tenantID, no, p.req.Source, strings.TrimSpace(p.req.ContactName), Trimmed(p.req.ContactPhone), p.req.PassengerID, p.deptID,
		p.req.ReserveStart, p.req.ReserveEnd, p.req.VehicleID, strings.TrimSpace(p.req.Origin), p.req.OriginLng, p.req.OriginLat,
		strings.TrimSpace(p.req.Destination), p.req.DestLng, p.req.DestLat, Trimmed(p.req.Purpose), Trimmed(p.req.Remark),
		p.createdBy).Scan(&id)
	if err != nil {
		return uuid.Nil, "", err
	}
	return id, no, tx.Commit(ctx)
}

// update applies the assignments the service built; a request that changes
// nothing is a no-op.
func (s *store) update(ctx context.Context, tenantID, id uuid.UUID, sets []string, args []any) error {
	if len(sets) == 0 {
		return nil
	}
	all := append([]any{tenantID, id}, args...)
	_, err := s.db.Exec(ctx,
		fmt.Sprintf(`UPDATE bookings SET %s WHERE tenant_id = $1 AND id = $2`, strings.Join(sets, ", ")), all...)
	return err
}

// setStatus moves the booking to next and stamps the matching timestamp.
func (s *store) setStatus(ctx context.Context, tenantID, id uuid.UUID, next string, reason *string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE bookings SET status = $3,
		  departed_at   = CASE WHEN $3 = 'departed'  THEN now() ELSE departed_at   END,
		  completed_at  = CASE WHEN $3 = 'completed' THEN now() ELSE completed_at  END,
		  cancel_reason = CASE WHEN $3 = 'cancelled' THEN $4   ELSE cancel_reason  END
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, next, reason)
	return err
}

// Trimmed normalises an optional text field: blank becomes NULL.
func Trimmed(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
