package role

import (
	"errors"
	"testing"

	"github.com/caoyb888/zhiyuche/apps/api/internal/perm"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func TestValidateCode(t *testing.T) {
	for _, ok := range []string{"ab", "fleet_manager", "a1", "r_012345678901234567890123456789"} { // 32 位上限
		if err := ValidateCode(ok); err != nil {
			t.Errorf("%q should be accepted: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a", "A_b", "1abc", "has-dash", "has space", "中文", "a12345678901234567890123456789012"} {
		if err := ValidateCode(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func badRequest(t *testing.T, err error) {
	t.Helper()
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Code != httpx.CodeBadRequest {
		t.Fatalf("expected 40000 AppError, got %v", err)
	}
}

func TestValidatePermissions(t *testing.T) {
	got, err := ValidatePermissions([]string{"system:user:view", "dashboard:view", "system:user:view"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "dashboard:view" || got[1] != "system:user:view" {
		t.Fatalf("expected de-duplicated sorted list, got %v", got)
	}
	if got, err := ValidatePermissions(nil, false); err != nil || len(got) != 0 {
		t.Fatalf("empty set must be valid, got %v %v", got, err)
	}

	_, err = ValidatePermissions([]string{"system:user:nope"}, true)
	badRequest(t, err)
	_, err = ValidatePermissions([]string{"system.user"}, true) // 菜单码不是动作
	badRequest(t, err)

	// 平台专属：非平台租户拒绝，平台租户接受
	_, err = ValidatePermissions([]string{"system:tenant:view"}, false)
	badRequest(t, err)
	if _, err := ValidatePermissions([]string{"system:tenant:view"}, true); err != nil {
		t.Fatalf("platform tenant must accept platform-only codes: %v", err)
	}
}

func collect(nodes []*PermissionNode, into map[string]*PermissionNode) {
	for _, n := range nodes {
		into[n.Code] = n
		collect(n.Children, into)
	}
}

func TestBuildPermissionTree(t *testing.T) {
	full := map[string]*PermissionNode{}
	collect(BuildPermissionTree(true), full)
	limited := map[string]*PermissionNode{}
	collect(BuildPermissionTree(false), limited)

	// 平台树包含注册表里的每个 def；非平台树去掉平台专属动作及因此变空的菜单
	for _, d := range perm.Defs {
		n, ok := full[d.Code]
		if !ok {
			t.Fatalf("platform tree misses %q", d.Code)
		}
		if n.Type != string(d.Type) {
			t.Fatalf("%q type = %q, want %q", d.Code, n.Type, d.Type)
		}
		if d.Type == perm.Action && len(n.Children) != 0 {
			t.Fatalf("action %q must be a leaf", d.Code)
		}
		if d.Type == perm.Action {
			if _, ok := limited[d.Code]; ok == perm.PlatformOnly[d.Code] {
				t.Fatalf("non-platform tree: %q present=%v, platformOnly=%v", d.Code, ok, perm.PlatformOnly[d.Code])
			}
		}
	}
	if _, ok := limited["system.tenant"]; ok {
		t.Fatal("system.tenant menu must disappear for non-platform tenants (no actions left)")
	}
	sys := full["system"]
	if sys == nil || sys.Path != "/system" || len(sys.Children) == 0 || sys.Children[0].Code != "system.user" {
		t.Fatalf("system menu shape wrong: %+v", sys)
	}
	user := full["system.user"]
	if user.Children[0].Code != "system:user:view" || user.Children[0].Type != "action" {
		t.Fatalf("first child of system.user should be the view action, got %+v", user.Children[0])
	}
}
