package device

import (
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

func TestNormalizeSerial(t *testing.T) {
	ok := []string{"SIM-0001", "abcd", "  VG_12-ab  ", "A1B2C3D4E5F6G7H8I9J0A1B2C3D4E5F6G7H8I9J0A1B2C3D4E5F6G7H8I9J0ABCD"}
	for _, s := range ok {
		if _, err := NormalizeSerial(s); err != nil {
			t.Errorf("%q should be valid: %v", s, err)
		}
	}
	bad := []string{"", "abc", "has space", "中文序列号", "a.b.c", "x" + string(make([]byte, 64))}
	for _, s := range bad {
		if _, err := NormalizeSerial(s); status(err) != 400 {
			t.Errorf("%q should be 400", s)
		}
	}
	if s, _ := NormalizeSerial("  SIM-0002 "); s != "SIM-0002" {
		t.Errorf("trim: %q", s)
	}
}

func TestIsOnline(t *testing.T) {
	now := time.Now()
	if IsOnline(nil, now) {
		t.Error("nil should be offline")
	}
	recent := now.Add(-time.Minute)
	if !IsOnline(&recent, now) {
		t.Error("1m ago should be online")
	}
	old := now.Add(-6 * time.Minute)
	if IsOnline(&old, now) {
		t.Error("6m ago should be offline")
	}
}

func TestCheckUnbind(t *testing.T) {
	if got := status(CheckUnbind("idle", false)); got != 200 {
		t.Errorf("idle: %d", got)
	}
	if got := status(CheckUnbind("in_use", false)); got != 409 {
		t.Errorf("in_use: %d", got)
	}
	if got := status(CheckUnbind("idle", true)); got != 409 {
		t.Errorf("open trip: %d", got)
	}
}

func TestCheckBind(t *testing.T) {
	v1, v2, d1, d2 := "v1", "v2", "d1", "d2"
	// free device, free vehicle
	if noop, err := CheckBind(nil, &v1, nil, &d1); err != nil || noop {
		t.Errorf("free/free: noop=%v err=%v", noop, err)
	}
	// already bound to the same vehicle → no-op
	if noop, err := CheckBind(&v1, &v1, &d1, &d1); err != nil || !noop {
		t.Errorf("same vehicle: noop=%v err=%v", noop, err)
	}
	// bound elsewhere → 409
	if _, err := CheckBind(&v1, &v2, nil, &d1); status(err) != 409 {
		t.Errorf("bound elsewhere: %v", err)
	}
	// target vehicle has another device → 409
	if _, err := CheckBind(nil, &v1, &d2, &d1); status(err) != 409 {
		t.Errorf("target taken: %v", err)
	}
}
