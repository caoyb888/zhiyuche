package device

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type store struct{ db *pgxpool.Pool }

const baseFrom = `
	FROM devices d
	LEFT JOIN vehicles v ON v.id = d.vehicle_id AND v.deleted_at IS NULL
	WHERE d.deleted_at IS NULL`

const baseSelect = `
	SELECT d.id, d.tenant_id, d.serial_no, d.vehicle_id, v.plate_no AS vehicle_plate, d.model, d.firmware, d.iccid,
	       d.status, d.last_online_at, d.last_ip, d.remark, d.created_at, d.updated_at` + baseFrom

var sortable = map[string]string{
	"serial_no": "d.serial_no", "status": "d.status", "last_online_at": "d.last_online_at", "created_at": "d.created_at",
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) ([]Device, int64, error) {
	where := []string{"d.tenant_id = $1"}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(d.serial_no ILIKE $%[1]d OR d.iccid ILIKE $%[1]d OR v.plate_no ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Status != "" {
		add("d.status = $%d", q.Status)
	}
	if q.Bound != nil {
		if *q.Bound {
			where = append(where, "d.vehicle_id IS NOT NULL")
		} else {
			where = append(where, "d.vehicle_id IS NULL")
		}
	}
	if q.Online != nil {
		secs := vstatus.OfflineAfter.Seconds()
		if *q.Online {
			add("d.last_online_at > now() - $%d * interval '1 second'", secs)
		} else {
			add("(d.last_online_at IS NULL OR d.last_online_at <= now() - $%d * interval '1 second')", secs)
		}
	}
	cond := " AND " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*)`+baseFrom+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "d.created_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Device
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*Device, error) {
	var d Device
	err := pgxscan.Get(ctx, s.db, &d, baseSelect+` AND d.tenant_id = $1 AND d.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// serialExists is global: a gateway serial identifies one physical unit.
func (s *store) serialExists(ctx context.Context, serial string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM devices WHERE serial_no = $1 AND deleted_at IS NULL`, serial).Scan(&n)
	return n > 0, err
}

// vehicleInfo is what binding rules need to know about a vehicle.
type vehicleInfo struct {
	ID       uuid.UUID  `db:"id"`
	Status   string     `db:"status"`
	TripID   *uuid.UUID `db:"current_trip_id"`
	DeviceID *uuid.UUID `db:"device_id"`
}

func (s *store) vehicle(ctx context.Context, tenantID, id uuid.UUID) (*vehicleInfo, error) {
	var v vehicleInfo
	err := pgxscan.Get(ctx, s.db, &v, `
		SELECT v.id, v.status, vs.current_trip_id, dev.id AS device_id
		FROM vehicles v
		LEFT JOIN vehicle_status vs ON vs.vehicle_id = v.id
		LEFT JOIN devices dev ON dev.vehicle_id = v.id AND dev.deleted_at IS NULL
		WHERE v.tenant_id = $1 AND v.id = $2 AND v.deleted_at IS NULL`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *store) create(ctx context.Context, tenantID uuid.UUID, serial, keyHash string, req CreateRequest) (uuid.UUID, error) {
	model := "VIG-100E"
	if m := nilIfEmpty(req.Model); m != nil {
		model = *m
	}
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO devices (tenant_id, serial_no, vehicle_id, api_key_hash, model, firmware, iccid, remark)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		tenantID, serial, req.VehicleID, keyHash, model, nilIfEmpty(req.Firmware), nilIfEmpty(req.ICCID), nilIfEmpty(req.Remark)).Scan(&id)
	return id, err
}

func (s *store) update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) error {
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if m := nilIfEmpty(req.Model); m != nil {
		set("model", *m)
	}
	if req.Firmware != nil {
		set("firmware", nilIfEmpty(req.Firmware))
	}
	if req.ICCID != nil {
		set("iccid", nilIfEmpty(req.ICCID))
	}
	if req.Status != nil {
		set("status", *req.Status)
	}
	if req.Remark != nil {
		set("remark", nilIfEmpty(req.Remark))
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE devices SET %s WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) setVehicle(ctx context.Context, tenantID, id uuid.UUID, vehicleID *uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE devices SET vehicle_id = $3 WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id, vehicleID)
	return err
}

func (s *store) setKeyHash(ctx context.Context, tenantID, id uuid.UUID, hash string) error {
	_, err := s.db.Exec(ctx, `UPDATE devices SET api_key_hash = $3 WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id, hash)
	return err
}

// softDelete marks the device deleted and releases the vehicle binding so the
// partial unique indexes free the serial and the vehicle.
func (s *store) softDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE devices SET deleted_at = now(), vehicle_id = NULL, status = 'disabled' WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	return err
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}
