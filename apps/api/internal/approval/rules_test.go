package approval

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, Shanghai)
	if err != nil {
		panic(err)
	}
	return t
}

func TestParseHHMM(t *testing.T) {
	for _, s := range []string{"00:00", "22:00", "23:59", "06:30"} {
		if _, err := ParseHHMM(s); err != nil {
			t.Errorf("%s should be valid: %v", s, err)
		}
	}
	for _, s := range []string{"24:00", "22:60", "7:00", "22-00", "", "2200"} {
		if _, err := ParseHHMM(s); err == nil {
			t.Errorf("%s should be invalid", s)
		}
	}
	if m, _ := ParseHHMM("06:30"); m != 390 {
		t.Errorf("06:30 = %d minutes, want 390", m)
	}
}

func TestNightOverlap(t *testing.T) {
	cases := []struct {
		name       string
		start, end string
		ns, ne     string
		want       bool
	}{
		{"daytime only", "2026-09-09 09:00", "2026-09-09 18:00", "22:00", "06:00", false},
		{"ends after night start", "2026-09-09 20:00", "2026-09-09 22:30", "22:00", "06:00", true},
		{"starts before night end (early morning)", "2026-09-09 05:00", "2026-09-09 08:00", "22:00", "06:00", true},
		{"ends exactly at night start", "2026-09-09 20:00", "2026-09-09 22:00", "22:00", "06:00", false},
		{"starts exactly at night end", "2026-09-09 06:00", "2026-09-09 09:00", "22:00", "06:00", false},
		{"multi-day span", "2026-09-09 09:00", "2026-09-10 09:00", "22:00", "06:00", true},
		{"window inside a single day", "2026-09-09 12:30", "2026-09-09 13:30", "12:00", "14:00", true},
		{"non-crossing window not touched", "2026-09-09 15:00", "2026-09-09 16:00", "12:00", "14:00", false},
		{"invalid window", "2026-09-09 20:00", "2026-09-09 23:00", "xx", "06:00", false},
		{"empty range", "2026-09-09 23:00", "2026-09-09 23:00", "22:00", "06:00", false},
		{"utc input converted to shanghai", "2026-09-09 14:30", "2026-09-09 15:00", "22:00", "06:00", true}, // 14:30Z == 22:30 CST
	}
	for _, c := range cases {
		s, e := at(c.start), at(c.end)
		if c.name == "utc input converted to shanghai" {
			s = time.Date(2026, 9, 9, 14, 30, 0, 0, time.UTC)
			e = time.Date(2026, 9, 9, 15, 0, 0, 0, time.UTC)
		}
		if got := NightOverlap(s, e, c.ns, c.ne); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestLevel2Reasons(t *testing.T) {
	deptA, deptB := uuid.New(), uuid.New()
	km := func(v float64) *float64 { return &v }
	base := DefaultRules()
	base.Level2Km = km(50)
	base.Level2CrossDept = true
	base.Level2TripTypes = []string{"official"}

	day := Level2Input{Start: at("2026-09-09 09:00"), End: at("2026-09-09 12:00"), TripType: "daily", ApplicantDept: &deptA}

	if got := base.Level2Reasons(day); len(got) != 0 {
		t.Errorf("plain daytime daily trip should need one level, got %v", got)
	}
	in := day
	in.PlannedKm = km(50)
	if got := base.Level2Reasons(in); len(got) != 1 {
		t.Errorf("km >= threshold: want 1 reason, got %v", got)
	}
	in = day
	in.PlannedKm = km(49.9)
	if got := base.Level2Reasons(in); len(got) != 0 {
		t.Errorf("km below threshold: want none, got %v", got)
	}
	in = day
	in.End = at("2026-09-09 23:00")
	if got := base.Level2Reasons(in); len(got) != 1 {
		t.Errorf("night overlap: want 1 reason, got %v", got)
	}
	in = day
	in.VehicleHomeDept = &deptB
	if got := base.Level2Reasons(in); len(got) != 1 {
		t.Errorf("cross dept: want 1 reason, got %v", got)
	}
	in.VehicleHomeDept = &deptA
	if got := base.Level2Reasons(in); len(got) != 0 {
		t.Errorf("same dept: want none, got %v", got)
	}
	in = day
	in.ApplicantDept = nil
	in.VehicleHomeDept = &deptB
	if got := base.Level2Reasons(in); len(got) != 1 {
		t.Errorf("applicant without dept vs vehicle with home dept counts as cross dept, got %v", got)
	}
	in = day
	in.TripType = "official"
	if got := base.Level2Reasons(in); len(got) != 1 {
		t.Errorf("trip type: want 1 reason, got %v", got)
	}
	// all at once
	in = Level2Input{PlannedKm: km(120), Start: at("2026-09-09 21:00"), End: at("2026-09-10 01:00"), TripType: "official", ApplicantDept: &deptA, VehicleHomeDept: &deptB}
	if got := base.Level2Reasons(in); len(got) != 4 {
		t.Errorf("all rules: want 4 reasons, got %v", got)
	}
	// disabled rules never escalate
	off := base
	off.Enabled = false
	if got := off.Level2Reasons(in); len(got) != 0 {
		t.Errorf("disabled rules: want none, got %v", got)
	}
	// nil threshold means the km rule is off
	noKm := base
	noKm.Level2Km = nil
	in = day
	in.PlannedKm = km(9999)
	if got := noKm.Level2Reasons(in); len(got) != 0 {
		t.Errorf("nil level2_km: want none, got %v", got)
	}
}

func TestNextStatus(t *testing.T) {
	ok := func(status, ev string, lvl, step int, want string) {
		t.Helper()
		got, err := NextStatus(status, ev, lvl, step)
		if err != nil || got != want {
			t.Errorf("%s --%s--> got (%q, %v) want %q", status, ev, got, err, want)
		}
	}
	bad := func(status, ev string, lvl, step int) {
		t.Helper()
		if _, err := NextStatus(status, ev, lvl, step); !errors.Is(err, ErrTransition) {
			t.Errorf("%s --%s--> expected ErrTransition, got %v", status, ev, err)
		}
	}
	ok(StatusPendingL1, EvApprove, 1, 1, StatusApproved)
	ok(StatusPendingL1, EvApprove, 2, 1, StatusPendingL2)
	ok(StatusPendingL2, EvApprove, 2, 2, StatusApproved)
	ok(StatusPendingL1, EvReject, 2, 1, StatusRejected)
	ok(StatusPendingL2, EvReject, 2, 2, StatusRejected)
	ok(StatusPendingL1, EvCancel, 1, 1, StatusCancelled)
	ok(StatusApproved, EvCancel, 1, 1, StatusCancelled)
	ok(StatusApproved, EvTripStart, 1, 1, StatusInUse)
	ok(StatusInUse, EvTripEnd, 1, 1, StatusCompleted)
	ok(StatusInUse, EvTripCancel, 1, 1, StatusApproved)
	ok(StatusApproved, EvExpire, 1, 1, StatusExpired)
	ok(StatusPendingL2, EvExpire, 2, 2, StatusExpired)

	bad(StatusApproved, EvApprove, 1, 1)
	bad(StatusRejected, EvApprove, 1, 1)
	bad(StatusInUse, EvCancel, 1, 1)
	bad(StatusCompleted, EvCancel, 1, 1)
	bad(StatusApproved, EvReject, 1, 1)
	bad(StatusPendingL1, EvTripStart, 1, 1)
	bad(StatusApproved, EvTripEnd, 1, 1)
	bad(StatusExpired, EvApprove, 1, 1)
	bad(StatusCancelled, EvExpire, 1, 1)
}

func TestFormatNo(t *testing.T) {
	day := time.Date(2026, 9, 9, 16, 30, 0, 0, time.UTC) // 2026-09-10 00:30 in Shanghai
	if got := FormatNo("ZY", day, 23, 4); got != "ZY-20260910-0023" {
		t.Errorf("apply no = %s", got)
	}
	if got := FormatNo("T", day, 7, 3); got != "T-20260910-007" {
		t.Errorf("trip no = %s", got)
	}
	if got := FormatNo("T", day, 1234, 3); got != "T-20260910-1234" {
		t.Errorf("overflowing width must not truncate: %s", got)
	}
	if got := DayPrefix("ZY", day); got != "ZY-20260910" {
		t.Errorf("day prefix = %s", got)
	}
}

func TestRulesUpdatePresence(t *testing.T) {
	var u RulesUpdate
	if err := json.Unmarshal([]byte(`{"level2_km": null, "night_start": "21:30", "level2_trip_types": []}`), &u); err != nil {
		t.Fatal(err)
	}
	if !u.Has("level2_km") || u.Level2Km != nil {
		t.Error("explicit null level2_km must be present and nil")
	}
	if u.Has("level2_approver_id") {
		t.Error("absent key must not be present")
	}
	if !u.Has("level2_trip_types") || len(u.Level2TripTypes) != 0 {
		t.Error("empty array must be present and empty")
	}
	if u.NightStart == nil || *u.NightStart != "21:30" {
		t.Error("night_start not parsed")
	}
	if msg := ValidateRulesUpdate(&u); msg != "" {
		t.Errorf("valid update rejected: %s", msg)
	}
	badStart := "25:00"
	u.NightStart = &badStart
	if msg := ValidateRulesUpdate(&u); msg == "" {
		t.Error("25:00 should be rejected")
	}
}

func TestDefaultRules(t *testing.T) {
	r := DefaultRules()
	if !r.Enabled || !r.Level2Night || r.NightStart != "22:00" || r.NightEnd != "06:00" || r.FallbackApproverRole != "approver" || r.OverdueAlertMinutes != 30 {
		t.Errorf("defaults differ from the table defaults: %+v", r)
	}
	if r.Level2Km != nil || r.Level2CrossDept || len(r.Level2TripTypes) != 0 || r.UpdatedAt != nil {
		t.Errorf("defaults differ from the table defaults: %+v", r)
	}
}
