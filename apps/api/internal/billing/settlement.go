package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

const settlementSelect = `
	SELECT s.id, s.tenant_id, s.period, s.dept_id, d.name AS dept_name, s.trip_count, s.trip_cost::float8 AS trip_cost,
	       s.charge_count, s.charge_cost::float8 AS charge_cost, s.penalty::float8 AS penalty, s.total::float8 AS total,
	       s.budget::float8 AS budget, s.status, s.generated_at, s.confirmed_at, s.confirmed_by, cu.name AS confirmed_by_name
	FROM settlements s
	LEFT JOIN departments d ON d.id = s.dept_id
	LEFT JOIN users cu ON cu.id = s.confirmed_by
	WHERE s.tenant_id = $1`

const settlementOrder = ` ORDER BY s.period DESC, (s.dept_id IS NULL) DESC, d.name`

func (s *Service) ListSettlements(ctx context.Context, tenantID uuid.UUID, q SettlementListQuery) ([]Settlement, error) {
	period := q.Period
	if period == "" {
		var latest *string
		if err := s.app.DB.QueryRow(ctx, `SELECT max(period) FROM settlements WHERE tenant_id = $1`, tenantID).Scan(&latest); err != nil {
			return nil, err
		}
		if latest == nil {
			return []Settlement{}, nil
		}
		period = *latest
	} else if _, _, err := periodRange(period); err != nil {
		return nil, httpx.BadRequest(err.Error())
	}
	args := []any{tenantID, period}
	cond := " AND s.period = $2"
	if q.Status != "" {
		args = append(args, q.Status)
		cond += " AND s.status = $3"
	}
	rows := []Settlement{}
	err := pgxscan.Select(ctx, s.app.DB, &rows, settlementSelect+cond+settlementOrder, args...)
	return rows, err
}

func (s *Service) GetSettlement(ctx context.Context, tenantID, id uuid.UUID) (*Settlement, error) {
	var st Settlement
	err := pgxscan.Get(ctx, s.app.DB, &st, settlementSelect+` AND s.id = $2`, tenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("结算单不存在")
	}
	if err != nil {
		return nil, err
	}
	lines, err := s.settlementLines(ctx, id)
	if err != nil {
		return nil, err
	}
	st.Lines = lines
	return &st, nil
}

func (s *Service) settlementLines(ctx context.Context, id uuid.UUID) ([]SettlementLine, error) {
	lines := []SettlementLine{}
	err := pgxscan.Select(ctx, s.app.DB, &lines, `
		SELECT id, settlement_id, kind, ref_id, ref_no, user_id, user_name, vehicle_plate, occurred_at,
		       quantity::float8 AS quantity, amount::float8 AS amount, detail
		FROM settlement_lines WHERE settlement_id = $1 ORDER BY occurred_at, id`, id)
	for i := range lines {
		lines[i].hydrate()
	}
	return lines, err
}

// Periods lists months with trip or charging data (newest first) and the
// settlement state of each.
func (s *Service) Periods(ctx context.Context, tenantID uuid.UUID) ([]PeriodInfo, error) {
	rows := []PeriodInfo{}
	err := pgxscan.Select(ctx, s.app.DB, &rows, `
		WITH p AS (
		  SELECT DISTINCT to_char(end_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM') AS period FROM trips
		  WHERE tenant_id = $1 AND status = 'completed' AND end_at IS NOT NULL
		  UNION
		  SELECT DISTINCT to_char(end_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM') FROM charge_transactions
		  WHERE tenant_id = $1 AND status = 'settled' AND end_at IS NOT NULL
		  UNION
		  SELECT DISTINCT period FROM settlements WHERE tenant_id = $1
		)
		SELECT p.period, COALESCE(s.status, 'none') AS status, COALESCE(s.total, 0)::float8 AS total
		FROM p LEFT JOIN settlements s ON s.tenant_id = $1 AND s.period = p.period AND s.dept_id IS NULL
		ORDER BY p.period DESC`, tenantID)
	return rows, err
}

