package param

import (
	"context"
	"errors"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct{ db *pgxpool.Pool }

// effectiveSelect joins each global default with the tenant's override ($1).
const effectiveSelect = `
	SELECT g.key,
	       COALESCE(t.value, g.value)             AS value,
	       g.value_type,
	       COALESCE(t.description, g.description) AS description,
	       g.is_public,
	       CASE WHEN t.id IS NULL THEN 'global' ELSE 'tenant' END AS source,
	       g.value                                AS global_value,
	       COALESCE(t.updated_at, g.updated_at)   AS updated_at
	FROM sys_params g
	LEFT JOIN sys_params t ON t.key = g.key AND t.tenant_id = $1
	WHERE g.tenant_id IS NULL`

func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery) ([]Param, error) {
	rows := []Param{}
	sql := effectiveSelect
	args := []any{tenantID}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		sql += ` AND (g.key ILIKE $2 OR g.description ILIKE $2)`
	}
	err := pgxscan.Select(ctx, s.db, &rows, sql+` ORDER BY g.key`, args...)
	return rows, err
}

func (s *store) getEffective(ctx context.Context, tenantID uuid.UUID, key string) (*Param, error) {
	var p Param
	err := pgxscan.Get(ctx, s.db, &p, effectiveSelect+` AND g.key = $2`, tenantID, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type globalRow struct {
	Key       string `db:"key"`
	ValueType string `db:"value_type"`
}

func (s *store) getGlobal(ctx context.Context, key string) (*globalRow, error) {
	var g globalRow
	err := pgxscan.Get(ctx, s.db, &g, `SELECT key, value_type FROM sys_params WHERE tenant_id IS NULL AND key = $1`, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *store) setGlobal(ctx context.Context, key, value string, description *string, updatedBy uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE sys_params SET value = $2, description = COALESCE($3, description), updated_by = $4
		WHERE tenant_id IS NULL AND key = $1`, key, value, description, updatedBy)
	return err
}

// upsertTenant creates or updates the tenant override, copying type / public
// flag / description from the global row.
func (s *store) upsertTenant(ctx context.Context, tenantID uuid.UUID, key, value string, description *string, updatedBy uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO sys_params (tenant_id, key, value, value_type, description, is_public, updated_by)
		SELECT $1, g.key, $3, g.value_type, COALESCE($4::text, g.description), g.is_public, $5
		FROM sys_params g WHERE g.tenant_id IS NULL AND g.key = $2
		ON CONFLICT (COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), key)
		DO UPDATE SET value = EXCLUDED.value,
		              description = COALESCE($4::text, sys_params.description),
		              updated_by = EXCLUDED.updated_by`,
		tenantID, key, value, description, updatedBy)
	return err
}

// deleteTenant removes the override; false when there was none.
func (s *store) deleteTenant(ctx context.Context, tenantID uuid.UUID, key string) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM sys_params WHERE tenant_id = $1 AND key = $2`, tenantID, key)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
