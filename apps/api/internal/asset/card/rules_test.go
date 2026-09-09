package card

import (
	"errors"
	"testing"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func TestNormalizeUID(t *testing.T) {
	cases := map[string]string{
		"abcdef12":                         "ABCDEF12",
		"  A1B2C3D0 ":                      "A1B2C3D0",
		"0123456789abcdef0123456789ABCDEF": "0123456789ABCDEF0123456789ABCDEF",
	}
	for in, want := range cases {
		got, err := NormalizeUID(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q err %v, want %q", in, got, err, want)
		}
	}
	bad := []string{"", "ABC", "ABCDEF1", "GHIJKLMN", "abcd-ef12", "0123456789abcdef0123456789ABCDEF0"}
	for _, in := range bad {
		_, err := NormalizeUID(in)
		var ae *httpx.AppError
		if !errors.As(err, &ae) || ae.Status != 400 {
			t.Errorf("%q should be 400, got %v", in, err)
		}
	}
}
