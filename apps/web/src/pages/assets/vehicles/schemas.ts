import { z } from 'zod'
import type { VehicleManualStatus, VehicleStatus } from '../../../api/types'
import type { SelectOption } from '../../../components/ui/Select'

const DATE_RULE = /^\d{4}-\d{2}-\d{2}$/
/** VIN：17 位，不含 I/O/Q */
export const VIN_RULE = /^[A-HJ-NPR-Z0-9]{17}$/i

const optionalDate = z.string().trim().refine((v) => v === '' || DATE_RULE.test(v), '日期格式为 YYYY-MM-DD')

function intField(min: number, max: number, message: string) {
  return z.string().trim().refine((v) => v === '' || (/^\d+$/.test(v) && Number(v) >= min && Number(v) <= max), message)
}

function numField(min: number, message: string) {
  return z.string().trim().refine((v) => v === '' || (Number.isFinite(Number(v)) && Number(v) >= min), message)
}

export const vehicleFormSchema = z.object({
  plate_no: z.string().trim().min(2, '车牌 2–16 个字符').max(16, '车牌 2–16 个字符'),
  vin: z.string().trim().refine((v) => v === '' || VIN_RULE.test(v), 'VIN 须为 17 位字母数字（不含 I/O/Q）'),
  brand: z.string().trim().max(64, '最多 64 个字符'),
  model: z.string().trim().max(64, '最多 64 个字符'),
  color: z.string().trim().max(32, '最多 32 个字符'),
  seat_count: intField(1, 60, '座位数为 1–60 的整数'),
  battery_kwh: numField(1, '电池容量须 ≥ 1 kWh'),
  range_km_full: intField(1, 5000, '满电续航为正整数（km）'),
  odometer_km: numField(0, '里程须 ≥ 0'),
  purchase_date: optionalDate,
  insurance_expire: optionalDate,
  inspection_expire: optionalDate,
  home_dept_id: z.string().nullable(),
  status: z.enum(['idle', 'maintenance', 'disabled']),
  remark: z.string().trim().max(500, '备注最多 500 个字符'),
})

export type VehicleFormValues = z.infer<typeof vehicleFormSchema>

export const VEHICLE_MANUAL_STATUS_LABEL: Record<VehicleManualStatus, string> = {
  idle: '空闲',
  maintenance: '维保中',
  disabled: '停用',
}

export const vehicleManualStatusOptions: SelectOption[] = (Object.keys(VEHICLE_MANUAL_STATUS_LABEL) as VehicleManualStatus[]).map((s) => ({ value: s, label: VEHICLE_MANUAL_STATUS_LABEL[s] }))

export function isManualStatus(s: VehicleStatus): s is VehicleManualStatus {
  return s === 'idle' || s === 'maintenance' || s === 'disabled'
}

/** 在途 / 充电中的车辆不可手动改状态（契约：updateVehicle 409） */
export function isStatusLocked(s: VehicleStatus): boolean {
  return s === 'in_use' || s === 'charging'
}

export const onlineOptions: SelectOption[] = [
  { value: 'true', label: '在线' },
  { value: 'false', label: '离线' },
]

/** '' | 'true' | 'false' → boolean | undefined */
export function parseBool(v: string): boolean | undefined {
  return v === 'true' ? true : v === 'false' ? false : undefined
}

export function numberOrUndefined(v: string): number | undefined {
  const t = v.trim()
  if (t === '') return undefined
  const n = Number(t)
  return Number.isFinite(n) ? n : undefined
}
