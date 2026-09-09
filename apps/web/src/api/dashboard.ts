import { get } from './client'
import type { DashboardOverview } from './types'

export const dashboardKeys = {
  all: ['dashboard'] as const,
  overview: ['dashboard', 'overview'] as const,
}

/** GET /dashboard/overview —— 车辆状态计数、待办审批、今日行程汇总、设备在线、最近异常事件 */
export const getDashboardOverview = (): Promise<DashboardOverview> => get<DashboardOverview>('/dashboard/overview')
