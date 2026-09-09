package vehicle

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

	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type store struct{ db *pgxpool.Pool }

const baseFrom = `
	FROM vehicles v
	LEFT JOIN departments d ON d.id = v.home_dept_id
	LEFT JOIN devices dev ON dev.vehicle_id = v.id AND dev.deleted_at IS NULL
	LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
	WHERE v.deleted_at IS NULL`

const baseSelect = `
	SELECT v.id, v.tenant_id, v.plate_no, v.vin, v.brand, v.model, v.color, v.seat_count, v.battery_kwh, v.range_km_full,
	       v.status, v.odometer_km, v.soc, v.soh, v.purchase_date, v.insurance_expire, v.inspection_expire,
	       v.home_dept_id, d.name AS home_dept_name, dev.id AS device_id, dev.serial_no AS device_serial,
	       v.remark, v.created_at, v.updated_at` + baseFrom

var sortable = map[string]string{
	"plate_no": "v.plate_no", "status": "v.status", "odometer_km": "v.odometer_km", "created_at": "v.created_at",
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) ([]Vehicle, int64, error) {
	where := []string{"v.tenant_id = $1"}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(v.plate_no ILIKE $%[1]d OR v.vin ILIKE $%[1]d OR v.brand ILIKE $%[1]d OR v.model ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Status != "" {
		add("v.status = $%d", q.Status)
	}
	if q.DeptID != "" {
		if id, err := uuid.Parse(q.DeptID); err == nil {
			// 含子部门：通过 path 前缀匹配（同 user 模块）
			add("v.home_dept_id IN (SELECT id FROM departments WHERE deleted_at IS NULL AND path LIKE (SELECT path FROM departments WHERE id = $%d) || '%%')", id)
		}
	}
	if q.Online != nil {
		secs := vstatus.OfflineAfter.Seconds()
		if *q.Online {
			add("vs.last_telemetry_at > now() - $%d * interval '1 second'", secs)
		} else {
			add("(vs.last_telemetry_at IS NULL OR vs.last_telemetry_at <= now() - $%d * interval '1 second')", secs)
		}
	}
	cond := " AND " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*)`+baseFrom+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "v.created_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Vehicle
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*Vehicle, error) {
	var v Vehicle
	err := pgxscan.Get(ctx, s.db, &v, baseSelect+` AND v.tenant_id = $1 AND v.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *store) options(ctx context.Context, tenantID uuid.UUID, status string) ([]Brief, error) {
	q := `
		SELECT v.id, v.plate_no, v.brand, v.model, v.status, vs.soc, vs.range_km, v.home_dept_id, d.name AS home_dept_name
		FROM vehicles v
		LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
		LEFT JOIN departments d ON d.id = v.home_dept_id
		WHERE v.deleted_at IS NULL AND v.tenant_id = $1`
	args := []any{tenantID}
	if status != "" {
		q += " AND v.status = $2"
		args = append(args, status)
	}
	var rows []Brief
	if err := pgxscan.Select(ctx, s.db, &rows, q+" ORDER BY v.plate_no", args...); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Brief{}
	}
	return rows, nil
}

func (s *store) plateExists(ctx context.Context, tenantID uuid.UUID, plate string, exclude uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE tenant_id = $1 AND plate_no = $2 AND id <> $3 AND deleted_at IS NULL`, tenantID, plate, exclude).Scan(&n)
	return n > 0, err
}

// vinExists is global: a VIN identifies one physical vehicle across tenants.
func (s *store) vinExists(ctx context.Context, vin string, exclude uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE vin = $1 AND id <> $2 AND deleted_at IS NULL`, vin, exclude).Scan(&n)
	return n > 0, err
}

func (s *store) deptExists(ctx context.Context, tenantID, deptID uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM departments WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, deptID).Scan(&n)
	return n > 0, err
}

