// Package perm is the code-defined permission registry.
//
// Two kinds of entries:
//   - menu:   navigation nodes (code like "system.user", with route path and icon)
//   - action: fine-grained permission codes "module:resource:action"
//
// Menu visibility is derived by convention: a leaf menu "a.b" is visible when the
// user holds "a:b:view"; a parent menu is visible when any child is. Roles are
// therefore only ever assigned action codes.
package perm

import "strings"

type Kind string

const (
	Menu   Kind = "menu"
	Action Kind = "action"
)

type Def struct {
	Code   string
	Name   string
	Type   Kind
	Parent string // parent code ("" for root)
	Path   string // frontend route (menus only)
	Icon   string // lucide icon name (menus only)
	Sort   int
}

// Defs is the single source of truth. Order within a parent = display order.
var Defs = []Def{
	// ---- 业务菜单（Demo 页面，行为权限从阶段 2 起细化）----
	{Code: "dashboard", Name: "总览", Type: Menu, Path: "/", Icon: "LayoutDashboard", Sort: 10},
	{Code: "dashboard:view", Name: "查看总览", Type: Action, Parent: "dashboard"},

	{Code: "approval", Name: "公务审批", Type: Menu, Path: "/approval", Icon: "FileText", Sort: 20},
	{Code: "approval:view", Name: "查看审批", Type: Action, Parent: "approval"},

	{Code: "trip", Name: "行程管理", Type: Menu, Path: "/trips", Icon: "Map", Sort: 30},
	{Code: "trip:view", Name: "查看行程", Type: Action, Parent: "trip"},

	{Code: "report", Name: "费用报表", Type: Menu, Path: "/reports", Icon: "BarChart2", Sort: 40},
	{Code: "report:view", Name: "查看报表", Type: Action, Parent: "report"},

	{Code: "health", Name: "车辆健康", Type: Menu, Path: "/health", Icon: "HeartPulse", Sort: 50},
	{Code: "health:view", Name: "查看车辆健康", Type: Action, Parent: "health"},

	{Code: "charging", Name: "充电管理", Type: Menu, Path: "/charging", Icon: "Zap", Sort: 60},
	{Code: "charging:view", Name: "查看充电", Type: Action, Parent: "charging"},

	// ---- 系统管理 ----
	{Code: "system", Name: "系统管理", Type: Menu, Path: "/system", Icon: "Settings", Sort: 900},

	{Code: "system.user", Name: "用户管理", Type: Menu, Parent: "system", Path: "/system/users", Icon: "Users", Sort: 10},
	{Code: "system:user:view", Name: "查看用户", Type: Action, Parent: "system.user"},
	{Code: "system:user:create", Name: "新建用户", Type: Action, Parent: "system.user"},
	{Code: "system:user:update", Name: "编辑用户", Type: Action, Parent: "system.user"},
	{Code: "system:user:delete", Name: "删除用户", Type: Action, Parent: "system.user"},
	{Code: "system:user:reset-password", Name: "重置密码", Type: Action, Parent: "system.user"},
	{Code: "system:user:assign-roles", Name: "分配角色", Type: Action, Parent: "system.user"},
	{Code: "system:user:import", Name: "导入用户", Type: Action, Parent: "system.user"},
	{Code: "system:user:export", Name: "导出用户", Type: Action, Parent: "system.user"},

	{Code: "system.dept", Name: "部门管理", Type: Menu, Parent: "system", Path: "/system/depts", Icon: "Network", Sort: 20},
	{Code: "system:dept:view", Name: "查看部门", Type: Action, Parent: "system.dept"},
	{Code: "system:dept:create", Name: "新建部门", Type: Action, Parent: "system.dept"},
	{Code: "system:dept:update", Name: "编辑部门", Type: Action, Parent: "system.dept"},
	{Code: "system:dept:delete", Name: "删除部门", Type: Action, Parent: "system.dept"},

	{Code: "system.role", Name: "角色管理", Type: Menu, Parent: "system", Path: "/system/roles", Icon: "ShieldCheck", Sort: 30},
	{Code: "system:role:view", Name: "查看角色", Type: Action, Parent: "system.role"},
	{Code: "system:role:create", Name: "新建角色", Type: Action, Parent: "system.role"},
	{Code: "system:role:update", Name: "编辑角色", Type: Action, Parent: "system.role"},
	{Code: "system:role:delete", Name: "删除角色", Type: Action, Parent: "system.role"},
	{Code: "system:role:assign-perms", Name: "分配权限", Type: Action, Parent: "system.role"},

	{Code: "system.permission", Name: "菜单与权限", Type: Menu, Parent: "system", Path: "/system/permissions", Icon: "KeyRound", Sort: 40},
	{Code: "system:permission:view", Name: "查看权限点", Type: Action, Parent: "system.permission"},

	{Code: "system.dict", Name: "字典管理", Type: Menu, Parent: "system", Path: "/system/dicts", Icon: "BookOpen", Sort: 50},
	{Code: "system:dict:view", Name: "查看字典", Type: Action, Parent: "system.dict"},
	{Code: "system:dict:create", Name: "新建字典", Type: Action, Parent: "system.dict"},
	{Code: "system:dict:update", Name: "编辑字典", Type: Action, Parent: "system.dict"},
	{Code: "system:dict:delete", Name: "删除字典", Type: Action, Parent: "system.dict"},

	{Code: "system.param", Name: "参数设置", Type: Menu, Parent: "system", Path: "/system/params", Icon: "SlidersHorizontal", Sort: 60},
	{Code: "system:param:view", Name: "查看参数", Type: Action, Parent: "system.param"},
	{Code: "system:param:update", Name: "修改参数", Type: Action, Parent: "system.param"},

	{Code: "system.template", Name: "通知模板", Type: Menu, Parent: "system", Path: "/system/templates", Icon: "MessageSquare", Sort: 70},
	{Code: "system:template:view", Name: "查看通知模板", Type: Action, Parent: "system.template"},
	{Code: "system:template:create", Name: "新建通知模板", Type: Action, Parent: "system.template"},
	{Code: "system:template:update", Name: "编辑通知模板", Type: Action, Parent: "system.template"},
	{Code: "system:template:delete", Name: "删除通知模板", Type: Action, Parent: "system.template"},

	{Code: "system.audit", Name: "审计日志", Type: Menu, Parent: "system", Path: "/system/audit-logs", Icon: "ScrollText", Sort: 80},
	{Code: "system:audit:view", Name: "查看审计日志", Type: Action, Parent: "system.audit"},

	{Code: "system.tenant", Name: "租户管理", Type: Menu, Parent: "system", Path: "/system/tenants", Icon: "Building2", Sort: 90},
	{Code: "system:tenant:view", Name: "查看租户", Type: Action, Parent: "system.tenant"},
	{Code: "system:tenant:create", Name: "新建租户", Type: Action, Parent: "system.tenant"},
	{Code: "system:tenant:update", Name: "编辑租户", Type: Action, Parent: "system.tenant"},
	{Code: "system:tenant:delete", Name: "停用租户", Type: Action, Parent: "system.tenant"},
}

