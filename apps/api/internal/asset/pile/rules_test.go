package pile

import (
	"errors"
	"testing"

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

func TestNormalizeCode(t *testing.T) {
	for _, ok := range []string{"P1", "SIM-PILE-01", " CP_0001 ", "ab"} {
		if _, err := NormalizeCode(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "P", "P 1", "桩1", "p.1", "!"} {
		if _, err := NormalizeCode(bad); status(err) != 400 {
			t.Errorf("%q should be 400", bad)
		}
	}
}

func TestCheckManualStatus(t *testing.T) {
	if status(CheckManualStatus(Disabled)) != 200 || status(CheckManualStatus(Offline)) != 200 {
		t.Error("disabled/offline should be allowed")
	}
	for _, s := range []string{Available, Charging, Faulted, "", "bogus"} {
		if status(CheckManualStatus(s)) != 400 {
			t.Errorf("%q should be 400", s)
		}
	}
}