func (s *store) create(ctx context.Context, tenantID uuid.UUID, req CreateRequest, createdBy uuid.UUID) (uuid.UUID, error) {
	seat, battery, rangeKm, odo := 5, 60.0, 400, 0.0
	if req.SeatCount != nil {
		seat = *req.SeatCount
	}
	if req.BatteryKwh != nil {
		battery = *req.BatteryKwh
	}
	if req.RangeKmFull != nil {
		rangeKm = *req.RangeKmFull
	}
	if req.OdometerKm != nil {
		odo = *req.OdometerKm
	}
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO vehicles (tenant_id, plate_no, vin, brand, model, color, seat_count, battery_kwh, range_km_full, status,
		                      odometer_km, purchase_date, insurance_expire, inspection_expire, home_dept_id, remark, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'idle',$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
		tenantID, strings.TrimSpace(req.PlateNo), nilIfEmpty(req.VIN), nilIfEmpty(req.Brand), nilIfEmpty(req.Model), nilIfEmpty(req.Color),
		seat, battery, rangeKm, odo, dateArg(req.PurchaseDate), dateArg(req.InsuranceExpire), dateArg(req.InspectionExpire),
		req.HomeDeptID, nilIfEmpty(req.Remark), createdBy).Scan(&id)
	return id, err
}

// update writes every present archive field; status is handled by the service through vstatus.
func (s *store) update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) error {
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.PlateNo != nil {
		set("plate_no", strings.TrimSpace(*req.PlateNo))
	}
	if req.VIN != nil {
		set("vin", nilIfEmpty(req.VIN))
	}
	if req.Brand != nil {
		set("brand", nilIfEmpty(req.Brand))
	}
	if req.Model != nil {
		set("model", nilIfEmpty(req.Model))
	}
	if req.Color != nil {
		set("color", nilIfEmpty(req.Color))
	}
	if req.SeatCount != nil {
		set("seat_count", *req.SeatCount)
	}
	if req.BatteryKwh != nil {
		set("battery_kwh", *req.BatteryKwh)
	}
	if req.RangeKmFull != nil {
		set("range_km_full", *req.RangeKmFull)
	}
	if req.OdometerKm != nil {
		set("odometer_km", *req.OdometerKm)
	}
	if req.PurchaseDate != nil {
		set("purchase_date", dateArg(req.PurchaseDate))
	}
	if req.InsuranceExpire != nil {
		set("insurance_expire", dateArg(req.InsuranceExpire))
	}
	if req.InspectionExpire != nil {
		set("inspection_expire", dateArg(req.InspectionExpire))
	}
	if req.ClearHomeDept {
		set("home_dept_id", nil)
	} else if req.HomeDeptID != nil {
		set("home_dept_id", *req.HomeDeptID)
	}
	if req.Remark != nil {
		set("remark", nilIfEmpty(req.Remark))
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE vehicles SET %s WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...)
	return err
}

// activeApprovals counts unfinished approvals that reference the vehicle.
func (s *store) activeApprovals(ctx context.Context, id uuid.UUID) (int64, error) {
	var n int64
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM approvals WHERE vehicle_id = $1 AND status IN ('pending_l1','pending_l2','approved','in_use')`, id).Scan(&n)
	return n, err
}

func (s *store) tripCount(ctx context.Context, id uuid.UUID) (int64, error) {
	var n int64
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM trips WHERE vehicle_id = $1`, id).Scan(&n)
	return n, err
}

// softDelete marks the vehicle deleted and releases its gateway.
func (s *store) softDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE vehicles SET deleted_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE devices SET vehicle_id = NULL WHERE vehicle_id = $1 AND deleted_at IS NULL`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *store) telemetry(ctx context.Context, id uuid.UUID, from, to time.Time, limit int) ([]TelemetryPoint, error) {
	var rows []TelemetryPoint
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT ts, lng, lat, speed, heading, soc, soh, cell_temp, motor_temp, odometer_km, acc_on, locked, sign_on, charging
		FROM vehicle_telemetry WHERE vehicle_id = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts ASC LIMIT $4`, id, from, to, limit)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []TelemetryPoint{}
	}
	return rows, nil
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}
