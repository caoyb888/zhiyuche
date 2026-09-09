import { z } from 'zod'
import type { PileManualStatus, PileStatus, PileType } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 与契约 PileCreate.pile_code 一致 */
export const PILE_CODE_RULE = /^[A-Za-z0-9_-]{2,64}$/

const coordSchema = z
  .object({ lng: z.number(), lat: z.number(), address: z.string().optional() })
  .nullable()

function numField(min: number, message: string) {
  return z.string().trim().refine((v) => v === '' || (Number.isFinite(Number(v)) && Number(v) >= min), message)
}

export const pileFormSchema = z.object({
  pile_code: z.string().trim().regex(PILE_CODE_RULE, '编号为 2–64 位字母、数字、_ 或 -'),
  name: z.string().trim().min(1, '请输入名称').max(64, '最多 64 个字符'),
  type: z.enum(['fast', 'slow']),
  power_kw: numField(0.1, '功率须为正数（kW）'),
  connector_count: z.string().trim().refine((v) => v === '' || (/^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 20), '枪数为 1–20 的整数'),
  vendor: z.string().trim().max(64, '最多 64 个字符'),
  location: z.string().trim().max(200, '最多 200 个字符'),
  coord: coordSchema,
  /** 编辑时可手动设为 disabled / offline；'' 表示不改 */
  status: z.enum(['', 'offline', 'disabled']),
  remark: z.string().trim().max(500, '备注最多 500 个字符'),
})

export type PileFormValues = z.infer<typeof pileFormSchema>

export const pileTypeLabel: Record<PileType, string> = {
  fast: '快充',
  slow: '慢充',
}

export const pileTypeOptions: SelectOption[] = [
  { value: 'fast', label: '快充' },
  { value: 'slow', label: '慢充' },
]

export function isPileType(v: string): v is PileType {
  return v === 'fast' || v === 'slow'
}

export const pileStatusLabel: Record<PileStatus, string> = {
  available: '空闲',
  charging: '充电中',
  offline: '离线',
  faulted: '故障',
  disabled: '停用',
}

export const pileStatusColor: Record<PileStatus, BadgeColor> = {
  available: 'green',
  charging: 'amber',
  offline: 'gray',
  faulted: 'red',
  disabled: 'gray',
}

/** 地图标记颜色 */
export const pileStatusHex: Record<PileStatus, string> = {
  available: '#10b981',
  charging: '#f59e0b',
  offline: '#94a3b8',
  faulted: '#ef4444',
  disabled: '#cbd5e1',
}

export const pileStatusOptions: SelectOption[] = (Object.keys(pileStatusLabel) as PileStatus[]).map((s) => ({ value: s, label: pileStatusLabel[s] }))

export function isPileStatus(v: string): v is PileStatus {
  return v in pileStatusLabel
}

/** 手动可设置的状态（契约：只能设 disabled 或恢复 offline，其余由 OCPP 上报） */
export const pileManualStatusOptions: SelectOption[] = [
  { value: 'offline', label: '恢复（离线，等待桩上线）' },
  { value: 'disabled', label: '停用' },
]

export function isPileManualStatus(v: string): v is PileManualStatus {
  return v === 'offline' || v === 'disabled'
}

export function numberOrUndefined(v: string): number | undefined {
  const t = v.trim()
  if (t === '') return undefined
  const n = Number(t)
  return Number.isFinite(n) ? n : undefined
}
