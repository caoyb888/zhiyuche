import type { BadgeColor } from './ui/Badge'
import type { SelectOption } from './ui/Select'

/** 后端 notify 包的通知类型（apps/api/internal/notify/notify.go） */
export const NOTIFICATION_TYPES: Array<{ value: string; label: string; color: BadgeColor }> = [
  { value: 'approval.pending', label: '待审批', color: 'amber' },
  { value: 'approval.approved', label: '审批通过', color: 'green' },
  { value: 'approval.rejected', label: '审批驳回', color: 'red' },
  { value: 'approval.cancelled', label: '申请取消', color: 'gray' },
  { value: 'trip.started', label: '行程开始', color: 'blue' },
  { value: 'trip.ended', label: '行程结束', color: 'blue' },
  { value: 'trip.event', label: '行程异常', color: 'red' },
  { value: 'system', label: '系统', color: 'purple' },
]

export const NOTIFICATION_TYPE_OPTIONS: SelectOption[] = NOTIFICATION_TYPES.map((t) => ({ value: t.value, label: t.label }))

export function notificationTypeMeta(type: string): { label: string; color: BadgeColor } {
  const hit = NOTIFICATION_TYPES.find((t) => t.value === type)
  return hit ?? { label: type || '通知', color: 'gray' }
}
