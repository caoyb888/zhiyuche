package billing

// Integration tests against a real PostgreSQL (migrations applied). They run
// only when ZY_TEST_DATABASE_URL is set, e.g.
//
//	ZY_TEST_DATABASE_URL=postgres://zhiyuche:zhiyuche@localhost:20432/zhiyuche_be1?sslmode=disable go test ./internal/billing/
//
// Every test seeds its own tenant and removes it afterwards.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

type fixture struct {
	ctx     context.Context
	db      *pgxpool.Pool
	svc     *Service
	tenant  uuid.UUID
	dept    uuid.UUID
	driver  uuid.UUID
	leader  uuid.UUID
	vehicle uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("ZY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ZY_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	a := &app.App{Log: zerolog.Nop(), DB: db, Hub: ws.NewHub(zerolog.Nop())}
	f := &fixture{ctx: ctx, db: db, svc: NewService(a)}
	sfx := fmt.Sprintf("%d%04d", time.Now().Unix()%100000, rand.Intn(10000))

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	one := func(sql string, args ...any) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := db.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
		return id
	}
	f.tenant = one(`INSERT INTO tenants (code, name) VALUES ($1, $2) RETURNING id`, "bt_"+sfx, "计费集成测试租户")
	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM settlement_lines WHERE settlement_id IN (SELECT id FROM settlements WHERE tenant_id = $1)`,
			`DELETE FROM settlements WHERE tenant_id = $1`,
			`DELETE FROM account_transactions WHERE tenant_id = $1`,
			`DELETE FROM accounts WHERE tenant_id = $1`,
			`DELETE FROM billing_rules WHERE tenant_id = $1`,
			`DELETE FROM notifications WHERE tenant_id = $1`,
			`DELETE FROM trip_events WHERE tenant_id = $1`,
			`DELETE FROM trips WHERE tenant_id = $1`,
			`DELETE FROM vehicles WHERE tenant_id = $1`,
			`UPDATE departments SET leader_user_id = NULL WHERE tenant_id = $1`,
			`DELETE FROM users WHERE tenant_id = $1`,
			`DELETE FROM departments WHERE tenant_id = $1`,
			`DELETE FROM audit_logs WHERE tenant_id = $1`,
			`DELETE FROM tenants WHERE id = $1`,
		} {
			if _, err := db.Exec(ctx, sql, f.tenant); err != nil {
				t.Logf("cleanup %s: %v", sql, err)
			}
		}
		db.Close()
	})
	f.dept = one(`INSERT INTO departments (tenant_id, name, path, monthly_budget) VALUES ($1, '测试部', '/', 1000) RETURNING id`, f.tenant)
	exec(`UPDATE departments SET path = '/' || id || '/' WHERE id = $1`, f.dept)
	user := func(name string) uuid.UUID {
		return one(`INSERT INTO users (tenant_id, dept_id, username, password_hash, name) VALUES ($1, $2, $3, 'x', $4) RETURNING id`, f.tenant, f.dept, name+"_"+sfx, name)
	}
	f.driver, f.leader = user("driver"), user("leader")
	exec(`UPDATE departments SET leader_user_id = $2 WHERE id = $1`, f.dept, f.leader)
	f.vehicle = one(`INSERT INTO vehicles (tenant_id, plate_no, battery_kwh, status, odometer_km) VALUES ($1, $2, 60, 'idle', 1000) RETURNING id`, f.tenant, "计"+sfx[len(sfx)-6:])
	return f
}

// completedTrip inserts a finished trip: 47.3 km / 2h43m14s / 6.8 kWh at 09:05 (proposal example → ¥46.40 with the default rule).
func (f *fixture) completedTrip(t *testing.T, tripType string, start time.Time, overspeed int) uuid.UUID {
	t.Helper()
	end := start.Add(2*time.Hour + 43*time.Minute + 14*time.Second)
	var id uuid.UUID
	err := f.db.QueryRow(f.ctx, `
		INSERT INTO trips (tenant_id, trip_no, vehicle_id, driver_id, trip_type, source, status, start_at, end_at, distance_km, energy_kwh, end_soc)
		VALUES ($1, $2, $3, $4, $5, 'simulator', 'completed', $6, $7, 47.3, 6.8, 61) RETURNING id`,
		f.tenant, fmt.Sprintf("T-BT-%d", rand.Intn(1_000_000)), f.vehicle, f.driver, tripType, start, end).Scan(&id)
	if err != nil {
		t.Fatalf("insert trip: %v", err)
	}
	for i := 0; i < overspeed; i++ {
		if _, err := f.db.Exec(f.ctx, `INSERT INTO trip_events (tenant_id, trip_id, vehicle_id, type, ts) VALUES ($1, $2, $3, 'overspeed', $4)`, f.tenant, id, f.vehicle, start.Add(time.Duration(i+1)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func at0905() time.Time { return time.Date(2026, 6, 18, 9, 5, 0, 0, engine.Location()) }

func TestTripHookDebitAndIdempotent(t *testing.T) {
	f := newFixture(t)
	tripID := f.completedTrip(t, "official", at0905(), 0)
	hook := NewTripHook(f.svc.app)
	if err := hook(f.ctx, f.tenant, tripID); err != nil {
		t.Fatalf("hook: %v", err)
	}
	var status string
	var cost float64
	var accountID uuid.UUID
	var txnID int64
	if err := f.db.QueryRow(f.ctx, `SELECT billing_status, cost::float8, account_id, account_txn_id FROM trips WHERE id = $1`, tripID).Scan(&status, &cost, &accountID, &txnID); err != nil {
		t.Fatal(err)
	}
	if status != BillingCharged || cost != 46.40 || txnID == 0 {
		t.Fatalf("trip not charged as expected: status=%s cost=%v txn=%d", status, cost, txnID)
	}
	// official trip → department account (driver's department); budget copied from the department
	acc, err := f.svc.GetAccount(f.ctx, f.tenant, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Level != LevelDepartment || acc.OwnerID != f.dept || acc.Balance != -46.40 || acc.MonthlyBudget != 1000 {
		t.Fatalf("department account = %+v", acc)
	}
	txn, err := f.svc.GetTransaction(f.ctx, f.tenant, txnID)
	if err != nil {
		t.Fatal(err)
	}
	if txn.Type != TxnTrip || txn.Amount != -46.40 || txn.RefNo == nil || *txn.RefNo == "" || txn.AccountName != "测试部" {
		t.Fatalf("transaction = %+v", txn)
	}
	// driver was told
	if f.count(t, `SELECT count(*) FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND type = 'system' AND title LIKE '行程费用已扣除%'`, f.tenant, f.driver) != 1 {
		t.Fatal("driver notification missing")
	}
	// idempotent: a second call neither debits nor errors
	if err := hook(f.ctx, f.tenant, tripID); err != nil {
		t.Fatalf("second hook: %v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM account_transactions WHERE tenant_id = $1 AND type = 'trip'`, f.tenant); n != 1 {
		t.Fatalf("expected exactly 1 trip transaction, got %d", n)
	}
	// month_spent counts the debit only when it falls in the current month
	if ms, _ := monthBounds(time.Now()); txn.CreatedAt.After(ms) {
		if acc, _ = f.svc.GetAccount(f.ctx, f.tenant, accountID); acc.MonthSpent != 46.40 {
			t.Fatalf("month_spent = %v", acc.MonthSpent)
		}
	}
}

