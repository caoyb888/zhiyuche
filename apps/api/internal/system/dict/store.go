package dict

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

const typeSelect = `
	SELECT t.id, t.tenant_id, (t.tenant_id IS NULL) AS is_global, t.code, t.name, t.description, t.is_system,
	       t.created_at, t.updated_at
	FROM dict_types t`

const itemSelect = `
	SELECT i.id, i.dict_type_id, i.label, i.value, i.sort, i.color, i.status, i.extra, i.created_at, i.updated_at
	FROM dict_items i`

var sortable = map[string]string{"code": "t.code", "name": "t.name", "created_at": "t.created_at"}

// list returns global types plus the tenant's own; a tenant type hides the
// global type with the same code.
func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) ([]DictType, int64, error) {
	where := []string{
		"(t.tenant_id IS NULL OR t.tenant_id = $1)",
		"NOT (t.tenant_id IS NULL AND EXISTS (SELECT 1 FROM dict_types o WHERE o.tenant_id = $1 AND o.code = t.code))",
	}
	args := []any{tenantID}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		where = append(where, fmt.Sprintf("(t.code ILIKE $%[1]d OR t.name ILIKE $%[1]d)", len(args)))
	}
	cond := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM dict_types t`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "t.code"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []DictType
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s, t.id LIMIT $%d OFFSET $%d", typeSelect, cond, order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	for i := range rows {
		rows[i].Items = []DictItem{}
	}
	return rows, total, nil
}

func (s *store) getType(ctx context.Context, id uuid.UUID) (*DictType, error) {
	var t DictType
	err := pgxscan.Get(ctx, s.db, &t, typeSelect+` WHERE t.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Items = []DictItem{}
	return &t, nil
}

func (s *store) items(ctx context.Context, typeID uuid.UUID) ([]DictItem, error) {
	rows := []DictItem{}
	err := pgxscan.Select(ctx, s.db, &rows, itemSelect+` WHERE i.dict_type_id = $1 ORDER BY i.sort, i.value`, typeID)
	return rows, err
}

func (s *store) createType(ctx context.Context, tenantID *uuid.UUID, req CreateRequest) (uuid.UUID, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO dict_types (tenant_id, code, name, description, is_system)
		VALUES ($1, $2, $3, $4, false) RETURNING id`, tenantID, req.Code, req.Name, req.Description).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	for _, it := range req.Items {
		if _, err := insertItem(ctx, tx, id, it); err != nil {
			return uuid.Nil, err
		}
	}
	return id, tx.Commit(ctx)
}

// rowQuerier is satisfied by both *pgxpool.Pool and pgx.Tx.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertItem(ctx context.Context, q rowQuerier, typeID uuid.UUID, it ItemCreateRequest) (uuid.UUID, error) {
	extra, err := extraJSON(it.Extra)
	if err != nil {
		return uuid.Nil, err
	}
	status := it.Status
	if status == "" {
		status = "active"
	}
	var id uuid.UUID
	err = q.QueryRow(ctx, `
		INSERT INTO dict_items (dict_type_id, label, value, sort, color, status, extra)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		typeID, it.Label, it.Value, it.Sort, it.Color, status, extra).Scan(&id)
	return id, err
}

func (s *store) updateType(ctx context.Context, id uuid.UUID, req UpdateRequest) error {
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
		set("description", nilIfEmpty(*req.Description))
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE dict_types SET %s WHERE id = $1`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) deleteType(ctx context.Context, id uuid.UUID) error {
	// dict_items has ON DELETE CASCADE
	_, err := s.db.Exec(ctx, `DELETE FROM dict_types WHERE id = $1`, id)
	return err
}

// itemRow is an item together with its owning type (for ownership checks).
type itemRow struct {
	DictItem
	TypeTenantID *uuid.UUID `db:"type_tenant_id"`
	TypeCode     string     `db:"type_code"`
}

func (s *store) getItem(ctx context.Context, id uuid.UUID) (*itemRow, error) {
	var r itemRow
	err := pgxscan.Get(ctx, s.db, &r, `
		SELECT i.id, i.dict_type_id, i.label, i.value, i.sort, i.color, i.status, i.extra, i.created_at, i.updated_at,
		       t.tenant_id AS type_tenant_id, t.code AS type_code
		FROM dict_items i JOIN dict_types t ON t.id = i.dict_type_id
		WHERE i.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *store) createItem(ctx context.Context, typeID uuid.UUID, req ItemCreateRequest) (uuid.UUID, error) {
	return insertItem(ctx, s.db, typeID, req)
}

func (s *store) updateItem(ctx context.Context, id uuid.UUID, req ItemUpdateRequest) error {
	sets := []string{}
	args := []any{id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if req.Label != nil {
		set("label", *req.Label)
	}
	if req.Value != nil {
		set("value", *req.Value)
	}
	if req.Sort != nil {
		set("sort", *req.Sort)
	}
	if req.Color != nil {
		set("color", nilIfEmpty(*req.Color))
	}
	if req.Status != nil {
		set("status", *req.Status)
	}
	if req.Extra != nil {
		extra, err := extraJSON(req.Extra)
		if err != nil {
			return err
		}
		set("extra", extra)
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE dict_items SET %s WHERE id = $1`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) deleteItem(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM dict_items WHERE id = $1`, id)
	return err
}

// resolveTypeByCode picks the tenant's type for code, else the global one.
func (s *store) resolveTypeByCode(ctx context.Context, tenantID uuid.UUID, code string) (*DictType, error) {
	var t DictType
	err := pgxscan.Get(ctx, s.db, &t, typeSelect+`
		WHERE t.code = $1 AND (t.tenant_id = $2 OR t.tenant_id IS NULL)
		ORDER BY t.tenant_id NULLS LAST LIMIT 1`, code, tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *store) activeItems(ctx context.Context, typeID uuid.UUID) ([]DictItem, error) {
	rows := []DictItem{}
	err := pgxscan.Select(ctx, s.db, &rows, itemSelect+` WHERE i.dict_type_id = $1 AND i.status = 'active' ORDER BY i.sort, i.value`, typeID)
	return rows, err
}

func nilIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
