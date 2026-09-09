package tenant

import (
	"errors"
	"testing"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func TestValidateCode(t *testing.T) {
	for _, ok := range []string{"ab", "chenhua", "t-01", "a1", "x-012345678901234567890123456789"} { // 32 位上限
		if err := ValidateCode(ok); err != nil {
			t.Errorf("%q should be accepted: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a", "-ab", "1ab", "Chenhua", "has_underscore", "has space", "中文", "x-01234567890123456789012345678901"} {
		if err := ValidateCode(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize(CreateRequest{Code: "t1", Name: "T1", AdminUsername: "t1_admin"})
	if got.AdminName != "管理员" {
		t.Fatalf("admin_name default = %q", got.AdminName)
	}
	got = Normalize(CreateRequest{AdminName: "老板"})
	if got.AdminName != "老板" {
		t.Fatalf("explicit admin_name must be kept, got %q", got.AdminName)
	}
}

func TestStatusTransition(t *testing.T) {
	str := func(s string) *string { return &s }

	if tr, err := StatusTransition(StatusActive, nil, false); err != nil || tr != NoChange {
		t.Fatalf("nil status: %v %v", tr, err)
	}
	if tr, err := StatusTransition(StatusActive, str(StatusActive), false); err != nil || tr != NoChange {
		t.Fatalf("same status: %v %v", tr, err)
	}
	if tr, err := StatusTransition(StatusActive, str(StatusDisabled), false); err != nil || tr != Disable {
		t.Fatalf("active→disabled: %v %v", tr, err)
	}
	if tr, err := StatusTransition(StatusDisabled, str(StatusActive), false); err != nil || tr != Reactivate {
		t.Fatalf("disabled→active: %v %v", tr, err)
	}
	// 已停用再停用：无事发生
	if tr, err := StatusTransition(StatusDisabled, str(StatusDisabled), false); err != nil || tr != NoChange {
		t.Fatalf("disabled→disabled: %v %v", tr, err)
	}
	// 平台租户不可停用
	_, err := StatusTransition(StatusActive, str(StatusDisabled), true)
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Code != httpx.CodeBadRequest {
		t.Fatalf("platform disable must be a 40000 error, got %v", err)
	}
}
