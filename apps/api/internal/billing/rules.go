package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

const ruleSelect = `
	SELECT id, tenant_id, name, rule, is_default, enabled,
	       to_char(effective_from, 'YYYY-MM-DD') AS effective_from, to_char(effective_to, 'YYYY-MM-DD') AS effective_to,
	       created_by, created_at, updated_at
	FROM billing_rules WHERE deleted_at IS NULL AND tenant_id = $1`

func (s *Service) ListRules(ctx context.Context, tenantID uuid.UUID) ([]Rule, error) {
	rows := []Rule{}
	if err := pgxscan.Select(ctx, s.app.DB, &rows, ruleSelect+` ORDER BY is_default DESC, created_at DESC`, tenantID); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].hydrate()
	}
	return rows, nil
}

func (s *Service) GetRule(ctx context.Context, tenantID, id uuid.UUID) (*Rule, error) {
	var r Rule
	err := pgxscan.Get(ctx, s.app.DB, &r, ruleSelect+` AND id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("计费规则不存在")
	}
	if err != nil {
		return nil, err
	}
	r.hydrate()
	return &r, nil
}

// parseRule validates an inline rule document (400 on failure).
func parseRule(raw []byte, name string) (engine.Rule, []byte, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return engine.Rule{}, nil, httpx.BadRequest("rule 不能为空")
	}
	doc, err := engine.Parse(withRuleName(raw, name))
	if err != nil {
		return engine.Rule{}, nil, httpx.BadRequest("规则校验失败：" + err.Error())
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return engine.Rule{}, nil, err
	}
	return doc, b, nil
}

func (s *Service) CreateRule(ctx context.Context, tenantID uuid.UUID, req RuleCreateRequest, actor uuid.UUID) (*Rule, error) {
	_, doc, err := parseRule(req.Rule, req.Name)
	if err != nil {
		return nil, err
	}
	if err := checkDates(req.EffectiveFrom, req.EffectiveTo); err != nil {
		return nil, err
	}
	var id uuid.UUID
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM billing_rules WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&n); err != nil {
			return err
		}
		isDefault := n == 0 || req.Activate
		if isDefault {
			if _, err := tx.Exec(ctx, `UPDATE billing_rules SET is_default = false WHERE tenant_id = $1 AND is_default AND deleted_at IS NULL`, tenantID); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, `
			INSERT INTO billing_rules (tenant_id, name, rule, is_default, effective_from, effective_to, created_by)
			VALUES ($1, $2, $3, $4, NULLIF($5::text, '')::date, NULLIF($6::text, '')::date, $7) RETURNING id`,
			tenantID, strings.TrimSpace(req.Name), doc, isDefault, req.EffectiveFrom, req.EffectiveTo, actor).Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	return s.GetRule(ctx, tenantID, id)
}

func (s *Service) UpdateRule(ctx context.Context, tenantID, id uuid.UUID, req RuleUpdateRequest) (*Rule, *Rule, error) {
	before, err := s.GetRule(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	sets := []string{}
	args := []any{tenantID, id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	setDate := func(col string, v *string) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = NULLIF($%d::text, '')::date", col, len(args)))
	}
	name := before.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		set("name", name)
	}
	if len(req.Rule) > 0 && string(req.Rule) != "null" {
		_, doc, err := parseRule(req.Rule, name)
		if err != nil {
			return nil, nil, err
		}
		set("rule", doc)
	}
	if req.Enabled != nil {
		if !*req.Enabled && before.IsDefault {
			return nil, nil, httpx.BadRequest("生效中的规则不能停用，请先切换生效规则")
		}
		set("enabled", *req.Enabled)
	}
	from, to := before.EffectiveFrom, before.EffectiveTo
	if v, present, err := dateField(req.EffectiveFrom); err != nil {
		return nil, nil, err
	} else if present {
		from = v
		setDate("effective_from", v)
	}
	if v, present, err := dateField(req.EffectiveTo); err != nil {
		return nil, nil, err
	} else if present {
		to = v
		setDate("effective_to", v)
	}
	if err := checkDates(from, to); err != nil {
		return nil, nil, err
	}
	if len(sets) > 0 {
		if _, err := s.app.DB.Exec(ctx, fmt.Sprintf(`UPDATE billing_rules SET %s WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...); err != nil {
			return nil, nil, err
		}
	}
	after, err := s.GetRule(ctx, tenantID, id)
	return before, after, err
}

