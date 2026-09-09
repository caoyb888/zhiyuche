import { z } from 'zod'
import type { TenantStatus } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 与契约 TenantCreate.code 一致 */
export const TENANT_CODE_RULE = /^[a-z][a-z0-9-]{1,31}$/
const PASSWORD_RULE = /^(?=.*[A-Za-z])(?=.*\d).{8,}$/

const common = {
  name: z.string().trim().min(1, '请输入租户名称').max(128, '名称最多 128 个字符'),
  license_no: z.string().trim().max(64, '执照号最多 64 个字符'),
  contact_name: z.string().trim().max(64, '联系人最多 64 个字符'),
  contact_phone: z.string().trim().max(32, '电话最多 32 个字符'),
  /** datetime-local 值，空串表示不设置 */
  expires_at: z.string(),
}

export const tenantCreateSchema = z.object({
  code: z.string().trim().regex(TENANT_CODE_RULE, '2–32 位小写字母、数字或短横线，且以字母开头'),
  ...common,
  admin_username: z.string().trim().min(2, '用户名 2–64 个字符').max(64, '用户名 2–64 个字符'),
  admin_password: z
    .string()
    .max(72, '密码最多 72 位')
    .refine((v) => v === '' || PASSWORD_RULE.test(v), '至少 8 位，且包含字母和数字'),
  admin_name: z.string().trim().max(64, '姓名最多 64 个字符'),
})

export const tenantEditSchema = z.object(common)

export type TenantCreateValues = z.infer<typeof tenantCreateSchema>
export type TenantEditValues = z.infer<typeof tenantEditSchema>

export const tenantStatusLabel: Record<TenantStatus, string> = { active: '正常', disabled: '停用' }
export const tenantStatusColor: Record<TenantStatus, BadgeColor> = { active: 'green', disabled: 'gray' }
export const tenantStatusOptions: SelectOption[] = [
  { value: 'active', label: '正常' },
  { value: 'disabled', label: '停用' },
]

export function isTenantStatus(v: string): v is TenantStatus {
  return v === 'active' || v === 'disabled'
}
