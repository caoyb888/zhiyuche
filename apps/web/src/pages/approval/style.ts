import type { Approval, ApprovalStatus, ApprovalStepAction, ApprovalUrgency, TripType, VehicleBrief } from '../../api/types'
import { formatKm, formatPercent } from '../../components/map/vehicleStyle'
import type { BadgeColor } from '../../components/ui/Badge'
import type { SelectOption } from '../../components/ui/Select'

/** 可用车辆下拉项：车牌 · 型号 · SOC · 续航 */
export function vehicleOptionLabel(v: VehicleBrief): string {
  const parts = [v.plate_no]
  if (v.model) parts.push(v.model)
  parts.push(`SOC ${formatPercent(v.soc)}`)
  parts.push(`续航 ${formatKm(v.range_km)}`)
  return parts.join(' · ')
}

export const APPROVAL_STATUS_LABEL: Record<ApprovalStatus, string> = {
  pending_l1: '待一级审批',
  pending_l2: '待二级审批',
  approved: '已批准',
  rejected: '已驳回',
  cancelled: '已撤销',
  in_use: '用车中',
  completed: '已完成',
  expired: '已过期',
}

/** 8 种状态配色：待审批琥珀 / 批准绿 / 驳回红 / 撤销灰 / 用车中蓝 / 完成紫 / 过期灰 */
export const APPROVAL_STATUS_BADGE: Record<ApprovalStatus, BadgeColor> = {
  pending_l1: 'amber',
  pending_l2: 'amber',
  approved: 'green',
  rejected: 'red',
  cancelled: 'gray',
  in_use: 'blue',
  completed: 'purple',
  expired: 'gray',
}

export const APPROVAL_STATUS_OPTIONS: SelectOption[] = (Object.keys(APPROVAL_STATUS_LABEL) as ApprovalStatus[]).map((s) => ({ value: s, label: APPROVAL_STATUS_LABEL[s] }))

export function isApprovalStatus(v: string): v is ApprovalStatus {
  return v in APPROVAL_STATUS_LABEL
}

export function isPendingStatus(s: ApprovalStatus): boolean {
  return s === 'pending_l1' || s === 'pending_l2'
}

export const TRIP_TYPE_LABEL: Record<TripType, string> = {
  official: '公务用车',
  daily: '日常用车',
}

export const TRIP_TYPE_BADGE: Record<TripType, BadgeColor> = {
  official: 'blue',
  daily: 'gray',
}

export const TRIP_TYPE_OPTIONS: SelectOption[] = (Object.keys(TRIP_TYPE_LABEL) as TripType[]).map((t) => ({ value: t, label: TRIP_TYPE_LABEL[t] }))

export function isTripType(v: string): v is TripType {
  return v in TRIP_TYPE_LABEL
}

export const URGENCY_LABEL: Record<ApprovalUrgency, string> = {
  normal: '普通',
  urgent: '紧急',
}

export const STEP_ACTION_LABEL: Record<ApprovalStepAction, string> = {
  pending: '待处理',
  approved: '已通过',
  rejected: '已驳回',
  skipped: '已跳过',
}

export const STEP_ACTION_BADGE: Record<ApprovalStepAction, BadgeColor> = {
  pending: 'amber',
  approved: 'green',
  rejected: 'red',
  skipped: 'gray',
}

/** 当前待处理步骤的审批人姓名（非待审批状态返回 null） */
export function currentApproverName(a: Approval): string | null {
  if (!isPendingStatus(a.status)) return null
  const step = a.steps.find((s) => s.step_no === a.current_step) ?? a.steps.find((s) => s.action === 'pending')
  return step?.approver.name ?? null
}

/** 事由：字典 label 优先，回退到 code */
export function purposeText(a: Pick<Approval, 'purpose_code' | 'purpose_label'>): string {
  return a.purpose_label || a.purpose_code
}
