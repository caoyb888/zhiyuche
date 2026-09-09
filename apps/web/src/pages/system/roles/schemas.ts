import { z } from 'zod'

/** 与契约 RoleCreate.code 一致 */
export const ROLE_CODE_RULE = /^[a-z][a-z0-9_]{1,31}$/

export const roleFormSchema = z.object({
  code: z.string().trim().regex(ROLE_CODE_RULE, '2–32 位小写字母、数字或下划线，且以字母开头'),
  name: z.string().trim().min(1, '请输入角色名称').max(64, '名称最多 64 个字符'),
  description: z.string().trim().max(255, '描述最多 255 个字符'),
  permissions: z.array(z.string()),
})

export type RoleFormValues = z.infer<typeof roleFormSchema>