// PlatformOnly lists action codes only the platform tenant may hold.
var PlatformOnly = map[string]bool{
	"system:tenant:view":   true,
	"system:tenant:create": true,
	"system:tenant:update": true,
	"system:tenant:delete": true,
}

// ActionCodes returns every action code, optionally excluding platform-only ones.
func ActionCodes(includePlatform bool) []string {
	var out []string
	for _, d := range Defs {
		if d.Type != Action {
			continue
		}
		if !includePlatform && PlatformOnly[d.Code] {
			continue
		}
		out = append(out, d.Code)
	}
	return out
}

// IsAction reports whether code is a registered action.
func IsAction(code string) bool {
	for _, d := range Defs {
		if d.Code == code && d.Type == Action {
			return true
		}
	}
	return false
}

// MenuRequires returns the action code that makes a leaf menu visible ("a.b" -> "a:b:view").
func MenuRequires(menuCode string) string {
	return strings.ReplaceAll(menuCode, ".", ":") + ":view"
}

// MenuNode is the tree shape returned to the frontend.
type MenuNode struct {
	Code     string      `json:"code"`
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Icon     string      `json:"icon,omitempty"`
	Children []*MenuNode `json:"children,omitempty"`
}

// MenusFor builds the visible menu tree for a permission set (nil set = everything).
func MenusFor(has func(code string) bool) []*MenuNode {
	byParent := map[string][]Def{}
	for _, d := range Defs {
		if d.Type == Menu {
			byParent[d.Parent] = append(byParent[d.Parent], d)
		}
	}
	var build func(parent string) []*MenuNode
	build = func(parent string) []*MenuNode {
		var nodes []*MenuNode
		for _, d := range byParent[parent] {
			children := build(d.Code)
			isLeaf := len(byParent[d.Code]) == 0
			visible := (isLeaf && (has == nil || has(MenuRequires(d.Code)))) || (!isLeaf && len(children) > 0)
			if !visible {
				continue
			}
			nodes = append(nodes, &MenuNode{Code: d.Code, Name: d.Name, Path: d.Path, Icon: d.Icon, Children: children})
		}
		return nodes
	}
	return build("")
}

// DefaultRole describes a built-in role seeded for every tenant.
type DefaultRole struct {
	Code        string
	Name        string
	Description string
	Perms       []string // nil = all non-platform actions
}

// DefaultRoles are created for each new tenant (idempotently).
var DefaultRoles = []DefaultRole{
	{Code: "tenant_admin", Name: "租户管理员", Description: "拥有本租户全部权限"},
	{Code: "fleet_manager", Name: "车队管理员", Description: "车辆调度、行程、健康、充电管理",
		Perms: []string{"dashboard:view", "approval:view", "trip:view", "health:view", "charging:view", "report:view",
			"system:dict:view", "system:param:view", "system:user:view", "system:dept:view"}},
	{Code: "approver", Name: "审批人", Description: "审批公务用车申请",
		Perms: []string{"dashboard:view", "approval:view", "trip:view"}},
	{Code: "finance", Name: "财务", Description: "费用报表、账单与充电费用",
		Perms: []string{"dashboard:view", "report:view", "charging:view", "trip:view"}},
	{Code: "employee", Name: "员工", Description: "申请用车、查看自己的行程",
		Perms: []string{"dashboard:view"}},
}

// SuperAdminRole is the platform-level role (tenant_id NULL); its holders are also users.is_super.
var SuperAdminRole = DefaultRole{Code: "super_admin", Name: "超级管理员", Description: "平台全部权限，跨租户"}
