package card

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

type store struct{ db *pgxpool.Pool }

const baseFrom = `
	FROM nfc_cards c
	LEFT JOIN users u ON u.id = c.user_id AND u.deleted_at IS NULL
	WHERE c.deleted_at IS NULL`

const baseSelect = `
	SELECT c.id, c.tenant_id, c.card_uid, c.user_id, u.name AS user_name, u.username, c.status, c.issued_at, c.remark,
	       c.created_at, c.updated_at` + baseFrom

var sortable = map[string]string{
	"card_uid": "c.card_uid", "status": "c.status", "issued_at": "c.issued_at", "created_at": "c.created_at",
}

func (s *store) list(ctx context.Context, tenantID uuid.UUID, q ListQuery, pg pagination.Query) ([]Card, int64, error) {
	where := []string{"c.tenant_id = $1"}
	args := []any{tenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		add("(c.card_uid ILIKE $%[1]d OR u.name ILIKE $%[1]d OR u.username ILIKE $%[1]d)", "%"+kw+"%")
	}
	if q.Status != "" {
		add("c.status = $%d", q.Status)
	}
	if q.Bound != nil {
		if *q.Bound {
			where = append(where, "c.user_id IS NOT NULL")
		} else {
			where = append(where, "c.user_id IS NULL")
		}
	}
	cond := " AND " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRow(ctx, `SELECT count(*)`+baseFrom+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "c.created_at DESC"
	if col, ok := sortable[pg.SortBy]; ok {
		order = col
		if pg.SortDesc {
			order += " DESC"
		}
	}
	args = append(args, pg.Limit(), pg.Offset())
	var rows []Card
	err := pgxscan.Select(ctx, s.db, &rows,
		fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d", baseSelect, cond, order, len(args)-1, len(args)), args...)
	return rows, total, err
}

func (s *store) get(ctx context.Context, tenantID, id uuid.UUID) (*Card, error) {
	var c Card
	err := pgxscan.Get(ctx, s.db, &c, baseSelect+` AND c.tenant_id = $1 AND c.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *store) uidExists(ctx context.Context, tenantID uuid.UUID, uid string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM nfc_cards WHERE tenant_id = $1 AND card_uid = $2 AND deleted_at IS NULL`, tenantID, uid).Scan(&n)
	return n > 0, err
}

// userActive reports whether the user exists in the tenant and is still employed (status active).
func (s *store) userActive(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND id = $2 AND status = 'active' AND deleted_at IS NULL`, tenantID, userID).Scan(&n)
	return n > 0, err
}

func (s *store) create(ctx context.Context, tenantID uuid.UUID, uid string, req CreateRequest) (uuid.UUID, error) {
	issued := time.Now()
	if req.IssuedAt != nil {
		issued = *req.IssuedAt
	}
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO nfc_cards (tenant_id, card_uid, user_id, status, issued_at, remark)
		VALUES ($1,$2,$3,'active',$4,$5) RETURNING id`,
		tenantID, uid, req.UserID, issued, nilIfEmpty(req.Remark)).Scan(&id)
	return id, err
}

func (s *store) update(ctx context.Context, tenantID, id uuid.UUID, req UpdateRequest) error {
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
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
	_, err := s.db.Exec(ctx, fmt.Sprintf(`UPDATE nfc_cards SET %s WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...)
	return err
}

func (s *store) setUser(ctx context.Context, tenantID, id uuid.UUID, userID *uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE nfc_cards SET user_id = $3 WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id, userID)
	return err
}

func (s *store) setStatus(ctx context.Context, tenantID, id uuid.UUID, status string) error {
	_, err := s.db.Exec(ctx, `UPDATE nfc_cards SET status = $3 WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id, status)
	return err
}

func (s *store) softDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE nfc_cards SET deleted_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	return err
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}
