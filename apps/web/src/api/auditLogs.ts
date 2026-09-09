import { compactParams, get } from './client'
import type { AuditLog, Page, PageQuery } from './types'

/** GET /system/audit-logs 的筛选条件（不含分页）；from / to 为 RFC3339 */
export interface AuditLogFilter {
  user_id?: string
  username?: string
  module?: string
  action?: string
  target_id?: string
  /** summary / path 模糊 */
  keyword?: string
  from?: string
  to?: string
}

export type AuditLogListParams = AuditLogFilter & PageQuery

export const auditLogKeys = {
  all: ['audit-logs'] as const,
  list: (params: AuditLogListParams) => ['audit-logs', 'list', params] as const,
  detail: (id: number) => ['audit-logs', 'detail', id] as const,
}

/** 列表项不含 before / after */
export const listAuditLogs = (params: AuditLogListParams): Promise<Page<AuditLog>> =>
  get<Page<AuditLog>>('/system/audit-logs', { params: compactParams(params) })

/** 详情含 before / after */
export const getAuditLog = (id: number): Promise<AuditLog> => get<AuditLog>(`/system/audit-logs/${id}`)
