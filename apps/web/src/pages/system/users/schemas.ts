import { z } from 'zod'
import type { UserStatus } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 密码规则：至少 8 位，含字母和数字（与后端 auth.ValidatePassword 一致） */
export const PASSWORD_RULE = /^(?=.*[A-Za-z])(?=.*\d).{8,}$/
export const PHONE_RULE = /^\d{11}$/

/** 可留空的密码（留空 = 使用系统缺省密码） */
export const optionalPassword = z
  .string()
  .max(72, '密码最多 72 位')
  .refine((v) => v === '' || PASSWORD_RULE.test(v), '至少 8 位，且包含字母和数字')

export const userFormSchema = z.object({
  username: z.string().trim().min(2, '用户名 2–64 个字符').max(64, '用户名 2–64 个字符'),
  name: z.string().trim().min(1, '请输入姓名').max(64, '姓名最多 64 个字符'),
  password: optionalPassword,
  phone: z
    .string()
    .trim()
    .refine((v) => v === '' || PHONE_RULE.test(v), '手机号须为 11 位数字'),
  email: z
    .string()
    .trim()
    .refine((v) => v === '' || z.string().email().safeParse(v).success, '邮箱格式不正确'),
  employee_no: z.string().trim().max(64, '工号最多 64 个字符'),
  dept_id: z.string().nullable(),
  role_ids: z.array(z.string()),
  status: z.enum(['active', 'disabled', 'locked']),
})

export type UserFormValues = z.infer<typeof userFormSchema>

export const userStatusLabel: Record<UserStatus, string> = {
  active: '正常',
  disabled: '停用',
  locked: '锁定',
}

export const userStatusColor: Record<UserStatus, BadgeColor> = {
  active: 'green',
  disabled: 'gray',
  locked: 'red',
}

/** 筛选 / 表单用状态选项（锁定由后端产生，表单中仅在已锁定用户上显示） */
export const userStatusOptions: SelectOption[] = [
  { value: 'active', label: '正常' },
  { value: 'disabled', label: '停用' },
  { value: 'locked', label: '锁定' },
]

export function isUserStatus(v: string): v is UserStatus {
  return v === 'active' || v === 'disabled' || v === 'locked'
}

/** 两个 id 集合是否相同（忽略顺序） */
export function sameIdSet(a: readonly string[], b: readonly string[]): boolean {
  if (a.length !== b.length) return false
  const s = new Set(a)
  return b.every((x) => s.has(x))
}
