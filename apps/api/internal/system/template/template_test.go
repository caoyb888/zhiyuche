package template

import "testing"

func TestValidCode(t *testing.T) {
	good := []string{"approval.submitted", "trip.started", "ab", "a1", "vehicle_low_soc", "x.y.z_1"}
	for _, c := range good {
		if !ValidCode(c) {
			t.Errorf("%q should be valid", c)
		}
	}
	long := "a"
	for len(long) < 65 {
		long += "b"
	}
	bad := []string{"", "a", "A.b", "1abc", ".abc", "abc-def", "abc def", "_abc", "中文", long}
	for _, c := range bad {
		if ValidCode(c) {
			t.Errorf("%q should be rejected", c)
		}
	}
	if !ValidCode(long[:64]) {
		t.Errorf("64 chars should be valid")
	}
}
