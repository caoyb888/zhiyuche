package role

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

// baseSelect: $1 = tenant id, $2 = whether platform-level (tenant_id NULL) roles are visible.
const baseSelect = `
	SELECT r.id, r.tenant_id, r.code, r.name, r.description, r.is_system,
	       (SELECT count(*) FROM user_roles ur JOIN users u ON u.id = ur.user_id AND u.deleted_at IS NULL
	         WHERE ur.role_id = r.id) AS user_count,
	       r.created_at, r.updated_at
	FROM roles r
	WHERE r.deleted_at IS NULL AND (r.tenant_id = $1 OR ($2 AND r.tenant_id IS NULL))`

var sortable = map[string]string{
	"code": "r.code", "name": "r.name", "created_at": "r.created_at", "updated_at": "r.updated_at",
}

func (s *store) isPlatform(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	var v bool
	err := s.db.QueryRow(ctx, `SELECT is_platform FROM tenants WHERE id = $1`, tenantID).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return v, err
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, platform bool, q ListQuery, pg pagination.Query) ([]Role, int64, error) {
	args := []any{tenantID, platform}
	cond := ""
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		cond = fmt.Sprintf(" AND (r.code ILIKE $%[1]d OR r.name ILIKE $%[1]d)", len(args))
	}
	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM roles r WHERE r.deleted_at IS NULL AND (r.tenant_id = $1 OR ($2 AND r.tenant_id IS NULL))`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "r.tenant_id NULLS FIRST, r.is_system DESC, r.created_at"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Role
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachPerms(ctx, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *store) get(ctx context.Context, tenantID uuid.UUID, platform bool, id uuid.UUID) (*Role, error) {
	var r Role
	err := pgxscan.Get(ctx, s.db, &r, baseSelect+` AND r.id = $3`, tenantID, platform, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows := []Role{r}
	if err := s.attachPerms(ctx, rows); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

func (s *store) attachPerms(ctx context.Context, roles []Role) error {
	for i := range roles {
		roles[i].Permissions = []string{}
	}
	if len(roles) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(roles))
	idx := map[uuid.UUID]int{}
	for i, r := range roles {
		ids[i] = r.ID
		idx[r.ID] = i
	}
	type row struct {
		RoleID uuid.UUID `db:"role_id"`
		Code   string    `db:"permission_code"`
	}
	var rows []row
	err := pgxscan.Select(ctx, s.db, &rows,
		`SELECT role_id, permission_code FROM role_permissions WHERE role_id = ANY($1) ORDER BY permission_code`, ids)
	if err != nil {
		return err
	}
	for _, r := range rows {
		i := idx[r.RoleID]
		roles[i].Permissions = append(roles[i].Permissions, r.Code)
	}
	return nil
}

func (s *store) options(ctx context.Context, tenantID uuid.UUID) ([]Brief, error) {
	var rows []Brief
	err := pgxscan.Select(ctx, s.db, &rows,
		`SELECT id, code, name FROM roles WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY code`, tenantID)
	return rows, err
}

func (s *store) codeExists(ctx context.Context, tenantID uuid.UUID, code string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM roles WHERE tenant_id = $1 AND code = $2 AND deleted_at IS NULL`, tenantID, code).Scan(&n)
	return n > 0, err
}

func (s *store) create(ctx context.Context, tenantID uuid.UUID, req CreateRequest, perms []string) (uuid.UUID, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var id uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO roles (tenant_id, code, name, description, is_system) VALUES ($1,$2,$3,$4,false) RETURNING id`,
		tenantID, req.Code, req.Name, nilIfEmpty(req.Description)).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	if err := replacePerms(ctx, tx, id, perms); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit(ctx)
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
	if req.Description != nil {
		set("description", nilIfEmpty(req.Description))
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE roles SET %s WHERE id = $1 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) softDelete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE roles SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *store) setPerms(ctx context.Context, id uuid.UUID, perms []string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := replacePerms(ctx, tx, id, perms); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replacePerms(ctx context.Context, tx pgx.Tx, roleID uuid.UUID, perms []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	for _, code := range perms {
		if _, err := tx.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_code) VALUES ($1, $2) ON CONFLICT DO NOTHING`, roleID, code); err != nil {
			return err
		}
	}
	return nil
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}
