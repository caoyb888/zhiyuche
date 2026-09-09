package role

import (
	"regexp"
	"sort"

	"github.com/caoyb888/zhiyuche/apps/api/internal/perm"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// ValidateCode enforces the role code pattern from the contract.
func ValidateCode(code string) error {
	if !codeRe.MatchString(code) {
		return httpx.BadRequest("角色代码须为 2-32 位小写字母、数字或下划线，且以字母开头")
	}
	return nil
}

// ValidatePermissions checks every code is a registered action and, for a
// non-platform tenant, none is platform-only. Returns the de-duplicated,
// sorted list on success.
func ValidatePermissions(codes []string, isPlatform bool) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if _, dup := seen[c]; dup {
			continue
		}
		if !perm.IsAction(c) {
			return nil, httpx.BadRequest("未知的权限码：" + c)
		}
		if !isPlatform && perm.PlatformOnly[c] {
			return nil, httpx.BadRequest("非平台租户不能包含平台专属权限：" + c)
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Strings(out)
	return out, nil
}

// BuildPermissionTree turns the registry into menu → (sub-menus, actions).
// Platform-only actions are dropped for non-platform tenants, and a menu that
// ends up with no children is dropped with them.
func BuildPermissionTree(includePlatform bool) []*PermissionNode {
	byParent := map[string][]perm.Def{}
	for _, d := range perm.Defs {
		byParent[d.Parent] = append(byParent[d.Parent], d)
	}
	var build func(parent string) []*PermissionNode
	build = func(parent string) []*PermissionNode {
		nodes := []*PermissionNode{}
		for _, d := range byParent[parent] {
			switch d.Type {
			case perm.Action:
				if !includePlatform && perm.PlatformOnly[d.Code] {
					continue
				}
				nodes = append(nodes, &PermissionNode{Code: d.Code, Name: d.Name, Type: string(perm.Action)})
			case perm.Menu:
				children := build(d.Code)
				if len(children) == 0 {
					continue
				}
				nodes = append(nodes, &PermissionNode{Code: d.Code, Name: d.Name, Type: string(perm.Menu), Path: d.Path, Icon: d.Icon, Children: children})
			}
		}
		return nodes
	}
	return build("")
}
