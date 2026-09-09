import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 后端 audit.Record 使用的模块名（与各 handler 的 module 常量一致） */
export const auditModuleOptions: SelectOption[] = [
  { value: 'auth', label: '认证 auth' },
  { value: 'system.user', label: '用户 system.user' },
  { value: 'system.dept', label: '部门 system.dept' },
  { value: 'system.role', label: '角色 system.role' },
  { value: 'system.tenant', label: '租户 system.tenant' },
  { value: 'system.dict', label: '字典 system.dict' },
  { value: 'system.param', label: '参数 system.param' },
  { value: 'system.template', label: '通知模板 system.template' },
]

const moduleLabel = new Map(auditModuleOptions.map((o) => [o.value, o.label.replace(/\s.*$/, '')]))

export function auditModuleLabel(module: string): string {
  return moduleLabel.get(module) ?? module
}

export function statusColor(status: number): BadgeColor {
  if (status >= 500) return 'red'
  if (status >= 400) return 'amber'
  if (status >= 200 && status < 300) return 'green'
  return 'gray'
}

export const methodColor: Record<string, BadgeColor> = {
  GET: 'gray',
  POST: 'blue',
  PUT: 'amber',
  PATCH: 'amber',
  DELETE: 'red',
}
