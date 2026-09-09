package auditlog

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

const listColumns = `
	a.id, a.tenant_id, a.user_id, a.username, a.action, a.module, a.target_type, a.target_id, a.summary,
	COALESCE(a.method, '') AS method, COALESCE(a.path, '') AS path, COALESCE(a.status, 0) AS status,
	a.ip, a.user_agent, a.request_id, a.created_at`

var sortable = map[string]string{"id": "a.id", "created_at": "a.created_at", "module": "a.module", "action": "a.action"}

// tenantFilter is nil for the platform-wide view (super admin without X-Tenant-ID).
func (s *store) list(ctx context.Context, tenantFilter *uuid.UUID, f filter, pg pagination.Query) ([]AuditLog, int64, error) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if tenantFilter != nil {
		add("a.tenant_id = $%d", *tenantFilter)
	}
	if f.UserID != nil {
		add("a.user_id = $%d", *f.UserID)
	}
	if f.Username != "" {
		add("a.username ILIKE $%d", "%"+f.Username+"%")
	}
	if f.Module != "" {
		add("a.module = $%d", f.Module)
	}
	if f.Action != "" {
		add("a.action = $%d", f.Action)
	}
	if f.TargetID != "" {
		add("a.target_id = $%d", f.TargetID)
	}
	if f.Keyword != "" {
		add("(a.summary ILIKE $%[1]d OR a.path ILIKE $%[1]d)", "%"+f.Keyword+"%")
	}
	if f.From != nil {
		add("a.created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("a.created_at <= $%d", *f.To)
	}
	cond := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM audit_logs a`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "a.id DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []AuditLog
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("SELECT %s FROM audit_logs a%s ORDER BY %s LIMIT $%d OFFSET $%d", listColumns, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantFilter *uuid.UUID, id int64) (*AuditLog, error) {
	sql := `SELECT ` + listColumns + `, a.before, a.after FROM audit_logs a WHERE a.id = $1`
	args := []any{id}
	if tenantFilter != nil {
		sql += ` AND a.tenant_id = $2`
		args = append(args, *tenantFilter)
	}
	var row AuditLog
	err := pgxscan.Get(ctx, s.db, &row, sql, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
