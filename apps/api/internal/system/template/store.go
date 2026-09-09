package template

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
	SELECT n.id, n.tenant_id, (n.tenant_id IS NULL) AS is_global, n.code, n.channel, n.title, n.content, n.enabled,
	       n.created_at, n.updated_at
	FROM notification_templates n`

var sortable = map[string]string{"code": "n.code", "channel": "n.channel", "title": "n.title", "created_at": "n.created_at"}

// list merges global and tenant templates; a tenant row hides the global row
// with the same (code, channel).
func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) ([]Template, int64, error) {
	where := []string{
		"(n.tenant_id IS NULL OR n.tenant_id = $1)",
		"NOT (n.tenant_id IS NULL AND EXISTS (SELECT 1 FROM notification_templates o WHERE o.tenant_id = $1 AND o.code = n.code AND o.channel = n.channel))",
	}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(n.code ILIKE $%[1]d OR n.title ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Channel != "" {
		add("n.channel = $%d", q.Channel)
	}
	cond := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM notification_templates n`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "n.code, n.channel"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Template
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s, n.id LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, id uuid.UUID) (*Template, error) {
	var t Template
	err := pgxscan.Get(ctx, s.db, &t, baseSelect+` WHERE n.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *store) create(ctx context.Context, tenantID *uuid.UUID, req CreateRequest, enabled bool) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO notification_templates (tenant_id, code, channel, title, content, enabled)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		tenantID, req.Code, req.Channel, req.Title, req.Content, enabled).Scan(&id)
	return id, err
}

func (s *store) update(ctx context.Context, id uuid.UUID, req UpdateRequest) error {
	sets := []string{}
	args := []any{id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.Title != nil {
		set("title", *req.Title)
	}
	if req.Content != nil {
		set("content", *req.Content)
	}
	if req.Enabled != nil {
		set("enabled", *req.Enabled)
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE notification_templates SET %s WHERE id = $1`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM notification_templates WHERE id = $1`, id)
	return err
}
