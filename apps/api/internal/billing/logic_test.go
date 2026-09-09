package billing

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
)

func TestResolveAttribution(t *testing.T) {
	tenant, user, dept := uuid.New(), uuid.New(), uuid.New()
	cases := []struct {
		name   string
		attr   string
		user   *uuid.UUID
		dept   *uuid.UUID
		want   target
	}{
		{"employee with user", LevelEmployee, &user, &dept, target{LevelEmployee, user}},
		{"employee without user falls back to dept", LevelEmployee, nil, &dept, target{LevelDepartment, dept}},
		{"employee without user/dept falls back to enterprise", LevelEmployee, nil, nil, target{LevelEnterprise, tenant}},
		{"department", LevelDepartment, &user, &dept, target{LevelDepartment, dept}},
		{"department without dept falls back to enterprise", LevelDepartment, &user, nil, target{LevelEnterprise, tenant}},
		{"enterprise", LevelEnterprise, &user, &dept, target{LevelEnterprise, tenant}},
		{"unknown attribution is enterprise", "boss", &user, &dept, target{LevelEnterprise, tenant}},
	}
	for _, c := range cases {
		if got := resolveAttribution(c.attr, c.user, c.dept, tenant); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
	nilID := uuid.Nil
	if got := resolveAttribution(LevelEmployee, &nilID, &nilID, tenant); got.Level != LevelEnterprise {
		t.Errorf("zero uuids must be treated as missing, got %+v", got)
	}
}

func TestAllocationRules(t *testing.T) {
	if !canAllocate(LevelEnterprise, LevelDepartment) || !canAllocate(LevelDepartment, LevelEmployee) {
		t.Fatal("hierarchical allocation must be allowed")
	}
	for _, p := range [][2]string{{LevelEnterprise, LevelEmployee}, {LevelDepartment, LevelDepartment}, {LevelEmployee, LevelEmployee}, {LevelDepartment, LevelEnterprise}} {
		if canAllocate(p[0], p[1]) {
			t.Errorf("%s → %s must be rejected", p[0], p[1])
		}
	}
	if !sufficient(10, 5, 15) || sufficient(10, 5, 15.01) || !sufficient(0.1+0.2, 0, 0.3) {
		t.Fatal("sufficient: balance + credit_limit must cover the amount (with float tolerance)")
	}
	if overdrawn(-5, 5) || !overdrawn(-5.01, 5) || overdrawn(0, 0) {
		t.Fatal("overdrawn: balance below -credit_limit")
	}
}

func TestPeriodRange(t *testing.T) {
	start, end, err := periodRange("2026-09")
	if err != nil {
		t.Fatal(err)
	}
	loc := engine.Location()
	if !start.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, loc)) || !end.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("range = %v .. %v", start, end)
	}
	// 2026-09-30 23:30 Shanghai is inside; 2026-10-01 00:00 UTC (08:00 Shanghai) is outside
	in := time.Date(2026, 9, 30, 23, 30, 0, 0, loc)
	out := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if in.Before(start) || !in.Before(end) || out.Before(end) {
		t.Fatal("boundary handling wrong")
	}
	if _, _, err := periodRange("2026-9"); err == nil {
		t.Fatal("2026-9 must be rejected")
	}
	if _, _, err := periodRange("2026-13"); err == nil {
		t.Fatal("month 13 must be rejected")
	}
	if periodOf(time.Date(2026, 8, 31, 17, 0, 0, 0, time.UTC)) != "2026-09" { // 01:00 Sep 1 Shanghai
		t.Fatal("periodOf must use Asia/Shanghai")
	}
	ms, me := monthBounds(time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC))
	if ms.Month() != 2 || me.Month() != 3 || ms.Day() != 1 {
		t.Fatalf("monthBounds = %v .. %v", ms, me)
	}
}

func TestWithRuleName(t *testing.T) {
	raw := []byte(`{"base_rate":{"per_km":1}}`)
	var m map[string]any
	if err := json.Unmarshal(withRuleName(raw, "套餐A"), &m); err != nil || m["rule_name"] != "套餐A" {
		t.Fatalf("rule_name not filled: %v %v", m, err)
	}
	raw = []byte(`{"rule_name":"inner","base_rate":{"per_km":1}}`)
	json.Unmarshal(withRuleName(raw, "outer"), &m)
	if m["rule_name"] != "inner" {
		t.Fatal("existing rule_name must be kept")
	}
	if string(withRuleName([]byte(`not json`), "x")) != "not json" {
		t.Fatal("invalid JSON passes through untouched")
	}
}

