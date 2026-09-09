package pile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type store struct{ db *pgxpool.Pool }

const baseSelect = `
	SELECT p.id, p.tenant_id, p.pile_code, p.name, p.type, p.power_kw, p.connector_count, p.vendor, p.location,
	       p.lng, p.lat, p.status, p.last_heartbeat_at, p.remark, p.created_at, p.updated_at
	FROM charge_piles p
	WHERE p.deleted_at IS NULL`

var sortable = map[string]string{
	"pile_code": "p.pile_code", "name": "p.name", "type": "p.type", "power_kw": "p.power_kw",
	"status": "p.status", "created_at": "p.created_at",
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) ([]Pile, int64, error) {
	where := []string{"p.tenant_id = $1"}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(p.pile_code ILIKE $%[1]d OR p.name ILIKE $%[1]d OR p.location ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Type != "" {
		add("p.type = $%d", q.Type)
	}
	if q.Status != "" {
		add("p.status = $%d", q.Status)
	}
	cond := " AND " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM charge_piles p WHERE p.deleted_at IS NULL`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "p.created_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Pile
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*Pile, error) {
	var p Pile
	err := pgxscan.Get(ctx, s.db, &p, baseSelect+` AND p.tenant_id = $1 AND p.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *store) codeExists(ctx context.Context, tenantID uuid.UUID, code string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM charge_piles WHERE tenant_id = $1 AND pile_code = $2 AND deleted_at IS NULL`, tenantID, code).Scan(&n)
	return n > 0, err
}

func (s *store) create(ctx context.Context, tenantID uuid.UUID, code string, req CreateRequest) (uuid.UUID, error) {
	typ, power, connectors := "slow", 7.0, 1
	if req.Type != nil {
		typ = *req.Type
	}
	if req.PowerKw != nil {
		power = *req.PowerKw
	}
	if req.ConnectorCount != nil {
		connectors = *req.ConnectorCount
	}
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO charge_piles (tenant_id, pile_code, name, type, power_kw, connector_count, vendor, location, lng, lat, status, remark)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'offline',$11) RETURNING id`,
		tenantID, code, strings.TrimSpace(req.Name), typ, power, connectors, nilIfEmpty(req.Vendor), nilIfEmpty(req.Location),
		req.Lng, req.Lat, nilIfEmpty(req.Remark)).Scan(&id)
	return id, err
}

func (s *store) update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) error {
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.Name != nil {
		set("name", strings.TrimSpace(*req.Name))
	}
	if req.Type != nil {
		set("type", *req.Type)
	}
	if req.PowerKw != nil {
		set("power_kw", *req.PowerKw)
	}
	if req.ConnectorCount != nil {
		set("connector_count", *req.ConnectorCount)
	}
	if req.Vendor != nil {
		set("vendor", nilIfEmpty(req.Vendor))
	}
	if req.Location != nil {
		set("location", nilIfEmpty(req.Location))
	}
	if req.Lng != nil {
		set("lng", *req.Lng)
	}
	if req.Lat != nil {
		set("lat", *req.Lat)
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
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE charge_piles SET %s WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) softDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE charge_piles SET deleted_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	return err
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}
