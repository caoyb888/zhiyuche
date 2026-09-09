package user

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
	SELECT u.id, u.tenant_id, u.dept_id, d.name AS dept_name, u.username, u.name, u.phone, u.email,
	       u.employee_no, u.avatar_url, u.status, u.is_super, u.last_login_at, u.created_at, u.updated_at
	FROM users u
	LEFT JOIN departments d ON d.id = u.dept_id
	WHERE u.deleted_at IS NULL`

var sortable = map[string]string{
	"username": "u.username", "name": "u.name", "status": "u.status",
	"created_at": "u.created_at", "last_login_at": "u.last_login_at",
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) ([]User, int64, error) {
	where := []string{"u.tenant_id = $1"}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(u.username ILIKE $%[1]d OR u.name ILIKE $%[1]d OR u.phone ILIKE $%[1]d OR u.employee_no ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.DeptID != "" {
		if id, err := uuid.Parse(q.DeptID); err == nil {
			// 含子部门：通过 path 前缀匹配
			add("u.dept_id IN (SELECT id FROM departments WHERE deleted_at IS NULL AND path LIKE (SELECT path FROM departments WHERE id = $%d) || '%%')", id)
		}
	}
	if q.Status != "" {
		add("u.status = $%d", q.Status)
	}
	if q.RoleID != "" {
		if id, err := uuid.Parse(q.RoleID); err == nil {
			add("EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id AND ur.role_id = $%d)", id)
		}
	}
	cond := " AND " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM users u WHERE u.deleted_at IS NULL`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := "u.created_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []User
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachRoles(ctx, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// options returns active users of the tenant as a brief list for pickers.
func (s *store) options(ctx context.Context, tenantID uuid.UUID, q OptionsQuery) ([]UserOption, error) {
	where := []string{"u.tenant_id = $1", "u.status = 'active'"}
	args := []any{tenantID}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		where = append(where, fmt.Sprintf("(u.username ILIKE $%[1]d OR u.name ILIKE $%[1]d OR u.phone ILIKE $%[1]d)", len(args)))
	}
	if q.DeptID != "" {
		if id, err := uuid.Parse(q.DeptID); err == nil {
			args = append(args, id)
			where = append(where, fmt.Sprintf("u.dept_id IN (SELECT id FROM departments WHERE deleted_at IS NULL AND path LIKE (SELECT path FROM departments WHERE id = $%d) || '%%')", len(args)))
		}
	}
	limit := q.Limit
	if limit == 0 {
		limit = 50
	}
	args = append(args, limit)
	rows := []UserOption{}
	err := pgxscan.Select(ctx, s.db, &rows, fmt.Sprintf(`
		SELECT u.id, u.name, u.username, d.name AS dept_name, u.phone
		FROM users u LEFT JOIN departments d ON d.id = u.dept_id
		WHERE u.deleted_at IS NULL AND %s
		ORDER BY u.name LIMIT $%d`, strings.Join(where, " AND "), len(args)), args...)
	return rows, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*User, error) {
	var u User
	err := pgxscan.Get(ctx, s.db, &u, baseSelect+` AND u.tenant_id = $1 AND u.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows := []User{u}
	if err := s.attachRoles(ctx, rows); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

func (s *store) attachRoles(ctx context.Context, users []User) error {
	for i := range users {
		users[i].Roles = []RoleBrief{}
	}
	if len(users) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(users))
	idx := map[uuid.UUID]int{}
	for i, u := range users {
		ids[i] = u.ID
		idx[u.ID] = i
	}
	type row struct {
		UserID uuid.UUID `db:"user_id"`
		RoleBrief
	}
	var rows []row
	err := pgxscan.Select(ctx, s.db, &rows, `
		SELECT ur.user_id, r.id, r.code, r.name FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
		WHERE ur.user_id = ANY($1) ORDER BY r.code`, ids)
	if err != nil {
		return err
	}
	for _, r := range rows {
		i := idx[r.UserID]
		users[i].Roles = append(users[i].Roles, r.RoleBrief)
	}
	return nil
}

func (s *store) usernameExists(ctx context.Context, tenantID uuid.UUID, username string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND username = $2 AND deleted_at IS NULL`, tenantID, username).Scan(&n)
	return n > 0, err
}

func (s *store) phoneExists(ctx context.Context, tenantID uuid.UUID, phone string, exclude uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND phone = $2 AND id <> $3 AND deleted_at IS NULL`, tenantID, phone, exclude).Scan(&n)
	return n > 0, err
}

func (s *store) deptExists(ctx context.Context, tenantID, deptID uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM departments WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, deptID).Scan(&n)
	return n > 0, err
}

// validRoleIDs keeps only roles that belong to the tenant (or are platform-level when allowed).
func (s *store) validRoleIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []uuid.UUID
	err := pgxscan.Select(ctx, s.db, &out,
		`SELECT id FROM roles WHERE id = ANY($1) AND tenant_id = $2 AND deleted_at IS NULL`, ids, tenantID)
	return out, err
}

func (s *store) create(ctx context.Context, tenantID uuid.UUID, req CreateRequest, hash string, roleIDs []uuid.UUID, createdBy uuid.UUID) (uuid.UUID, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	status := req.Status
	if status == "" {
		status = "active"
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (tenant_id, dept_id, username, password_hash, name, phone, email, employee_no, status, created_by, password_changed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now()) RETURNING id`,
		tenantID, req.DeptID, req.Username, hash, req.Name, req.Phone, req.Email, req.EmployeeNo, status, createdBy).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	if err := replaceRoles(ctx, tx, id, roleIDs); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit(ctx)
}

func (s *store) update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) error {
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.Name != nil {
		set("name", *req.Name)
	}
	if req.Phone != nil {
		set("phone", nilIfEmpty(*req.Phone))
	}
	if req.Email != nil {
		set("email", nilIfEmpty(*req.Email))
	}
	if req.EmployeeNo != nil {
		set("employee_no", nilIfEmpty(*req.EmployeeNo))
	}
	if req.AvatarURL != nil {
		set("avatar_url", nilIfEmpty(*req.AvatarURL))
	}
	if req.ClearDept {
		set("dept_id", nil)
	} else if req.DeptID != nil {
		set("dept_id", *req.DeptID)
	}
	if req.Status != nil {
		set("status", *req.Status)
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE users SET %s WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) softDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// 释放用户名与手机号唯一性：软删后允许重建
	if _, err := tx.Exec(ctx, `UPDATE users SET deleted_at = now(), status = 'disabled' WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *store) setPassword(ctx context.Context, tenantID, id uuid.UUID, hash string) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $3, password_changed_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id, hash)
	return err
}

func (s *store) setRoles(ctx context.Context, id uuid.UUID, roleIDs []uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := replaceRoles(ctx, tx, id, roleIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceRoles(ctx context.Context, tx pgx.Tx, userID uuid.UUID, roleIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, rid := range roleIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, rid); err != nil {
			return err
		}
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