// dateField decodes an optional nullable date: absent → present=false; null → nil; "YYYY-MM-DD" → value.
func dateField(raw json.RawMessage) (*string, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false, httpx.BadRequest("日期须为 YYYY-MM-DD 字符串或 null")
	}
	if len(v) != 10 {
		return nil, false, httpx.BadRequest("日期须为 YYYY-MM-DD")
	}
	return &v, true, nil
}

func checkDates(from, to *string) error {
	if from != nil && to != nil && *from > *to {
		return httpx.BadRequest("effective_from 不能晚于 effective_to")
	}
	return nil
}

func (s *Service) DeleteRule(ctx context.Context, tenantID, id uuid.UUID) (*Rule, error) {
	r, err := s.GetRule(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if r.IsDefault {
		return nil, httpx.Conflict("生效中的规则不能删除，请先切换生效规则")
	}
	_, err = s.app.DB.Exec(ctx, `UPDATE billing_rules SET deleted_at = now(), is_default = false WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	return r, err
}

func (s *Service) ActivateRule(ctx context.Context, tenantID, id uuid.UUID) (*Rule, error) {
	r, err := s.GetRule(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if !r.Enabled {
		return nil, httpx.BadRequest("已停用的规则不能设为生效")
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE billing_rules SET is_default = false WHERE tenant_id = $1 AND is_default AND deleted_at IS NULL`, tenantID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE billing_rules SET is_default = true WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.GetRule(ctx, tenantID, id)
}

// effectiveRule returns the tenant's active rule (is_default, enabled, inside
// its effective window) or the built-in default when the tenant has none.
func (s *Service) effectiveRule(ctx context.Context, tenantID uuid.UUID) (engine.Rule, *uuid.UUID, error) {
	var r Rule
	today := s.now().In(engine.Location()).Format("2006-01-02")
	err := pgxscan.Get(ctx, s.app.DB, &r, ruleSelect+`
		AND is_default AND enabled
		AND (effective_from IS NULL OR effective_from <= ($2::text)::date)
		AND (effective_to IS NULL OR effective_to >= ($2::text)::date)`, tenantID, today)
	if errors.Is(err, pgx.ErrNoRows) {
		return engine.Default(), nil, nil
	}
	if err != nil {
		return engine.Rule{}, nil, err
	}
	doc, err := engine.Parse(r.Raw)
	if err != nil {
		// a stored rule that no longer validates must not block billing
		s.app.Log.Warn().Err(err).Str("rule", r.ID.String()).Msg("stored billing rule invalid, using built-in default")
		return engine.Default(), nil, nil
	}
	return doc, &r.ID, nil
}

// Simulate prices a hypothetical trip with rule_id / inline rule / effective rule.
func (s *Service) Simulate(ctx context.Context, tenantID uuid.UUID, req SimulateRequest) (*engine.Result, error) {
	var rule engine.Rule
	switch {
	case len(req.Rule) > 0 && string(req.Rule) != "null":
		doc, _, err := parseRule(req.Rule, "")
		if err != nil {
			return nil, err
		}
		rule = doc
	case req.RuleID != nil:
		r, err := s.GetRule(ctx, tenantID, *req.RuleID)
		if err != nil {
			return nil, err
		}
		rule, err = engine.Parse(r.Raw)
		if err != nil {
			return nil, httpx.BadRequest("规则校验失败：" + err.Error())
		}
	default:
		var err error
		if rule, _, err = s.effectiveRule(ctx, tenantID); err != nil {
			return nil, err
		}
	}
	if req.Trip.EndAt.Before(req.Trip.StartAt) {
		return nil, httpx.BadRequest("end_at 不能早于 start_at")
	}
	res := engine.ComputeTrip(rule, req.Trip.engineInput())
	return &res, nil
}
