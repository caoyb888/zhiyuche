import { compactParams, get, put } from './client'
import type { Notification, Page } from './types'

export interface NotificationListParams {
  page?: number
  pageSize?: number
  /** 只看未读 */
  unread?: boolean
  type?: string
}

export const notificationKeys = {
  all: ['notifications'] as const,
  list: (params: NotificationListParams) => ['notifications', 'list', params] as const,
  unread: ['notifications', 'unread'] as const,
}

/** GET /notifications —— 我的通知（按时间倒序） */
export const listNotifications = (params: NotificationListParams): Promise<Page<Notification>> =>
  get<Page<Notification>>('/notifications', { params: compactParams(params) })

export interface UnreadCount {
  unread: number
}

export const getUnreadCount = (): Promise<UnreadCount> => get<UnreadCount>('/notifications/unread-count')

export const markNotificationsRead = (ids: string[]): Promise<unknown> => put<unknown>('/notifications/read', { ids })

export const markAllNotificationsRead = (): Promise<unknown> => put<unknown>('/notifications/read-all')

/** 通知 ref_type → 跳转地址；无法映射时返回 null */
export function notificationHref(n: Pick<Notification, 'ref_type' | 'ref_id'>): string | null {
  if (!n.ref_id) return null
  const id = encodeURIComponent(n.ref_id)
  switch (n.ref_type) {
    case 'approval':
      return `/approval?id=${id}`
    case 'trip':
      return `/trips?id=${id}`
    case 'vehicle':
      return `/assets/vehicles?id=${id}`
    default:
      return null
  }
}
