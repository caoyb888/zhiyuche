import { z } from 'zod'
import type { NotifyChannel } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 与契约 TemplateCreate.code 一致 */
export const TEMPLATE_CODE_RULE = /^[a-z][a-z0-9_.]{1,63}$/

export const templateFormSchema = z.object({
  code: z.string().trim().regex(TEMPLATE_CODE_RULE, '2–64 位小写字母、数字、下划线或点，且以字母开头'),
  channel: z.enum(['inapp', 'wechat', 'sms']),
  title: z.string().trim().min(1, '请输入标题').max(128, '标题最多 128 个字符'),
  content: z.string().trim().min(1, '请输入内容').max(4000, '内容最多 4000 个字符'),
  enabled: z.boolean(),
})

export type TemplateFormValues = z.infer<typeof templateFormSchema>

export const channelLabel: Record<NotifyChannel, string> = {
  inapp: '站内信',
  wechat: '微信',
  sms: '短信',
}

export const channelColor: Record<NotifyChannel, BadgeColor> = {
  inapp: 'blue',
  wechat: 'green',
  sms: 'amber',
}

export const channelOptions: SelectOption[] = (Object.keys(channelLabel) as NotifyChannel[]).map((c) => ({ value: c, label: channelLabel[c] }))

export function isNotifyChannel(v: string): v is NotifyChannel {
  return v === 'inapp' || v === 'wechat' || v === 'sms'
}

/** 提取内容中的 {{变量}} 名（去重，保持出现顺序） */
export function extractVariables(content: string): string[] {
  const out: string[] = []
  for (const m of content.matchAll(/\{\{\s*([A-Za-z_][\w.]*)\s*\}\}/g)) {
    if (!out.includes(m[1])) out.push(m[1])
  }
  return out
}
