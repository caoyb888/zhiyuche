import { lazy, type ComponentType } from 'react'

/** 受保护页面的路由声明：path 与后端菜单 path 一致，perm 为访问所需权限码 */
export interface AppRoute {
  path: string
  perm: string
  title: string
  description?: string
  component: ComponentType
}

export const appRoutes: AppRoute[] = [
  // ---- 业务（Demo，mock 数据）----
  { path: '/', perm: 'dashboard:view', title: '总览', description: '实时车辆状态', component: lazy(() => import('../pages/Dashboard')) },
  // ---- 公务审批 / 行程（真实接口）----
  { path: '/approval', perm: 'approval:view', title: '公务审批', description: '申请与审批流', component: lazy(() => import('../pages/approval')) },
  { path: '/approval/rules', perm: 'approval:rule', title: '审批规则', description: '二级审批触发条件与审批人', component: lazy(() => import('../pages/approval/rules')) },
  { path: '/trips', perm: 'trip:view', title: '行程管理', description: '轨迹与记录', component: lazy(() => import('../pages/trips')) },
  { path: '/charging', perm: 'charging:view', title: '充电管理', description: '桩实时状态、充电记录与复核', component: lazy(() => import('../pages/charging')) },
  { path: '/reports', perm: 'report:view', title: '费用报表', description: '部门费用分析', component: lazy(() => import('../pages/Reports')) },
  { path: '/health', perm: 'health:view', title: '车辆健康', description: 'AI 诊断', component: lazy(() => import('../pages/Health')) },
  // ---- 资产管理（path 与 registry.go 的菜单 Path 一致）----
  { path: '/assets/vehicles', perm: 'asset:vehicle:view', title: '车辆档案', description: '车辆基础信息与实时状态', component: lazy(() => import('../pages/assets/vehicles')) },
  { path: '/assets/devices', perm: 'asset:device:view', title: '网关设备', description: '车载网关与接入密钥', component: lazy(() => import('../pages/assets/devices')) },
  { path: '/assets/cards', perm: 'asset:card:view', title: 'NFC 卡', description: '刷卡取车用卡片', component: lazy(() => import('../pages/assets/cards')) },
  { path: '/assets/piles', perm: 'asset:pile:view', title: '充电桩档案', description: '桩位、功率与状态', component: lazy(() => import('../pages/assets/piles')) },
  // ---- 个人（无需权限码）----
  { path: '/notifications', perm: '', title: '通知中心', description: '站内通知', component: lazy(() => import('../pages/notifications')) },
  // ---- 系统管理（path 与 apps/api/internal/perm/registry.go 的菜单 Path 一致）----
  { path: '/system/users', perm: 'system:user:view', title: '用户管理', description: '账号、角色与部门归属', component: lazy(() => import('../pages/system/users')) },
  { path: '/system/depts', perm: 'system:dept:view', title: '部门管理', description: '组织架构与预算', component: lazy(() => import('../pages/system/depts')) },
  { path: '/system/roles', perm: 'system:role:view', title: '角色管理', description: '角色与权限分配', component: lazy(() => import('../pages/system/roles')) },
  { path: '/system/permissions', perm: 'system:permission:view', title: '菜单与权限', description: '权限点注册表（只读）', component: lazy(() => import('../pages/system/permissions')) },
  { path: '/system/dicts', perm: 'system:dict:view', title: '字典管理', description: '字典类型与条目', component: lazy(() => import('../pages/system/dicts')) },
  { path: '/system/params', perm: 'system:param:view', title: '参数设置', description: '系统参数与租户覆盖', component: lazy(() => import('../pages/system/params')) },
  { path: '/system/templates', perm: 'system:template:view', title: '通知模板', description: '站内信 / 微信 / 短信模板', component: lazy(() => import('../pages/system/templates')) },
  { path: '/system/audit-logs', perm: 'system:audit:view', title: '审计日志', description: '操作记录与变更数据', component: lazy(() => import('../pages/system/audit-logs')) },
  { path: '/system/tenants', perm: 'system:tenant:view', title: '租户管理', description: '入驻企业（平台管理员）', component: lazy(() => import('../pages/system/tenants')) },
]

/** 按路径查找路由声明（用于顶栏标题回退） */
export function findAppRoute(pathname: string): AppRoute | undefined {
  return appRoutes.find((r) => r.path === pathname)
}
