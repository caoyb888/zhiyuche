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
  { path: '/approval', perm: 'approval:view', title: '公务审批', description: '申请与审批流', component: lazy(() => import('../pages/Approval')) },
  { path: '/trips', perm: 'trip:view', title: '行程管理', description: '轨迹与记录', component: lazy(() => import('../pages/Trips')) },
  { path: '/reports', perm: 'report:view', title: '费用报表', description: '部门费用分析', component: lazy(() => import('../pages/Reports')) },
  { path: '/health', perm: 'health:view', title: '车辆健康', description: 'AI 诊断', component: lazy(() => import('../pages/Health')) },
  { path: '/charging', perm: 'charging:view', title: '充电管理', description: '充电桩状态', component: lazy(() => import('../pages/Charging')) },
  // ---- 系统管理 ----
  { path: '/system/users', perm: 'system:user:view', title: '用户管理', description: '账号、角色与部门归属', component: lazy(() => import('../pages/system/users')) },
  { path: '/system/depts', perm: 'system:dept:view', title: '部门管理', description: '组织架构与预算', component: lazy(() => import('../pages/system/depts')) },
]

/** 按路径查找路由声明（用于顶栏标题回退） */
export function findAppRoute(pathname: string): AppRoute | undefined {
  return appRoutes.find((r) => r.path === pathname)
}
