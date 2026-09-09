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
	{Code: "approval:view", Name: "查看审批（本人相关）", Type: Action, Parent: "approval"},
	{Code: "approval:create", Name: "发起用车申请", Type: Action, Parent: "approval"},
	{Code: "approval:approve", Name: "审批（处理指派给我的步骤）", Type: Action, Parent: "approval"},
	{Code: "approval:manage", Name: "管理全部申请（查看/取消/代审批/指派车辆）", Type: Action, Parent: "approval"},
	{Code: "approval:rule", Name: "配置审批规则", Type: Action, Parent: "approval"},

	{Code: "booking", Name: "预约派车", Type: Menu, Path: "/bookings", Icon: "PhoneCall", Sort: 25},
	{Code: "booking:view", Name: "查看预约", Type: Action, Parent: "booking"},
	{Code: "booking:create", Name: "新建预约（电话 / 直接）", Type: Action, Parent: "booking"},
	{Code: "booking:update", Name: "编辑预约（含改派车辆、出车、完成）", Type: Action, Parent: "booking"},
	{Code: "booking:cancel", Name: "取消预约", Type: Action, Parent: "booking"},

	{Code: "trip", Name: "行程管理", Type: Menu, Path: "/trips", Icon: "Map", Sort: 30},
	{Code: "trip:view", Name: "查看行程（本人相关）", Type: Action, Parent: "trip"},
	{Code: "trip:manage", Name: "管理全部行程（查看全部/手动开始结束/取消）", Type: Action, Parent: "trip"},
	{Code: "trip:export", Name: "导出行程", Type: Action, Parent: "trip"},

	{Code: "report", Name: "费用报表", Type: Menu, Path: "/reports", Icon: "BarChart2", Sort: 40},
	{Code: "report:view", Name: "查看报表", Type: Action, Parent: "report"},

	{Code: "health", Name: "车辆健康", Type: Menu, Path: "/health", Icon: "HeartPulse", Sort: 50},
	{Code: "health:view", Name: "查看车辆健康", Type: Action, Parent: "health"},

	{Code: "charging", Name: "充电管理", Type: Menu, Path: "/charging", Icon: "Zap", Sort: 60},
	{Code: "charging:view", Name: "查看充电（桩状态、充电记录）", Type: Action, Parent: "charging"},
	{Code: "charging:manage", Name: "远程启停充电", Type: Action, Parent: "charging"},
	{Code: "charging:review", Name: "复核充电事务（归属/计量偏差）", Type: Action, Parent: "charging"},
	{Code: "charging:export", Name: "导出充电记录", Type: Action, Parent: "charging"},

	// ---- 计费账户 ----
	{Code: "billing", Name: "计费账户", Type: Menu, Path: "/billing", Icon: "Wallet", Sort: 65},

	{Code: "billing.rule", Name: "计费规则", Type: Menu, Parent: "billing", Path: "/billing/rules", Icon: "Calculator", Sort: 10},
	{Code: "billing:rule:view", Name: "查看计费规则", Type: Action, Parent: "billing.rule"},
	{Code: "billing:rule:update", Name: "编辑/启用计费规则", Type: Action, Parent: "billing.rule"},

	{Code: "billing.account", Name: "账户管理", Type: Menu, Parent: "billing", Path: "/billing/accounts", Icon: "Wallet", Sort: 20},
	{Code: "billing:account:view", Name: "查看账户与流水", Type: Action, Parent: "billing.account"},
	{Code: "billing:account:recharge", Name: "企业账户充值", Type: Action, Parent: "billing.account"},
	{Code: "billing:account:allocate", Name: "额度划拨（企业→部门→员工）", Type: Action, Parent: "billing.account"},
	{Code: "billing:account:adjust", Name: "余额调整/退款", Type: Action, Parent: "billing.account"},

	{Code: "billing.settlement", Name: "月度结算", Type: Menu, Parent: "billing", Path: "/billing/settlements", Icon: "Receipt", Sort: 30},
	{Code: "billing:settlement:view", Name: "查看结算单", Type: Action, Parent: "billing.settlement"},
	{Code: "billing:settlement:generate", Name: "生成/重算结算单", Type: Action, Parent: "billing.settlement"},
	{Code: "billing:settlement:confirm", Name: "确认结算单", Type: Action, Parent: "billing.settlement"},
	{Code: "billing:settlement:export", Name: "导出结算单", Type: Action, Parent: "billing.settlement"},

	// ---- 资产管理 ----
	{Code: "asset", Name: "资产管理", Type: Menu, Path: "/assets", Icon: "Car", Sort: 70},

	{Code: "asset.vehicle", Name: "车辆档案", Type: Menu, Parent: "asset", Path: "/assets/vehicles", Icon: "Car", Sort: 10},
	{Code: "asset:vehicle:view", Name: "查看车辆", Type: Action, Parent: "asset.vehicle"},
	{Code: "asset:vehicle:create", Name: "新建车辆", Type: Action, Parent: "asset.vehicle"},
	{Code: "asset:vehicle:update", Name: "编辑车辆（含置维保/停用）", Type: Action, Parent: "asset.vehicle"},
	{Code: "asset:vehicle:delete", Name: "删除车辆", Type: Action, Parent: "asset.vehicle"},

	{Code: "asset.device", Name: "网关设备", Type: Menu, Parent: "asset", Path: "/assets/devices", Icon: "Cpu", Sort: 20},
	{Code: "asset:device:view", Name: "查看设备", Type: Action, Parent: "asset.device"},
	{Code: "asset:device:create", Name: "新建设备", Type: Action, Parent: "asset.device"},
	{Code: "asset:device:update", Name: "编辑设备（含绑定/解绑/换密钥）", Type: Action, Parent: "asset.device"},
	{Code: "asset:device:delete", Name: "删除设备", Type: Action, Parent: "asset.device"},

	{Code: "asset.card", Name: "NFC 卡", Type: Menu, Parent: "asset", Path: "/assets/cards", Icon: "CreditCard", Sort: 30},
	{Code: "asset:card:view", Name: "查看卡", Type: Action, Parent: "asset.card"},
	{Code: "asset:card:create", Name: "发卡", Type: Action, Parent: "asset.card"},
	{Code: "asset:card:update", Name: "编辑卡（含绑定/解绑/挂失）", Type: Action, Parent: "asset.card"},
	{Code: "asset:card:delete", Name: "删除卡", Type: Action, Parent: "asset.card"},

	{Code: "asset.pile", Name: "充电桩档案", Type: Menu, Parent: "asset", Path: "/assets/piles", Icon: "PlugZap", Sort: 40},
	{Code: "asset:pile:view", Name: "查看充电桩", Type: Action, Parent: "asset.pile"},
	{Code: "asset:pile:create", Name: "新建充电桩", Type: Action, Parent: "asset.pile"},
	{Code: "asset:pile:update", Name: "编辑充电桩", Type: Action, Parent: "asset.pile"},
	{Code: "asset:pile:delete", Name: "删除充电桩", Type: Action, Parent: "asset.pile"},

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
		Perms: []string{"dashboard:view", "approval:view", "approval:manage", "approval:rule",
			"booking:view", "booking:create", "booking:update", "booking:cancel",
			"trip:view", "trip:manage", "trip:export", "health:view", "charging:view", "report:view",
			"asset:vehicle:view", "asset:vehicle:create", "asset:vehicle:update", "asset:vehicle:delete",
			"asset:device:view", "asset:device:create", "asset:device:update", "asset:device:delete",
			"asset:card:view", "asset:card:create", "asset:card:update", "asset:card:delete",
			"asset:pile:view", "asset:pile:create", "asset:pile:update", "asset:pile:delete",
			"charging:manage", "charging:review", "billing:account:view", "billing:settlement:view", "billing:rule:view",
			"system:dict:view", "system:param:view", "system:user:view", "system:dept:view"}},
	{Code: "approver", Name: "审批人", Description: "审批公务用车申请",
		Perms: []string{"dashboard:view", "approval:view", "approval:approve", "approval:create", "trip:view", "asset:vehicle:view"}},
	{Code: "finance", Name: "财务", Description: "费用报表、账单与充电费用",
		Perms: []string{"dashboard:view", "report:view", "charging:view", "charging:review", "charging:export", "trip:view", "trip:export",
			"billing:rule:view", "billing:rule:update",
			"billing:account:view", "billing:account:recharge", "billing:account:allocate", "billing:account:adjust",
			"billing:settlement:view", "billing:settlement:generate", "billing:settlement:confirm", "billing:settlement:export",
			"system:dept:view", "system:user:view"}},
	{Code: "employee", Name: "员工", Description: "申请用车、查看自己的行程",
		Perms: []string{"dashboard:view", "approval:view", "approval:create", "trip:view"}},
}

// SuperAdminRole is the platform-level role (tenant_id NULL); its holders are also users.is_super.
var SuperAdminRole = DefaultRole{Code: "super_admin", Name: "超级管理员", Description: "平台全部权限，跨租户"}
