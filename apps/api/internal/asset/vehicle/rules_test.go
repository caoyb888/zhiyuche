package vehicle

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func status(err error) int {
	var ae *httpx.AppError
	if errors.As(err, &ae) {
		return ae.Status
	}
	if err == nil {
		return 200
	}
	return 500
}

func TestCheckManualStatus(t *testing.T) {
	cases := []struct {
		name    string
		current string
		hasTrip bool
		target  string
		want    int
	}{
		{"idle→maintenance", "idle", false, "maintenance", 200},
		{"charging→disabled", "charging", false, "disabled", 200},
		{"maintenance→idle", "maintenance", false, "idle", 200},
		{"target in_use rejected", "idle", false, "in_use", 400},
		{"target charging rejected", "idle", false, "charging", 400},
		{"target bogus rejected", "idle", false, "bogus", 400},
		{"in_use vehicle rejected", "in_use", false, "idle", 409},
		{"open trip rejected even if status idle", "idle", true, "maintenance", 409},
	}
	for _, c := range cases {
		if got := status(CheckManualStatus(c.current, c.hasTrip, c.target)); got != c.want {
			t.Errorf("%s: want %d got %d", c.name, c.want, got)
		}
	}
}

func TestCheckDeletable(t *testing.T) {
	if got := status(CheckDeletable("idle", false, 0)); got != 200 {
		t.Errorf("idle deletable: got %d", got)
	}
	if got := status(CheckDeletable("in_use", false, 0)); got != 409 {
		t.Errorf("in_use: got %d", got)
	}
	if got := status(CheckDeletable("idle", true, 0)); got != 409 {
		t.Errorf("open trip: got %d", got)
	}
	if got := status(CheckDeletable("idle", false, 2)); got != 409 {
		t.Errorf("active approvals: got %d", got)
	}
}

func TestCheckTelemetryRange(t *testing.T) {
	now := time.Now()
	if got := status(CheckTelemetryRange(now.Add(-time.Hour), now)); got != 200 {
		t.Errorf("1h window: got %d", got)
	}
	if got := status(CheckTelemetryRange(now, now)); got != 400 {
		t.Errorf("empty window: got %d", got)
	}
	if got := status(CheckTelemetryRange(now, now.Add(-time.Minute))); got != 400 {
		t.Errorf("reversed: got %d", got)
	}
	if got := status(CheckTelemetryRange(now.Add(-8*24*time.Hour), now)); got != 400 {
		t.Errorf("8 days: got %d", got)
	}
	if got := status(CheckTelemetryRange(now.Add(-7*24*time.Hour), now)); got != 200 {
		t.Errorf("exactly 7 days: got %d", got)
	}
	if got := status(CheckTelemetryRange(time.Time{}, now)); got != 400 {
		t.Errorf("zero from: got %d", got)
	}
}

func TestParseTelemetryLimit(t *testing.T) {
	if n, _ := ParseTelemetryLimit(""); n != DefaultTelemetryLimit {
		t.Errorf("default: got %d", n)
	}
	if n, _ := ParseTelemetryLimit("9999"); n != MaxTelemetryLimit {
		t.Errorf("clamp: got %d", n)
	}
	if n, _ := ParseTelemetryLimit("42"); n != 42 {
		t.Errorf("42: got %d", n)
	}
	for _, bad := range []string{"0", "-1", "abc"} {
		if _, err := ParseTelemetryLimit(bad); status(err) != 400 {
			t.Errorf("%q should be 400", bad)
		}
	}
}

func TestDateJSON(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2026-09-09"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Year() != 2026 || d.Month() != 9 || d.Day() != 9 {
		t.Errorf("parsed %v", d.Time)
	}
	b, _ := json.Marshal(d)
	if string(b) != `"2026-09-09"` {
		t.Errorf("marshal: %s", b)
	}
	if err := json.Unmarshal([]byte(`"09/09/2026"`), &d); err == nil {
		t.Error("bad layout should fail")
	}
	// null stays nil through a pointer field
	var v struct {
		D *Date `json:"d"`
	}
	if err := json.Unmarshal([]byte(`{"d":null}`), &v); err != nil || v.D != nil {
		t.Errorf("null: err=%v d=%v", err, v.D)
	}
	// scanning from the driver
	var s Date
	if err := s.Scan(time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)); err != nil || s.Day() != 2 {
		t.Errorf("scan: err=%v day=%d", err, s.Day())
	}
}
