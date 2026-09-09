import type { AccountLevel, AccountStatus, BillingLineKind, BillingPenaltyType, SettlementLineKind, SettlementStatus, TransactionType } from '../../api/types'
import type { BadgeColor } from '../../components/ui/Badge'
import type { SelectOption } from '../../components/ui/Select'
import { EMPTY, formatMoney } from '../../utils/format'

// ── 账户级别 ──────────────────────────────────────────
export const ACCOUNT_LEVEL_LABEL: Record<AccountLevel, string> = {
  enterprise: '企业',
  department: '部门',
  employee: '员工',
}

export const ACCOUNT_LEVEL_BADGE: Record<AccountLevel, BadgeColor> = {
  enterprise: 'purple',
  department: 'blue',
  employee: 'gray',
}

export const ACCOUNT_LEVEL_OPTIONS: SelectOption[] = (Object.keys(ACCOUNT_LEVEL_LABEL) as AccountLevel[]).map((l) => ({ value: l, label: ACCOUNT_LEVEL_LABEL[l] }))

export function isAccountLevel(v: string): v is AccountLevel {
  return v in ACCOUNT_LEVEL_LABEL
}

/** 下一级账户级别（企业 → 部门 → 员工；员工无下级） */
export function childLevel(level: AccountLevel): AccountLevel | null {
  if (level === 'enterprise') return 'department'
  if (level === 'department') return 'employee'
  return null
}

export const ACCOUNT_STATUS_LABEL: Record<AccountStatus, string> = {
  active: '正常',
  frozen: '已冻结',
}

export const ACCOUNT_STATUS_BADGE: Record<AccountStatus, BadgeColor> = {
  active: 'green',
  frozen: 'red',
}

// ── 流水类型 ──────────────────────────────────────────
export const TRANSACTION_TYPE_LABEL: Record<TransactionType, string> = {
  recharge: '充值',
  allocate_in: '划入',
  allocate_out: '划出',
  trip: '行程扣费',
  charge: '充电扣费',
  penalty: '罚金',
  refund: '退款',
  adjust: '调整',
}

export const TRANSACTION_TYPE_BADGE: Record<TransactionType, BadgeColor> = {
  recharge: 'green',
  allocate_in: 'green',
  allocate_out: 'amber',
  trip: 'blue',
  charge: 'blue',
  penalty: 'red',
  refund: 'green',
  adjust: 'gray',
}

export const TRANSACTION_TYPE_OPTIONS: SelectOption[] = (Object.keys(TRANSACTION_TYPE_LABEL) as TransactionType[]).map((t) => ({ value: t, label: TRANSACTION_TYPE_LABEL[t] }))

export function isTransactionType(v: string): v is TransactionType {
  return v in TRANSACTION_TYPE_LABEL
}

/** 关联单据链接：行程 → /trips?id=，充电事务 → /charging?tx= */
export function refLink(refType: string | null | undefined, refId: string | null | undefined): string | null {
  if (!refType || !refId) return null
  if (refType === 'trip') return `/trips?id=${encodeURIComponent(refId)}`
  if (refType === 'charge_transaction' || refType === 'charge') return `/charging?tx=${encodeURIComponent(refId)}`
  return null
}

// ── 结算 ─────────────────────────────────────────────
export const SETTLEMENT_STATUS_LABEL: Record<SettlementStatus, string> = {
  draft: '草稿',
  confirmed: '已确认',
}

export const SETTLEMENT_STATUS_BADGE: Record<SettlementStatus, BadgeColor> = {
  draft: 'amber',
  confirmed: 'green',
}

export const PERIOD_STATUS_LABEL: Record<'none' | SettlementStatus, string> = {
  none: '未生成',
  draft: '草稿',
  confirmed: '已确认',
}

export const SETTLEMENT_LINE_KIND_LABEL: Record<SettlementLineKind, string> = {
  trip: '行程',
  charge: '充电',
  penalty: '罚金',
}

export const SETTLEMENT_LINE_KIND_BADGE: Record<SettlementLineKind, BadgeColor> = {
  trip: 'blue',
  charge: 'green',
  penalty: 'red',
}

/** 明细行数量的单位：行程 km、充电 kWh */
export function lineQuantityUnit(kind: SettlementLineKind): string {
  return kind === 'trip' ? 'km' : kind === 'charge' ? 'kWh' : ''
}

// ── 计费规则 ──────────────────────────────────────────
export const PENALTY_TYPE_LABEL: Record<BillingPenaltyType, string> = {
  overspeed: '超速',
  not_charging_on_return: '低电未充电还车',
  harsh_driving: '急加减速',
  late_return: '超时还车',
}

export const PENALTY_TYPE_OPTIONS: SelectOption[] = (Object.keys(PENALTY_TYPE_LABEL) as BillingPenaltyType[]).map((t) => ({ value: t, label: PENALTY_TYPE_LABEL[t] }))

export const LINE_KIND_LABEL: Record<BillingLineKind, string> = {
  base: '基础费',
  multiplier: '时段系数',
  cap: '日封顶',
  surcharge: '附加费',
  electricity: '电费',
  penalty: '罚金',
}

export const LINE_KIND_BADGE: Record<BillingLineKind, BadgeColor> = {
  base: 'gray',
  multiplier: 'blue',
  cap: 'green',
  surcharge: 'amber',
  electricity: 'purple',
  penalty: 'red',
}

// ── 金额展示 ──────────────────────────────────────────

/** 带符号金额文本：+1,234.00 / -56.00 */
export function formatSignedMoney(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return EMPTY
  const abs = formatMoney(Math.abs(value))
  return value > 0 ? `+${abs}` : value < 0 ? `-${abs}` : abs
}

/** 有符号金额的颜色：正绿 / 负红 / 零灰 */
export function signedMoneyClass(value: number | null | undefined): string {
  if (value === null || value === undefined) return 'text-slate-400'
  return value > 0 ? 'text-emerald-600' : value < 0 ? 'text-red-600' : 'text-slate-500'
}

/** 余额颜色：负数红 */
export function balanceClass(value: number): string {
  return value < 0 ? 'text-red-600' : 'text-slate-800'
}

/** 预算使用率 0–100+（预算为 0 时返回 null） */
export function budgetUsage(spent: number | null | undefined, budget: number | null | undefined): number | null {
  if (!budget || budget <= 0) return null
  return Math.max(0, ((spent ?? 0) / budget) * 100)
}

/** 进度条颜色：< 80% 绿、< 100% 琥珀、超支红 */
export function usageBarClass(pct: number): string {
  return pct >= 100 ? 'bg-red-500' : pct >= 80 ? 'bg-amber-500' : 'bg-emerald-500'
}
