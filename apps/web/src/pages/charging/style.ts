import dayjs from 'dayjs'
import type { AccountLevel, ChargeBindMethod, ChargeReviewStatus, ChargeStatus, ChargeTransaction, ConnectorStatus, PileStatus } from '../../api/types'
import type { BadgeColor } from '../../components/ui/Badge'
import type { SelectOption } from '../../components/ui/Select'

// ── 事务状态 ──────────────────────────────────────────
export const CHARGE_STATUS_LABEL: Record<ChargeStatus, string> = {
  charging: '充电中',
  ended: '已结束',
  settled: '已结算',
  cancelled: '已取消',
}

export const CHARGE_STATUS_BADGE: Record<ChargeStatus, BadgeColor> = {
  charging: 'amber',
  ended: 'blue',
  settled: 'green',
  cancelled: 'gray',
}

export const CHARGE_STATUS_OPTIONS: SelectOption[] = (Object.keys(CHARGE_STATUS_LABEL) as ChargeStatus[]).map((s) => ({ value: s, label: CHARGE_STATUS_LABEL[s] }))

export function isChargeStatus(v: string): v is ChargeStatus {
  return v in CHARGE_STATUS_LABEL
}

// ── 复核状态 ──────────────────────────────────────────
export const REVIEW_STATUS_LABEL: Record<ChargeReviewStatus, string> = {
  none: '无需复核',
  pending: '待复核',
  approved: '复核通过',
  rejected: '已拒绝',
}

export const REVIEW_STATUS_BADGE: Record<ChargeReviewStatus, BadgeColor> = {
  none: 'gray',
  pending: 'amber',
  approved: 'green',
  rejected: 'red',
}

export const REVIEW_STATUS_OPTIONS: SelectOption[] = (Object.keys(REVIEW_STATUS_LABEL) as ChargeReviewStatus[]).map((s) => ({ value: s, label: REVIEW_STATUS_LABEL[s] }))

export function isReviewStatus(v: string): v is ChargeReviewStatus {
  return v in REVIEW_STATUS_LABEL
}

// ── 连接器（OCPP 1.6 StatusNotification）────────────────
export interface ConnectorStyle {
  label: string
  badge: BadgeColor
  /** 地图 / 小圆点颜色 */
  hex: string
  /** 充电中：呼吸点 */
  pulse?: boolean
  /** 可远程启动 */
  startable?: boolean
}

export const CONNECTOR_STYLE: Record<ConnectorStatus, ConnectorStyle> = {
  Available: { label: '空闲', badge: 'green', hex: '#10b981', startable: true },
  Preparing: { label: '已插枪', badge: 'blue', hex: '#3b82f6', startable: true },
  Charging: { label: '充电中', badge: 'amber', hex: '#f59e0b', pulse: true },
  SuspendedEV: { label: '车端暂停', badge: 'gray', hex: '#94a3b8' },
  SuspendedEVSE: { label: '桩端暂停', badge: 'gray', hex: '#94a3b8' },
  Finishing: { label: '结束中', badge: 'blue', hex: '#3b82f6' },
  Reserved: { label: '已预约', badge: 'purple', hex: '#8b5cf6' },
  Unavailable: { label: '不可用', badge: 'gray', hex: '#cbd5e1' },
  Faulted: { label: '故障', badge: 'red', hex: '#ef4444' },
}

export function connectorStyle(status: string): ConnectorStyle {
  return (CONNECTOR_STYLE as Record<string, ConnectorStyle>)[status] ?? { label: status, badge: 'gray', hex: '#94a3b8' }
}

// ── 桩状态（与桩档案页一致的配色；离线优先）────────────
export const PILE_STATUS_LABEL: Record<PileStatus, string> = {
  available: '空闲',
  charging: '充电中',
  offline: '离线',
  faulted: '故障',
  disabled: '停用',
}

export const PILE_STATUS_BADGE: Record<PileStatus, BadgeColor> = {
  available: 'green',
  charging: 'amber',
  offline: 'gray',
  faulted: 'red',
  disabled: 'gray',
}

export const PILE_STATUS_HEX: Record<PileStatus, string> = {
  available: '#10b981',
  charging: '#f59e0b',
  offline: '#94a3b8',
  faulted: '#ef4444',
  disabled: '#cbd5e1',
}