// Generate (re)builds the period's draft: rebills pending/failed trips, then
// replaces the draft rows and lines. A confirmed period is immutable (409).
func (s *Service) Generate(ctx context.Context, tenantID uuid.UUID, period string) ([]Settlement, error) {
	start, end, err := periodRange(period)
	if err != nil {
		return nil, httpx.BadRequest(err.Error())
	}
	var confirmed int
	if err := s.app.DB.QueryRow(ctx, `SELECT count(*) FROM settlements WHERE tenant_id = $1 AND period = $2 AND status = 'confirmed'`, tenantID, period).Scan(&confirmed); err != nil {
		return nil, err
	}
	if confirmed > 0 {
		return nil, httpx.Conflict("该期结算单已确认，不能重算")
	}

	// 1. rebill trips that were never charged
	var pending []uuid.UUID
	if err := pgxscan.Select(ctx, s.app.DB, &pending, `
		SELECT id FROM trips WHERE tenant_id = $1 AND status = 'completed' AND billing_status IN ('pending','failed')
		  AND end_at >= $2 AND end_at < $3 ORDER BY end_at`, tenantID, start, end); err != nil {
		return nil, err
	}
	for _, id := range pending {
		if _, err := s.BillTrip(ctx, tenantID, id); err != nil {
			s.app.Log.Warn().Err(err).Str("trip_id", id.String()).Msg("settlement rebill failed")
		}
	}

	// 2. facts
	trips, err := s.tripFacts(ctx, tenantID, start, end)
	if err != nil {
		return nil, err
	}
	charges, err := s.chargeFacts(ctx, tenantID, start, end)
	if err != nil {
		return nil, err
	}
	var depts []deptInfo
	if err := pgxscan.Select(ctx, s.app.DB, &depts, `
		SELECT id, name, monthly_budget::float8 AS budget FROM departments WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY sort, name`, tenantID); err != nil {
		return nil, err
	}
	drafts := buildSettlement(trips, charges, depts)

	// 3. replace the draft
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM settlements WHERE tenant_id = $1 AND period = $2 AND status = 'draft'`, tenantID, period); err != nil {
			return err
		}
		for _, d := range drafts {
			var id uuid.UUID
			err := tx.QueryRow(ctx, `
				INSERT INTO settlements (tenant_id, period, dept_id, trip_count, trip_cost, charge_count, charge_cost, penalty, total, budget)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
				tenantID, period, d.DeptID, d.TripCount, d.TripCost, d.ChargeCount, d.ChargeCost, d.Penalty, d.Total, d.Budget).Scan(&id)
			if err != nil {
				return err
			}
			for _, l := range d.Lines {
				var detail []byte
				if l.Detail != nil {
					detail, _ = json.Marshal(l.Detail)
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO settlement_lines (settlement_id, kind, ref_id, ref_no, user_id, user_name, vehicle_plate, occurred_at, quantity, amount, detail)
					VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10,$11)`,
					id, l.Kind, l.RefID, l.RefNo, l.UserID, l.UserName, l.Plate, l.OccurredAt, l.Quantity, l.Amount, detail); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.ListSettlements(ctx, tenantID, SettlementListQuery{Period: period})
}

type tripFactRow struct {
	ID         uuid.UUID  `db:"id"`
	TripNo     string     `db:"trip_no"`
	UserID     *uuid.UUID `db:"user_id"`
	UserName   *string    `db:"user_name"`
	DeptID     *uuid.UUID `db:"dept_id"`
	Plate      *string    `db:"plate"`
	EndAt      time.Time  `db:"end_at"`
	DistanceKm *float64   `db:"distance_km"`
	Cost       float64    `db:"cost"`
	Detail     []byte     `db:"cost_detail"`
}

func (s *Service) tripFacts(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]tripFact, error) {
	var rows []tripFactRow
	err := pgxscan.Select(ctx, s.app.DB, &rows, `
		SELECT t.id, t.trip_no, t.driver_id AS user_id, u.name AS user_name, u.dept_id, v.plate_no AS plate, t.end_at,
		       t.distance_km::float8 AS distance_km, t.cost::float8 AS cost, t.cost_detail
		FROM trips t LEFT JOIN users u ON u.id = t.driver_id LEFT JOIN vehicles v ON v.id = t.vehicle_id
		WHERE t.tenant_id = $1 AND t.status = 'completed' AND t.billing_status = 'charged' AND t.cost IS NOT NULL
		  AND t.end_at >= $2 AND t.end_at < $3
		ORDER BY t.end_at`, tenantID, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]tripFact, 0, len(rows))
	for _, r := range rows {
		f := tripFact{ID: r.ID, TripNo: r.TripNo, UserID: r.UserID, UserName: r.UserName, DeptID: r.DeptID, Plate: r.Plate, EndAt: r.EndAt, DistanceKm: r.DistanceKm, Cost: r.Cost}
		if len(r.Detail) > 0 {
			var d engine.Result
			if json.Unmarshal(r.Detail, &d) == nil {
				f.Detail = &d
			}
		}
		out = append(out, f)
	}
	return out, nil
}

type chargeFactRow struct {
	ID       uuid.UUID  `db:"id"`
	TxNo     string     `db:"tx_no"`
	UserID   *uuid.UUID `db:"user_id"`
	UserName *string    `db:"user_name"`
	DeptID   *uuid.UUID `db:"dept_id"`
	Plate    *string    `db:"plate"`
	EndAt    time.Time  `db:"end_at"`
	Kwh      *float64   `db:"kwh"`
	Cost     float64    `db:"cost"`
}

func (s *Service) chargeFacts(ctx context.Context, tenantID uuid.UUID, start, end time.Time) ([]chargeFact, error) {
	var rows []chargeFactRow
	err := pgxscan.Select(ctx, s.app.DB, &rows, `
		SELECT c.id, c.tx_no, c.user_id, u.name AS user_name, COALESCE(c.dept_id, u.dept_id) AS dept_id, v.plate_no AS plate, c.end_at,
		       c.kwh::float8 AS kwh, c.cost::float8 AS cost
		FROM charge_transactions c LEFT JOIN users u ON u.id = c.user_id LEFT JOIN vehicles v ON v.id = c.vehicle_id
		WHERE c.tenant_id = $1 AND c.status = 'settled' AND c.cost IS NOT NULL AND c.end_at >= $2 AND c.end_at < $3
		ORDER BY c.end_at`, tenantID, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]chargeFact, 0, len(rows))
	for _, r := range rows {
		out = append(out, chargeFact{ID: r.ID, TxNo: r.TxNo, UserID: r.UserID, UserName: r.UserName, DeptID: r.DeptID, Plate: r.Plate, EndAt: r.EndAt, Kwh: r.Kwh, Cost: r.Cost})
	}
	return out, nil
}

// Confirm freezes a settlement row; confirming the enterprise row confirms
// every row of the period.
func (s *Service) Confirm(ctx context.Context, tenantID, id uuid.UUID, actor uuid.UUID) (*Settlement, error) {
	st, err := s.GetSettlement(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if st.Status != SettlementConfirmed {
		cond, args := "s.id = $3", []any{tenantID, actor, id}
		if st.DeptID == nil {
			cond, args = "s.period = $3", []any{tenantID, actor, st.Period}
		}
		if _, err := s.app.DB.Exec(ctx, fmt.Sprintf(`
			UPDATE settlements s SET status = 'confirmed', confirmed_at = now(), confirmed_by = $2
			WHERE s.tenant_id = $1 AND %s AND s.status = 'draft'`, cond), args...); err != nil {
			return nil, err
		}
	}
	return s.GetSettlement(ctx, tenantID, id)
}

// ExportData collects what the xlsx export needs: the period's rows and the
// detail lines of the enterprise row (or of one department).
func (s *Service) ExportData(ctx context.Context, tenantID uuid.UUID, period string, deptID *uuid.UUID) ([]Settlement, []SettlementLine, error) {
	rows, err := s.ListSettlements(ctx, tenantID, SettlementListQuery{Period: period})
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, httpx.NotFound("该期尚未生成结算单")
	}
	var detailOf *Settlement
	for i := range rows {
		if deptID == nil && rows[i].DeptID == nil {
			detailOf = &rows[i]
		}
		if deptID != nil && rows[i].DeptID != nil && *rows[i].DeptID == *deptID {
			detailOf = &rows[i]
		}
	}
	if deptID != nil {
		if detailOf == nil {
			return nil, nil, httpx.NotFound("该部门在此期无结算单")
		}
		rows = []Settlement{*detailOf}
	}
	lines := []SettlementLine{}
	if detailOf != nil {
		if lines, err = s.settlementLines(ctx, detailOf.ID); err != nil {
			return nil, nil, err
		}
	}
	return rows, lines, nil
}

func deptLabel(st Settlement) string {
	if st.DeptID == nil {
		return "企业汇总"
	}
	if st.DeptName != nil {
		return *st.DeptName
	}
	return strings.ToUpper(st.DeptID.String()[:8])
}