func TestTripHookRebillsFailedAndSkipsNonCompleted(t *testing.T) {
	f := newFixture(t)
	hook := NewTripHook(f.svc.app)
	var ongoing uuid.UUID
	if err := f.db.QueryRow(f.ctx, `INSERT INTO trips (tenant_id, trip_no, vehicle_id, trip_type, status, start_at) VALUES ($1, 'T-BT-ONGOING', $2, 'official', 'ongoing', now()) RETURNING id`, f.tenant, f.vehicle).Scan(&ongoing); err != nil {
		t.Fatal(err)
	}
	if err := hook(f.ctx, f.tenant, ongoing); err == nil {
		t.Fatal("ongoing trip must not be billed")
	}
	// a trip marked failed is billed on retry (what settlement generation does)
	tripID := f.completedTrip(t, "official", at0905(), 0)
	if _, err := f.db.Exec(f.ctx, `UPDATE trips SET billing_status = 'failed' WHERE id = $1`, tripID); err != nil {
		t.Fatal(err)
	}
	if err := hook(f.ctx, f.tenant, tripID); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM trips WHERE id = $1 AND billing_status = 'charged'`, tripID) != 1 {
		t.Fatal("failed trip not rebilled")
	}
}

func TestAllocateInsufficientBalance(t *testing.T) {
	f := newFixture(t)
	ent, err := f.svc.EnterpriseAccount(f.ctx, f.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Recharge(f.ctx, f.tenant, RechargeRequest{Amount: 100}, f.leader); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Allocate(f.ctx, f.tenant, ent.ID, AllocateRequest{ToLevel: LevelDepartment, ToOwnerID: &f.dept, Amount: 150}, f.leader)
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Status != 409 {
		t.Fatalf("expected 409 for insufficient balance, got %v", err)
	}
	// enterprise → employee is not a valid hop
	_, err = f.svc.Allocate(f.ctx, f.tenant, ent.ID, AllocateRequest{ToLevel: LevelEmployee, ToOwnerID: &f.driver, Amount: 10}, f.leader)
	if !errors.As(err, &ae) || ae.Status != 400 {
		t.Fatalf("expected 400 for enterprise→employee, got %v", err)
	}
	out, err := f.svc.Allocate(f.ctx, f.tenant, ent.ID, AllocateRequest{ToLevel: LevelDepartment, ToOwnerID: &f.dept, Amount: 60, Remark: "六月额度"}, f.leader)
	if err != nil {
		t.Fatal(err)
	}
	if out.From.Type != TxnAllocateOut || out.From.Amount != -60 || out.From.BalanceAfter != 40 || out.To.Type != TxnAllocateIn || out.To.BalanceAfter != 60 {
		t.Fatalf("allocation = %+v", out)
	}
	if out.From.RefID == nil || *out.From.RefID != out.To.AccountID.String() || *out.To.RefID != ent.ID.String() {
		t.Fatal("allocation refs must point at the counterpart account")
	}
	// credit limit counts as available funds
	if _, _, err := f.svc.UpdateAccount(f.ctx, f.tenant, ent.ID, AccountUpdateRequest{CreditLimit: fp(100)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Allocate(f.ctx, f.tenant, ent.ID, AllocateRequest{ToAccountID: &out.To.AccountID, Amount: 120}, f.leader); err != nil {
		t.Fatalf("allocation within credit limit must succeed: %v", err)
	}
	if acc, _ := f.svc.GetAccount(f.ctx, f.tenant, ent.ID); acc.Balance != -80 {
		t.Fatalf("enterprise balance = %v", acc.Balance)
	}
	// department → employee creates the employee account on demand
	res, err := f.svc.Allocate(f.ctx, f.tenant, out.To.AccountID, AllocateRequest{ToLevel: LevelEmployee, ToOwnerID: &f.driver, Amount: 30}, f.leader)
	if err != nil {
		t.Fatal(err)
	}
	me, err := f.svc.MyAccount(f.ctx, f.tenant, f.driver)
	if err != nil || !me.Exists || me.Balance != 30 || me.ID != res.To.AccountID || len(me.Transactions) != 1 {
		t.Fatalf("my account = %+v err=%v", me, err)
	}
	if adj, err := f.svc.Adjust(f.ctx, f.tenant, me.ID, AdjustRequest{Amount: -5, Remark: "测试扣减"}, f.leader); err != nil || adj.BalanceAfter != 25 {
		t.Fatalf("adjust: %+v %v", adj, err)
	}
}

func TestOverdraftNotifiesOwner(t *testing.T) {
	f := newFixture(t)
	// daily trip → employee account; no balance, credit_limit 0 → overdraft → driver notified
	tripID := f.completedTrip(t, "daily", at0905(), 2)
	if _, err := f.svc.BillTrip(f.ctx, f.tenant, tripID); err != nil {
		t.Fatal(err)
	}
	me, err := f.svc.MyAccount(f.ctx, f.tenant, f.driver)
	if err != nil {
		t.Fatal(err)
	}
	if !me.Exists || me.Balance != -56.40 || me.Status != "active" { // 46.40 + 2 × 5 overspeed
		t.Fatalf("employee account = %+v", me.Account)
	}
	if f.count(t, `SELECT count(*) FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND type = 'system' AND title = '账户余额不足'`, f.tenant, f.driver) != 1 {
		t.Fatal("employee overdraft notification missing")
	}
	// department overdraft → leader
	dep := f.completedTrip(t, "official", at0905(), 0)
	if _, err := f.svc.BillTrip(f.ctx, f.tenant, dep); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND type = 'system' AND title = '账户余额不足'`, f.tenant, f.leader) != 1 {
		t.Fatal("department leader overdraft notification missing")
	}
	// within the credit limit no warning is sent
	deptAcc, _ := f.svc.findAccount(f.ctx, f.tenant, target{LevelDepartment, f.dept})
	if _, _, err := f.svc.UpdateAccount(f.ctx, f.tenant, deptAcc.ID, AccountUpdateRequest{CreditLimit: fp(1000)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.BillTrip(f.ctx, f.tenant, f.completedTrip(t, "official", at0905(), 0)); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND title = '账户余额不足'`, f.tenant, f.leader) != 1 {
		t.Fatal("no new warning expected inside the credit limit")
	}
}

func TestChargingHookIdempotent(t *testing.T) {
	f := newFixture(t)
	hook := NewChargingHook(f.svc.app)
	in := charging.BillingInput{TenantID: f.tenant, TxID: uuid.New(), TxNo: "C-BT-1", Kwh: 28.4, UserID: &f.driver, StartAt: time.Now().Add(-time.Hour), EndAt: time.Now()}
	r1, err := hook.PriceAndCharge(f.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	// default rule: 0.65 元/kWh, charging attribution department (driver's dept)
	if r1.UnitPrice != 0.65 || r1.Cost != 18.46 || r1.Attribution != LevelDepartment || r1.AccountTxnID == 0 {
		t.Fatalf("result = %+v", r1)
	}
	r2, err := hook.PriceAndCharge(f.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if r2.AccountTxnID != r1.AccountTxnID || r2.AccountID != r1.AccountID || r2.Cost != r1.Cost {
		t.Fatalf("second call must return the existing debit: %+v vs %+v", r1, r2)
	}
	if n := f.count(t, `SELECT count(*) FROM account_transactions WHERE tenant_id = $1 AND type = 'charge'`, f.tenant); n != 1 {
		t.Fatalf("expected 1 charge transaction, got %d", n)
	}
	// employee attribution via a tenant rule
	rule := engine.Default()
	rule.EVSpecific.ChargingAttribution = LevelEmployee
	raw, _ := json.Marshal(rule)
	if _, err := f.svc.CreateRule(f.ctx, f.tenant, RuleCreateRequest{Name: "员工付费", Rule: raw}, f.leader); err != nil {
		t.Fatal(err)
	}
	r3, err := hook.PriceAndCharge(f.ctx, charging.BillingInput{TenantID: f.tenant, TxID: uuid.New(), TxNo: "C-BT-2", Kwh: 10, UserID: &f.driver})
	if err != nil || r3.Attribution != LevelEmployee || r3.Cost != 6.5 {
		t.Fatalf("employee attribution: %+v %v", r3, err)
	}
	// unresolvable user → enterprise
	ghost := uuid.New()
	r4, err := hook.PriceAndCharge(f.ctx, charging.BillingInput{TenantID: f.tenant, TxID: uuid.New(), TxNo: "C-BT-3", Kwh: 1, UserID: &ghost})
	if err != nil || r4.Attribution != LevelEnterprise {
		t.Fatalf("fallback to enterprise: %+v %v", r4, err)
	}
}

func TestSettlementGenerateConfirm(t *testing.T) {
	f := newFixture(t)
	start := at0905()
	period := periodOf(start)
	t1 := f.completedTrip(t, "official", start, 3)         // 46.40 + 15 penalty
	t2 := f.completedTrip(t, "daily", start.Add(24*time.Hour), 0) // 46.40, employee
	_ = t1
	_ = t2
	// a settled charging session with cost in the period (no pile needed for the aggregation? pile_id is NOT NULL → create one)
	var pile uuid.UUID
	if err := f.db.QueryRow(f.ctx, `INSERT INTO charge_piles (tenant_id, pile_code, name) VALUES ($1, $2, '测试桩') RETURNING id`, f.tenant, "P-"+uuid.NewString()[:8]).Scan(&pile); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = f.db.Exec(f.ctx, `DELETE FROM charge_transactions WHERE tenant_id = $1`, f.tenant)
		_, _ = f.db.Exec(f.ctx, `DELETE FROM charge_piles WHERE tenant_id = $1`, f.tenant)
	})
	if _, err := f.db.Exec(f.ctx, `
		INSERT INTO charge_transactions (tenant_id, tx_no, pile_id, id_tag, user_id, dept_id, status, start_at, end_at, kwh, cost)
		VALUES ($1, $2, $3, 'TAG', $4, $5, 'settled', $6, $7, 28.4, 18.46)`,
		f.tenant, "C-BT-"+uuid.NewString()[:8], pile, f.driver, f.dept, start, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Generate(f.ctx, f.tenant, period)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].DeptID != nil || rows[1].DeptID == nil {
		t.Fatalf("rows = %+v", rows)
	}
	ent, dep := rows[0], rows[1]
	if ent.TripCount != 2 || ent.TripCost != 92.80 || ent.Penalty != 15 || ent.ChargeCount != 1 || ent.ChargeCost != 18.46 || ent.Total != 126.26 || ent.Budget != 1000 {
		t.Fatalf("enterprise row = %+v", ent)
	}
	if dep.TripCount != 2 || dep.Total != 126.26 || dep.Budget != 1000 || dep.DeptName == nil || *dep.DeptName != "测试部" {
		t.Fatalf("department row = %+v", dep)
	}
	// pending trips were billed by generate (both trips were pending before)
	if f.count(t, `SELECT count(*) FROM trips WHERE tenant_id = $1 AND billing_status = 'charged'`, f.tenant) != 2 {
		t.Fatal("generate must bill pending trips")
	}
	det, err := f.svc.GetSettlement(f.ctx, f.tenant, ent.ID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, l := range det.Lines {
		kinds[l.Kind]++
	}
	if kinds["trip"] != 2 || kinds["penalty"] != 1 || kinds["charge"] != 1 {
		t.Fatalf("lines = %v", kinds)
	}
	periods, err := f.svc.Periods(f.ctx, f.tenant)
	if err != nil || len(periods) != 1 || periods[0].Period != period || periods[0].Status != SettlementDraft || periods[0].Total != 126.26 {
		t.Fatalf("periods = %+v %v", periods, err)
	}
	// regenerate replaces the draft (same totals, no duplicates)
	if rows, err = f.svc.Generate(f.ctx, f.tenant, period); err != nil || len(rows) != 2 {
		t.Fatalf("regenerate: %v", err)
	}
	// confirm one department row only → enterprise row still draft
	if st, err := f.svc.Confirm(f.ctx, f.tenant, rows[1].ID, f.leader); err != nil || st.Status != SettlementConfirmed || st.ConfirmedByName == nil {
		t.Fatalf("confirm dept: %+v %v", st, err)
	}
	if _, err := f.svc.Generate(f.ctx, f.tenant, period); err == nil {
		t.Fatal("a period with a confirmed row must not be regenerated")
	}
	if st, err := f.svc.Confirm(f.ctx, f.tenant, rows[0].ID, f.leader); err != nil || st.Status != SettlementConfirmed {
		t.Fatalf("confirm enterprise: %v", err)
	}
	if f.count(t, `SELECT count(*) FROM settlements WHERE tenant_id = $1 AND status = 'draft'`, f.tenant) != 0 {
		t.Fatal("confirming the enterprise row must confirm the whole period")
	}
	_, lines, err := f.svc.ExportData(f.ctx, f.tenant, period, nil)
	if err != nil || len(lines) != 4 {
		t.Fatalf("export data: %d lines, %v", len(lines), err)
	}
	if xf, err := buildExport(period, rows, lines); err != nil || xf.GetSheetName(1) != detailSheet {
		t.Fatalf("xlsx: %v", err)
	}
}
