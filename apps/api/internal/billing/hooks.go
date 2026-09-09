package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
	"github.com/caoyb888/zhiyuche/apps/api/internal/notify"
)

// tripRow is what BillTrip reads from trips (+ driver dept, planned end, overspeed count).
type tripRow struct {
	ID            uuid.UUID  `db:"id"`
	TenantID      uuid.UUID  `db:"tenant_id"`
	TripNo        string     `db:"trip_no"`
	DriverID      *uuid.UUID `db:"driver_id"`
	DriverDeptID  *uuid.UUID `db:"driver_dept_id"`
	TripType      string     `db:"trip_type"`
	Status        string     `db:"status"`
	BillingStatus string     `db:"billing_status"`
	StartAt       time.Time  `db:"start_at"`
	EndAt         *time.Time `db:"end_at"`
	DistanceKm    *float64   `db:"distance_km"`
	EnergyKwh     *float64   `db:"energy_kwh"`
	EndSOC        *float64   `db:"end_soc"`
	HarshAccel    int        `db:"harsh_accel"`
	HarshBrake    int        `db:"harsh_brake"`
	PlannedEnd    *time.Time `db:"planned_end"`
	Overspeed     int        `db:"overspeed"`
	Cost          *float64   `db:"cost"`
	AccountTxnID  *int64     `db:"account_txn_id"`
}

const tripSelect = `
	SELECT t.id, t.tenant_id, t.trip_no, t.driver_id, u.dept_id AS driver_dept_id, t.trip_type, t.status, t.billing_status,
	       t.start_at, t.end_at, t.distance_km::float8 AS distance_km, t.energy_kwh::float8 AS energy_kwh, t.end_soc::float8 AS end_soc,
	       t.harsh_accel, t.harsh_brake, ap.planned_end,
	       (SELECT count(*) FROM trip_events e WHERE e.trip_id = t.id AND e.type = 'overspeed')::int AS overspeed,
	       t.cost::float8 AS cost, t.account_txn_id
	FROM trips t
	LEFT JOIN users u ON u.id = t.driver_id
	LEFT JOIN approvals ap ON ap.id = t.approval_id
	WHERE t.id = $1`

// tripBill is the outcome of BillTrip.
type tripBill struct {
	Trip   tripRow
	Result engine.Result
	Posted *posted
}

// BillTrip prices a completed trip, debits the attributed account and stamps
// the trip. Idempotent: a trip already 'charged' is returned untouched.
func (s *Service) BillTrip(ctx context.Context, tenantID, tripID uuid.UUID) (*tripBill, error) {
	var t tripRow
	err := pgxscan.Get(ctx, s.app.DB, &t, tripSelect, tripID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("trip %s not found", tripID)
	}
	if err != nil {
		return nil, err
	}
	if t.TenantID != tenantID {
		return nil, fmt.Errorf("trip %s belongs to another tenant", tripID)
	}
	if t.BillingStatus == BillingCharged {
		return &tripBill{Trip: t}, nil
	}
	if t.Status != "completed" || t.EndAt == nil {
		return nil, fmt.Errorf("trip %s is not completed (%s)", t.TripNo, t.Status)
	}
	rule, ruleID, err := s.effectiveRule(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	in := engine.TripInput{
		TripType: t.TripType, StartAt: t.StartAt, EndAt: *t.EndAt,
		OverspeedEvents: t.Overspeed, HarshEvents: t.HarshAccel + t.HarshBrake, EndSOC: t.EndSOC, PlannedEnd: t.PlannedEnd,
	}
	if t.DistanceKm != nil {
		in.DistanceKm = *t.DistanceKm
	}
	if t.EnergyKwh != nil {
		in.EnergyKwh = *t.EnergyKwh
	}
	res := engine.ComputeTrip(rule, in)
	tg := resolveAttribution(res.Attribution, t.DriverID, t.DriverDeptID, tenantID)
	detail, _ := json.Marshal(res)

	bill := &tripBill{Trip: t, Result: res}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		// re-check under lock so two concurrent hooks cannot both debit
		var status string
		if err := tx.QueryRow(ctx, `SELECT billing_status FROM trips WHERE id = $1 FOR UPDATE`, tripID).Scan(&status); err != nil {
			return err
		}
		if status == BillingCharged {
			return nil
		}
		dist := 0.0
		if t.DistanceKm != nil {
			dist = *t.DistanceKm
		}
		p, err := s.debit(ctx, tx, tenantID, tg, posting{
			typ: TxnTrip, amount: -res.Total, refType: RefTrip, refID: tripID.String(),
			remark: fmt.Sprintf("行程 %s 里程 %.1f km", t.TripNo, dist),
		})
		if err != nil {
			return err
		}
		var txnID *int64
		if p.TxnID != 0 {
			txnID = &p.TxnID
		}
		bill.Posted = p
		_, err = tx.Exec(ctx, `
			UPDATE trips SET cost = $2, cost_detail = $3, billing_status = 'charged', billing_rule_id = $4,
			       account_id = $5, account_txn_id = $6, billed_at = now()
			WHERE id = $1`, tripID, res.Total, detail, ruleID, p.Account.ID, txnID)
		return err
	})
	if err != nil {
		_, _ = s.app.DB.Exec(ctx, `UPDATE trips SET billing_status = 'failed' WHERE id = $1 AND billing_status <> 'charged'`, tripID)
		return nil, fmt.Errorf("bill trip %s: %w", t.TripNo, err)
	}
	if bill.Posted != nil && bill.Posted.TxnID != 0 {
		s.afterDebit(ctx, tenantID, bill.Posted)
		if t.DriverID != nil {
			label := accountLabel(bill.Posted.Account.Level, s.ownerName(ctx, bill.Posted.Account))
			_ = s.notify.Send(ctx, tenantID, []uuid.UUID{*t.DriverID}, notify.Message{
				Type: notify.TypeSystem, Title: "行程费用已扣除：" + t.TripNo,
				Content: fmt.Sprintf("行程费用 ¥%.2f 已从%s扣除，账户余额 ¥%.2f", res.Total, label, bill.Posted.BalanceAfter),
				RefType: RefTrip, RefID: tripID.String(),
			})
		}
	}
	return bill, nil
}

