package dept

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct{ db *pgxpool.Pool }

const baseSelect = `
	SELECT d.id, d.tenant_id, d.parent_id, d.name, d.code, d.path, d.sort, d.leader_user_id,
	       l.name AS leader_name, d.monthly_budget::float8 AS monthly_budget, d.status,
	       (SELECT count(*) FROM users u WHERE u.dept_id = d.id AND u.deleted_at IS NULL) AS user_count,
	       d.created_at, d.updated_at
	FROM departments d
	LEFT JOIN users l ON l.id = d.leader_user_id AND l.deleted_at IS NULL
	WHERE d.deleted_at IS NULL AND d.tenant_id = $1`

func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Dept, error) {
	where := []string{}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(d.name ILIKE $%[1]d OR d.code ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Status != "" {
		add("d.status = $%d", q.Status)
	}
	cond := ""
	if len(where) > 0 {
		cond = " AND " + strings.Join(where, " AND ")
	}
	var rows []Dept
	err := pgxscan.Select(ctx, s.db, &rows, baseSelect+cond+` ORDER BY d.sort, d.name, d.created_at`, args...)
	return rows, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*Dept, error) {
	var d Dept
	err := pgxscan.Get(ctx, s.db, &d, baseSelect+` AND d.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// nameExists checks sibling uniqueness (same tenant, same parent, not deleted).
func (s *store) nameExists(ctx context.Context, tenantID uuid.UUID, parentID *uuid.UUID, name string, exclude uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM departments
		WHERE tenant_id = $1 AND parent_id IS NOT DISTINCT FROM $2 AND name = $3 AND id <> $4 AND deleted_at IS NULL`,
		tenantID, parentID, name, exclude).Scan(&n)
	return n > 0, err
}

func (s *store) userExists(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, userID).Scan(&n)
	return n > 0, err
}

func (s *store) childCount(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM departments WHERE parent_id = $1 AND deleted_at IS NULL`, id).Scan(&n)
	return n, err
}

func (s *store) userCount(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE dept_id = $1 AND deleted_at IS NULL`, id).Scan(&n)
	return n, err
}

func (s *store) create(ctx context.Context, tenantID, id uuid.UUID, path string, req CreateRequest) error {
	status := req.Status
	if status == "" {
		status = "active"
	}
	sortNo := 0
	if req.Sort != nil {
		sortNo = *req.Sort
	}
	budget := 0.0
	if req.MonthlyBudget != nil {
		budget = *req.MonthlyBudget
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO departments (id, tenant_id, parent_id, name, code, path, sort, leader_user_id, monthly_budget, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		id, tenantID, req.ParentID, req.Name, nilIfEmpty(req.Code), path, sortNo, req.LeaderUserID, budget, status)
	return err
}

// move describes a parent change: the node's own parent_id plus the path
// prefix swap applied to the whole subtree.
type move struct {
	parentID *uuid.UUID
	oldPath  string
	newPath  string
}

func (s *store) update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest, mv *move) error {
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.Name != nil {
		set("name", *req.Name)
	}
	if req.Code != nil {
		set("code", nilIfEmpty(req.Code))
	}
	if req.Sort != nil {
		set("sort", *req.Sort)
	}
	if req.ClearLeader {
		set("leader_user_id", nil)
	} else if req.LeaderUserID != nil {
		set("leader_user_id", *req.LeaderUserID)
	}
	if req.MonthlyBudget != nil {
		set("monthly_budget", *req.MonthlyBudget)
	}
	if req.Status != nil {
		set("status", *req.Status)
	}
	if mv != nil {
		set("parent_id", mv.parentID)
	}
	if len(sets) == 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE departments SET %s WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...); err != nil {
		return err
	}
	if mv != nil {
		// 整棵子树（含自身）替换 path 前缀；path 只含 uuid 与 '/'，不会误配 LIKE 通配符
		if _, err := tx.Exec(ctx, `
			UPDATE departments SET path = $3 || substr(path, length($2) + 1)
			WHERE tenant_id = $1 AND path LIKE $2 || '%'`, tenantID, mv.oldPath, mv.newPath); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *store) softDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE departments SET deleted_at = now(), status = 'disabled' WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	return err
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}