func TestSplitTripCostAndLines(t *testing.T) {
	res := engine.ComputeTrip(engine.Default(), engine.TripInput{
		TripType: "daily", StartAt: time.Date(2026, 9, 1, 8, 0, 0, 0, engine.Location()), EndAt: time.Date(2026, 9, 1, 18, 0, 0, 0, engine.Location()),
		DistanceKm: 200, EndSOC: fp(15), OverspeedEvents: 3,
	})
	// 80 cap + 8 surcharge + 25 penalty = 113
	tc, pen := splitTripCost(res.Total, &res)
	if tc != 88 || pen != 25 {
		t.Fatalf("split = %v / %v, want 88 / 25", tc, pen)
	}
	if tc, pen := splitTripCost(12.5, nil); tc != 12.5 || pen != 0 {
		t.Fatal("no detail → all trip cost")
	}
	id := uuid.New()
	lines, _, _ := tripLines(tripFact{ID: id, TripNo: "T-1", EndAt: time.Now(), Cost: res.Total, Detail: &res})
	if len(lines) != 2 || lines[0].Kind != "trip" || lines[1].Kind != "penalty" || lines[1].Amount != 25 || lines[0].Amount != 88 {
		t.Fatalf("lines = %+v", lines)
	}
	det := lines[1].Detail.(map[string]any)["lines"].([]engine.Line)
	if len(det) != 2 { // overspeed + not charging
		t.Fatalf("penalty detail lines = %d", len(det))
	}
}

func TestBuildSettlement(t *testing.T) {
	d1, d2 := uuid.New(), uuid.New()
	u1, u2 := uuid.New(), uuid.New()
	pen := engine.Result{Penalty: 10, Lines: []engine.Line{{Item: "里程费", Kind: "base", Amount: 30}, {Item: "超速罚金", Kind: "penalty", Amount: 10}}}
	day := func(d int) time.Time { return time.Date(2026, 9, d, 10, 0, 0, 0, time.UTC) }
	trips := []tripFact{
		{ID: uuid.New(), TripNo: "T-3", UserID: &u1, DeptID: &d1, EndAt: day(3), Cost: 40, Detail: &pen},
		{ID: uuid.New(), TripNo: "T-1", UserID: &u1, DeptID: &d1, EndAt: day(1), Cost: 20.5},
		{ID: uuid.New(), TripNo: "T-9", UserID: &u2, DeptID: nil, EndAt: day(9), Cost: 5}, // 未分配部门：只进企业行
	}
	charges := []chargeFact{
		{ID: uuid.New(), TxNo: "C-1", UserID: &u2, DeptID: &d2, EndAt: day(2), Cost: 18.46},
		{ID: uuid.New(), TxNo: "C-2", DeptID: nil, EndAt: day(4), Cost: 1.04},
	}
	rows := buildSettlement(trips, charges, []deptInfo{{d1, "研发部", 1000}, {d2, "车队部", 500}})
	if len(rows) != 3 || rows[0].DeptID != nil {
		t.Fatalf("rows = %d, first must be enterprise", len(rows))
	}
	ent := rows[0]
	if ent.TripCount != 3 || ent.TripCost != 55.5 || ent.Penalty != 10 || ent.ChargeCount != 2 || ent.ChargeCost != 19.5 || ent.Total != 85 || ent.Budget != 1500 {
		t.Fatalf("enterprise row = %+v", ent)
	}
	if len(ent.Lines) != 6 || ent.Lines[0].RefNo != "T-1" || ent.Lines[len(ent.Lines)-1].RefNo != "T-9" {
		t.Fatalf("enterprise lines must be all facts in time order, got %d: %+v", len(ent.Lines), ent.Lines)
	}
	r1, r2 := rows[1], rows[2]
	if *r1.DeptID != d1 || r1.TripCount != 2 || r1.TripCost != 50.5 || r1.Penalty != 10 || r1.ChargeCount != 0 || r1.Total != 60.5 || r1.Budget != 1000 || len(r1.Lines) != 3 {
		t.Fatalf("dept1 row = %+v", r1)
	}
	if *r2.DeptID != d2 || r2.TripCount != 0 || r2.ChargeCount != 1 || r2.ChargeCost != 18.46 || r2.Total != 18.46 || len(r2.Lines) != 1 {
		t.Fatalf("dept2 row = %+v", r2)
	}
	// dept rows sum to less than enterprise exactly by the unassigned facts (5 + 1.04)
	if round2(ent.Total-r1.Total-r2.Total) != 6.04 {
		t.Fatalf("unassigned share = %v", ent.Total-r1.Total-r2.Total)
	}
	if usageRate(65, 1000) != "6.5%" || usageRate(1, 0) != "-" {
		t.Fatal("usageRate")
	}
}

func TestTripInputDTO(t *testing.T) {
	in := TripInputDTO{StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), DistanceKm: 10}.engineInput()
	if in.TripType != "official" {
		t.Fatal("trip_type defaults to official")
	}
	if accountLabel(LevelEmployee, "张三") != "员工账户（张三）" || accountLabel(LevelEnterprise, "x") != "企业账户" {
		t.Fatal("accountLabel")
	}
}

func fp(v float64) *float64 { return &v }
