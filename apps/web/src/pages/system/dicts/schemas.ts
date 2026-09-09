import { z } from 'zod'
import type { DictItemStatus } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 与契约 DictTypeCreate.code 一致 */
export const DICT_CODE_RULE = /^[a-z][a-z0-9_]{1,63}$/

const INT_RULE = /^-?\d+$/

export const dictItemRowSchema = z.object({
  label: z.string().trim().min(1, '请输入显示名').max(64, '显示名最多 64 个字符'),
  value: z.string().trim().min(1, '请输入值').max(64, '值最多 64 个字符'),
  sort: z.string().trim().regex(INT_RULE, '排序须为整数'),
})

export const dictTypeFormSchema = z.object({
  code: z.string().trim().regex(DICT_CODE_RULE, '2–64 位小写字母、数字或下划线，且以字母开头'),
  name: z.string().trim().min(1, '请输入字典名称').max(64, '名称最多 64 个字符'),
  description: z.string().trim().max(255, '描述最多 255 个字符'),
  items: z.array(dictItemRowSchema).superRefine((items, ctx) => {
    const seen = new Map<string, number>()
    items.forEach((it, i) => {
      const v = it.value.trim()
      if (!v) return
      const prev = seen.get(v)
      if (prev !== undefined) ctx.addIssue({ code: 'custom', path: [i, 'value'], message: `与第 ${prev + 1} 行的值重复` })
      else seen.set(v, i)
    })
  }),
})

export type DictTypeFormValues = z.infer<typeof dictTypeFormSchema>

/** 解析 extra 文本域：空串视为 {}；必须是 JSON 对象 */
export function parseExtra(text: string): Record<string, unknown> | null {
  const t = text.trim()
  if (!t) return {}
  try {
    const v: unknown = JSON.parse(t)
    if (typeof v !== 'object' || v === null || Array.isArray(v)) return null
    return v as Record<string, unknown>
  } catch {
    return null
  }
}

export const dictItemFormSchema = z.object({
  label: z.string().trim().min(1, '请输入显示名').max(64, '显示名最多 64 个字符'),
  value: z.string().trim().min(1, '请输入值').max(64, '值最多 64 个字符'),
  sort: z.string().trim().regex(INT_RULE, '排序须为整数'),
  color: z.string().trim().max(32, '颜色最多 32 个字符'),
  status: z.enum(['active', 'disabled']),
  extra: z.string().refine((v) => parseExtra(v) !== null, '须为合法的 JSON 对象，如 {"seats": 5}'),
})

export type DictItemFormValues = z.infer<typeof dictItemFormSchema>

export const dictItemStatusLabel: Record<DictItemStatus, string> = { active: '启用', disabled: '停用' }
export const dictItemStatusColor: Record<DictItemStatus, BadgeColor> = { active: 'green', disabled: 'gray' }
export const dictItemStatusOptions: SelectOption[] = [
  { value: 'active', label: '启用' },
  { value: 'disabled', label: '停用' },
]
