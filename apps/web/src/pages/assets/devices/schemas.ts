import { z } from 'zod'
import type { DeviceStatus } from '../../../api/types'
import type { BadgeColor } from '../../../components/ui/Badge'
import type { SelectOption } from '../../../components/ui/Select'

/** 与契约 DeviceCreate.serial_no 一致 */
export const SERIAL_RULE = /^[A-Za-z0-9_-]{4,64}$/
export const DEFAULT_DEVICE_MODEL = 'VIG-100E'

export const deviceCreateSchema = z.object({
  serial_no: z.string().trim().regex(SERIAL_RULE, '序列号为 4–64 位字母、数字、_ 或 -'),
  vehicle_id: z.string(),
  model: z.string().trim().min(1, '请输入型号').max(64, '最多 64 个字符'),
  firmware: z.string().trim().max(64, '最多 64 个字符'),
  iccid: z.string().trim().max(32, '最多 32 个字符'),
  remark: z.string().trim().max(500, '备注最多 500 个字符'),
})

export type DeviceCreateValues = z.infer<typeof deviceCreateSchema>

export const deviceEditSchema = z.object({
  model: z.string().trim().min(1, '请输入型号').max(64, '最多 64 个字符'),
  firmware: z.string().trim().max(64, '最多 64 个字符'),
  iccid: z.string().trim().max(32, '最多 32 个字符'),
  status: z.enum(['active', 'disabled']),
  remark: z.string().trim().max(500, '备注最多 500 个字符'),
})

export type DeviceEditValues = z.infer<typeof deviceEditSchema>

export const deviceStatusLabel: Record<DeviceStatus, string> = {
  active: '正常',
  disabled: '停用',
}

export const deviceStatusColor: Record<DeviceStatus, BadgeColor> = {
  active: 'green',
  disabled: 'gray',
}

export const deviceStatusOptions: SelectOption[] = [
  { value: 'active', label: '正常' },
  { value: 'disabled', label: '停用（拒绝上报）' },
]

export function isDeviceStatus(v: string): v is DeviceStatus {
  return v === 'active' || v === 'disabled'
}

export const boundOptions: SelectOption[] = [
  { value: 'true', label: '已绑定' },
  { value: 'false', label: '未绑定' },
]

export const onlineOptions: SelectOption[] = [
  { value: 'true', label: '在线' },
  { value: 'false', label: '离线' },
]

export function parseBool(v: string): boolean | undefined {
  return v === 'true' ? true : v === 'false' ? false : undefined
}
