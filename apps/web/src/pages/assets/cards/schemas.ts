import { z } from 'zod'
import type { CardStatus } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 与契约 CardCreate.card_uid 一致：8–32 位十六进制 */
export const CARD_UID_RULE = /^[A-Fa-f0-9]{8,32}$/

/** UserPicker 选中的持卡人 */
export const pickedUserSchema = z.object({
  id: z.string(),
  name: z.string(),
  username: z.string().optional(),
  dept_name: z.string().nullable().optional(),
})

export const cardIssueSchema = z.object({
  card_uid: z.string().trim().regex(CARD_UID_RULE, '卡片 UID 为 8–32 位十六进制（0-9 A-F）'),
  holder: pickedUserSchema.nullable(),
  /** 本地 datetime-local 值，空 = 由后端取当前时间 */
  issued_at: z.string(),
  remark: z.string().trim().max(500, '备注最多 500 个字符'),
})

export type CardIssueValues = z.infer<typeof cardIssueSchema>

export const cardEditSchema = z.object({
  status: z.enum(['active', 'lost', 'disabled']),
  remark: z.string().trim().max(500, '备注最多 500 个字符'),
})

export type CardEditValues = z.infer<typeof cardEditSchema>

export const cardStatusLabel: Record<CardStatus, string> = {
  active: '正常',
  lost: '已挂失',
  disabled: '停用',
}

export const cardStatusColor: Record<CardStatus, BadgeColor> = {
  active: 'green',
  lost: 'red',
  disabled: 'gray',
}

export const cardStatusOptions: SelectOption[] = (Object.keys(cardStatusLabel) as CardStatus[]).map((s) => ({ value: s, label: cardStatusLabel[s] }))

export function isCardStatus(v: string): v is CardStatus {
  return v === 'active' || v === 'lost' || v === 'disabled'
}

export const boundOptions: SelectOption[] = [
  { value: 'true', label: '已绑定持卡人' },
  { value: 'false', label: '未绑定' },
]

export function parseBool(v: string): boolean | undefined {
  return v === 'true' ? true : v === 'false' ? false : undefined
}