/** 卡片底色：故障红、充电中琥珀、离线 / 停用灰、空闲绿 */
export const PILE_CARD_CLASS: Record<PileStatus, string> = {
  available: 'border-emerald-100 bg-emerald-50/40',
  charging: 'border-amber-100 bg-amber-50/40',
  offline: 'border-slate-200 bg-slate-50',
  faulted: 'border-red-100 bg-red-50/50',
  disabled: 'border-slate-200 bg-slate-50',
}

// ── 归属 / 绑定方式 ────────────────────────────────────
export const BIND_METHOD_LABEL: Record<ChargeBindMethod, string> = {
  card: '刷卡匹配',
  location: '桩位匹配',
  recent_trip: '最近行程',
  manual: '人工指定',
  none: '未绑定',
  null: '未绑定',
}

export function bindMethodLabel(v: string | null | undefined): string {
  if (!v) return '未绑定'
  return (BIND_METHOD_LABEL as Record<string, string>)[v] ?? v
}

export const ATTRIBUTION_LABEL: Record<AccountLevel, string> = {
  enterprise: '企业账户',
  department: '部门账户',
  employee: '员工账户',
}

export function attributionLabel(v: string | null | undefined): string | null {
  if (!v) return null
  return (ATTRIBUTION_LABEL as Record<string, string>)[v] ?? v
}

// ── 交叉校验 ──────────────────────────────────────────
/** 桩侧 kWh 与 BMS 估算偏差超过该百分比进入人工复核 */
export const DEVIATION_THRESHOLD_PCT = 5

export function deviationExceeded(pct: number | null | undefined): boolean {
  return typeof pct === 'number' && Number.isFinite(pct) && pct > DEVIATION_THRESHOLD_PCT
}

export type ReviewReasonKind = 'unbound' | 'deviation' | 'pricing' | 'other'

export interface ReviewReason {
  kind: ReviewReasonKind
  label: string
}

/** 待复核原因（契约无显式字段，按事务数据推断）：未绑定车辆 → 偏差 → 计价失败 */
export function reviewReason(t: Pick<ChargeTransaction, 'vehicle' | 'user' | 'deviation_pct' | 'cost' | 'review_status' | 'status'>): ReviewReason {
  if (!t.vehicle) return { kind: 'unbound', label: '未绑定车辆' }
  if (deviationExceeded(t.deviation_pct)) return { kind: 'deviation', label: `偏差 ${formatPct(t.deviation_pct)}` }
  if (t.review_status === 'pending' && t.status !== 'charging' && (t.cost === null || t.cost === undefined)) return { kind: 'pricing', label: '计价失败' }
  return { kind: 'other', label: '待确认' }
}

export function formatPct(v: number | null | undefined, digits = 1): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  return `${v.toFixed(digits)}%`
}

export function formatKwh(v: number | null | undefined, digits = 2): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  return `${v.toFixed(digits)} kWh`
}

export function formatKw(v: number | null | undefined, digits = 1): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  return `${v.toFixed(digits)} kW`
}

/** 事务时长（分钟）：已结束取 duration_min，进行中按 start_at 到现在 */
export function txDurationMin(t: Pick<ChargeTransaction, 'start_at' | 'end_at' | 'duration_min' | 'status'>, now = Date.now()): number | null {
  if (typeof t.duration_min === 'number') return t.duration_min
  const s = dayjs(t.start_at)
  if (!s.isValid()) return null
  const e = t.end_at ? dayjs(t.end_at) : t.status === 'charging' ? dayjs(now) : null
  if (!e || !e.isValid()) return null
  return Math.max(0, e.diff(s, 'minute', true))
}

/** 是否可远程停止：进行中的事务 */
export function isStoppable(t: Pick<ChargeTransaction, 'status'> | null | undefined): boolean {
  return t?.status === 'charging'
}

/** 桩实时视图轮询间隔（WebSocket charging.updated 到达时也会失效重取） */
export const PILES_LIVE_INTERVAL_MS = 30_000

/** 桩类型 */
export const PILE_TYPE_LABEL: Record<'fast' | 'slow', string> = { fast: '快充', slow: '慢充' }
