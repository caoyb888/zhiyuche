package dict

import (
	"errors"
	"testing"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func TestValidCode(t *testing.T) {
	good := []string{"ab", "vehicle_type", "a1", "x_y", "trip_status_2"}
	for _, c := range good {
		if !ValidCode(c) {
			t.Errorf("%q should be valid", c)
		}
	}
	long := "a"
	for len(long) < 65 {
		long += "b"
	}
	bad := []string{"", "a", "A", "1abc", "Abc", "abc-def", "abc.def", "abc def", "_abc", "中文", long}
	for _, c := range bad {
		if ValidCode(c) {
			t.Errorf("%q should be rejected", c)
		}
	}
	if !ValidCode(long[:64]) {
		t.Errorf("64 chars should be valid")
	}
}

func TestCheckItemValues(t *testing.T) {
	if err := checkItemValues([]ItemCreateRequest{{Value: "a"}, {Value: "b"}}); err != nil {
		t.Fatalf("distinct values: %v", err)
	}
	var ae *httpx.AppError
	if err := checkItemValues([]ItemCreateRequest{{Value: "a"}, {Value: "a"}}); !errors.As(err, &ae) || ae.Status != 400 {
		t.Fatalf("duplicate must be 400, got %v", err)
	}
	if err := checkItemValues([]ItemCreateRequest{{Value: "  "}}); !errors.As(err, &ae) || ae.Status != 400 {
		t.Fatalf("blank must be 400, got %v", err)
	}
}

func TestExtraJSON(t *testing.T) {
	b, err := extraJSON(nil)
	if err != nil || string(b) != "{}" {
		t.Fatalf("nil extra → {} , got %s %v", b, err)
	}
	b, err = extraJSON(map[string]any{"k": 1})
	if err != nil || string(b) != `{"k":1}` {
		t.Fatalf("got %s %v", b, err)
	}
}
