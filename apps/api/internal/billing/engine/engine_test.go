package engine

import (
	"encoding/json"
	"testing"
	"time"
)

func at(h, m int) time.Time { return time.Date(2025, 6, 18, h, m, 0, 0, shanghai) }

func fp(v float64) *float64 { return &v }

// The worked example from the proposal §7.5: 47.3 km, 2h43m, 6.8 kWh at 09:05 → ¥46.40.
func TestProposalExample(t *testing.T) {
	r := Default()
	res := ComputeTrip(r, TripInput{
		TripType: "official", StartAt: at(9, 5), EndAt: at(9, 5).Add(2*time.Hour + 43*time.Minute + 14*time.Second),
		DistanceKm: 47.3, EnergyKwh: 6.8, EndSOC: fp(61),
	})
	if res.Total != 46.40 {
		t.Fatalf("total = %.2f, want 46.40; lines=%+v", res.Total, res.Lines)
	}
	if res.Multiplier != 1 || res.CapApplied || res.Penalty != 0 || res.Surcharge != 0 {
		t.Fatalf("unexpected extras: %+v", res)
	}
	if res.Attribution != AttrDepartment {
		t.Fatalf("official trip should bill the department, got %s", res.Attribution)
	}
	if len(res.Lines) != 3 {
		t.Fatalf("expected 3 lines (km, hour, electricity), got %d", len(res.Lines))
	}
}

func TestMultiplierCapSurchargePenalty(t *testing.T) {
	r := Default()
	// 08:00 start → 早高峰 ×1.3 ; 200 km, 10 h → base 120+50=170 → ×1.3 = 221 → cap 80 ; end SOC 15 → +10% surcharge + 未充电罚金 10 ; 3 overspeed → 15
	res := ComputeTrip(r, TripInput{
		TripType: "daily", StartAt: at(8, 0), EndAt: at(18, 0), DistanceKm: 200, EnergyKwh: 0, EndSOC: fp(15), OverspeedEvents: 3,
	})
	if res.Multiplier != 1.3 || !res.CapApplied {
		t.Fatalf("multiplier/cap: %+v", res)
	}
	if res.Surcharge != 8.00 { // 10% of 80
		t.Fatalf("surcharge = %.2f, want 8.00", res.Surcharge)
	}
	if res.Penalty != 25.00 { // 3×5 + 10
		t.Fatalf("penalty = %.2f, want 25.00", res.Penalty)
	}
	if res.Total != 113.00 { // 80 + 8 + 25
		t.Fatalf("total = %.2f, want 113.00; lines=%+v", res.Total, res.Lines)
	}
	if res.Attribution != AttrEmployee {
		t.Fatalf("daily trip should bill the employee")
	}
}

func TestOverspeedMaxFineAndCrossMidnight(t *testing.T) {
	r := Default()
	res := ComputeTrip(r, TripInput{StartAt: at(23, 30), EndAt: at(23, 30).Add(time.Hour), DistanceKm: 10, OverspeedEvents: 20})
	if res.Multiplier != 0.7 {
		t.Fatalf("23:30 should hit 深夜 ×0.7, got %v", res.Multiplier)
	}
	if res.Penalty != 50 {
		t.Fatalf("overspeed fine should cap at 50, got %v", res.Penalty)
	}
	res = ComputeTrip(r, TripInput{StartAt: at(5, 59), EndAt: at(6, 30), DistanceKm: 1})
	if res.Multiplier != 0.7 {
		t.Fatalf("05:59 should still be 深夜, got %v", res.Multiplier)
	}
	res = ComputeTrip(r, TripInput{StartAt: at(6, 0), EndAt: at(6, 30), DistanceKm: 1})
	if res.Multiplier != 1 {
		t.Fatalf("06:00 is outside 深夜, got %v", res.Multiplier)
	}
}

func TestLateReturnAndMultiDayCap(t *testing.T) {
	r := Default()
	r.PenaltyRules = append(r.PenaltyRules, PenaltyRule{Type: PenaltyLateReturn, Fine: 20, FinePerHour: 10})
	planned := at(12, 0)
	res := ComputeTrip(r, TripInput{StartAt: at(9, 0), EndAt: at(9, 0).Add(30 * time.Hour), DistanceKm: 1000, PlannedEnd: &planned})
	// 30h → 2 days cap = 160
	if !res.CapApplied || res.Total-res.Penalty != 160 {
		t.Fatalf("multi-day cap wrong: %+v", res)
	}
	// late by 27h → 20 + 27×10 = 290
	if res.Penalty != 290 {
		t.Fatalf("late penalty = %v, want 290", res.Penalty)
	}
}

func TestParseValidateAndDefaults(t *testing.T) {
	raw, _ := json.Marshal(Default())
	r, err := Parse(raw)
	if err != nil {
		t.Fatalf("default rule must validate: %v", err)
	}
	if r.TripAttribution.Daily != AttrEmployee {
		t.Fatalf("defaults not applied")
	}
	bad := []string{
		`{"rule_name":"x","base_rate":{"per_km":0,"per_hour":0}}`,
		`{"rule_name":"x","base_rate":{"per_km":1},"time_multipliers":[{"name":"a","range":"25:00-26:00","factor":1}]}`,
		`{"rule_name":"x","base_rate":{"per_km":1},"penalty_rules":[{"type":"nope"}]}`,
		`{"rule_name":"x","base_rate":{"per_km":1},"ev_specific":{"charging_attribution":"boss"}}`,
		`not json`,
	}
	for _, b := range bad {
		if _, err := Parse([]byte(b)); err == nil {
			t.Errorf("expected error for %s", b)
		}
	}
	if _, cost := ChargingCost(Default(), 28.4); cost != 18.46 {
		t.Fatalf("charging cost = %v, want 18.46", cost)
	}
}