// NewTripHook returns the trip.CompletedHook implementation.
func NewTripHook(a *app.App) func(ctx context.Context, tenantID, tripID uuid.UUID) error {
	svc := NewService(a)
	return func(ctx context.Context, tenantID, tripID uuid.UUID) error {
		_, err := svc.BillTrip(ctx, tenantID, tripID)
		return err
	}
}

// ---- charging ----

// NewChargingHook returns the charging.BillingHook implementation.
func NewChargingHook(a *app.App) charging.BillingHook { return &chargingHook{svc: NewService(a)} }

type chargingHook struct{ svc *Service }

func (h *chargingHook) PriceAndCharge(ctx context.Context, in charging.BillingInput) (*charging.BillingResult, error) {
	return h.svc.PriceAndCharge(ctx, in)
}

// PriceAndCharge prices a charging session with the effective rule and debits
// the account chosen by rule.ev_specific.charging_attribution. Idempotent on
// (charge_transaction, tx id): an existing charge transaction is returned.
func (s *Service) PriceAndCharge(ctx context.Context, in charging.BillingInput) (*charging.BillingResult, error) {
	rule, _, err := s.effectiveRule(ctx, in.TenantID)
	if err != nil {
		return nil, err
	}
	unitPrice, cost := engine.ChargingCost(rule, in.Kwh)

	// idempotency: already debited for this session
	var prev struct {
		ID        int64     `db:"id"`
		AccountID uuid.UUID `db:"account_id"`
		Amount    float64   `db:"amount"`
		Level     string    `db:"level"`
	}
	err = pgxscan.Get(ctx, s.app.DB, &prev, `
		SELECT x.id, x.account_id, x.amount::float8 AS amount, a.level FROM account_transactions x JOIN accounts a ON a.id = x.account_id
		WHERE x.tenant_id = $1 AND x.type = 'charge' AND x.ref_type = 'charge_transaction' AND x.ref_id = $2
		ORDER BY x.id LIMIT 1`, in.TenantID, in.TxID.String())
	if err == nil {
		return &charging.BillingResult{UnitPrice: unitPrice, Cost: -prev.Amount, Attribution: prev.Level, AccountID: prev.AccountID, AccountTxnID: prev.ID}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	deptID := in.DeptID
	if deptID == nil && in.UserID != nil {
		var d *uuid.UUID
		if err := s.app.DB.QueryRow(ctx, `SELECT dept_id FROM users WHERE id = $1 AND tenant_id = $2`, *in.UserID, in.TenantID).Scan(&d); err == nil {
			deptID = d
		}
	}
	tg := resolveAttribution(rule.EVSpecific.ChargingAttribution, in.UserID, deptID, in.TenantID)
	var p *posted
	debit := func(tg target) error {
		return s.withTx(ctx, func(tx pgx.Tx) (err error) {
			p, err = s.debit(ctx, tx, in.TenantID, tg, posting{
				typ: TxnCharge, amount: -cost, refType: RefChargeTx, refID: in.TxID.String(),
				remark: fmt.Sprintf("充电 %s %.2f kWh", in.TxNo, in.Kwh),
			})
			return err
		})
	}
	if err := debit(tg); err != nil {
		if tg.Level == LevelEnterprise {
			return nil, err
		}
		// the attributed owner cannot be resolved (deleted user/department) → enterprise pays
		s.app.Log.Warn().Err(err).Str("tx", in.TxNo).Msg("charging attribution unresolvable, falling back to enterprise")
		tg = target{LevelEnterprise, in.TenantID}
		if err := debit(tg); err != nil {
			return nil, err
		}
	}
	s.afterDebit(ctx, in.TenantID, p)
	return &charging.BillingResult{UnitPrice: unitPrice, Cost: cost, Attribution: tg.Level, AccountID: p.Account.ID, AccountTxnID: p.TxnID}, nil
}
