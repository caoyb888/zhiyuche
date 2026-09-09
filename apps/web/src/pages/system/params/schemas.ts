import type { ParamSource, ParamValueType } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'

export const paramTypeLabel: Record<ParamValueType, string> = {
  string: '文本',
  int: '整数',
  float: '数字',
  bool: '布尔',
  json: 'JSON',
}

export const paramSourceLabel: Record<ParamSource, string> = {
  global: '全局',
  tenant: '租户',
}

export const paramSourceColor: Record<ParamSource, BadgeColor> = {
  global: 'purple',
  tenant: 'blue',
}
