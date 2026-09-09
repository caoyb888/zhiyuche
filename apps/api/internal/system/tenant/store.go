package tenant

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
	SELECT t.id, t.code, t.name, t.license_no, t.contact_name, t.contact_phone, t.status, t.is_platform, t.expires_at,
	       (SELECT count(*) FROM users u WHERE u.tenant_id = t.id AND u.deleted_at IS NULL) AS user_count,
	       (SELECT count(*) FROM departments d WHERE d.tenant_id = t.id AND d.deleted_at IS NULL) AS dept_count,
	       t.created_at, t.updated_at
	FROM tenants t
	WHERE true`

var sortable = map[string]string{
	"code": "t.code", "name": "t.name", "status": "t.status",
	"created_at": "t.created_at", "expires_at": "t.expires_at",
}

func (s *store) list(ctx context.Context, q ListQuery, pg pagination.Query) ([]Tenant, int64, error) {
	where := []string{}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(t.code ILIKE $%[1]d OR t.name ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Status != "" {
		add("t.status = $%d", q.Status)
	}
	cond := ""
	if len(where) > 0 {
		cond = " AND " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM tenants t WHERE true`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "t.is_platform DESC, t.created_at"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Tenant
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	var t Tenant
	err := pgxscan.Get(ctx, s.db, &t, baseSelect+` AND t.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *store) codeExists(ctx context.Context, code string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM tenants WHERE code = $1`, code).Scan(&n)
	return n > 0, err
}

func (s *store) create(ctx context.Context, req CreateRequest) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO tenants (code, name, license_no, contact_name, contact_phone, expires_at, status, is_platform)
		VALUES ($1,$2,$3,$4,$5,$6,'active',false) RETURNING id`,
		req.Code, req.Name, nilIfEmpty(req.LicenseNo), nilIfEmpty(req.ContactName), nilIfEmpty(req.ContactPhone), req.ExpiresAt).Scan(&id)
	return id, err
}

// discard removes a half-created tenant (no other table references it yet):
// users (→ user_roles), roles (→ role_permissions), then the tenant row.
func (s *store) discard(ctx context.Context, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{
		`DELETE FROM users WHERE tenant_id = $1`,
		`DELETE FROM roles WHERE tenant_id = $1`,
		`DELETE FROM tenants WHERE id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *store) update(ctx context.Context, id uuid.UUID, req UpdateRequest) error {
	sets := []string{}
	args := []any{id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.Name != nil {
		set("name", *req.Name)
	}
	if req.LicenseNo != nil {
		set("license_no", nilIfEmpty(req.LicenseNo))
	}
	if req.ContactName != nil {
		set("contact_name", nilIfEmpty(req.ContactName))
	}
	if req.ContactPhone != nil {
		set("contact_phone", nilIfEmpty(req.ContactPhone))
	}
	if req.Status != nil {
		set("status", *req.Status)
	}
	if req.ClearExpires {
		set("expires_at", nil)
	} else if req.ExpiresAt != nil {
		set("expires_at", *req.ExpiresAt)
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE tenants SET %s WHERE id = $1`, strings.Join(sets, ", ")), args...)
	return err
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}
