import type { VehicleStatus } from '../../api/types'
import type { BadgeColor } from '../ui/Badge'
import type { SelectOption } from '../ui/Select'

export const VEHICLE_STATUS_LABEL: Record<VehicleStatus, string> = {
  idle: '空闲',
  in_use: '在途',
  charging: '充电中',
  maintenance: '维保中',
  disabled: '停用',
}

/** 地图标记颜色：空闲绿 / 在途蓝 / 充电琥珀 / 维保红 / 停用灰 */
export const VEHICLE_STATUS_COLOR: Record<VehicleStatus, string> = {
  idle: '#10b981',
  in_use: '#2563eb',
  charging: '#f59e0b',
  maintenance: '#ef4444',
  disabled: '#94a3b8',
}

export const VEHICLE_STATUS_BADGE: Record<VehicleStatus, BadgeColor> = {
  idle: 'green',
  in_use: 'blue',
  charging: 'amber',
  maintenance: 'red',
  disabled: 'gray',
}

export const VEHICLE_STATUS_OPTIONS: SelectOption[] = (Object.keys(VEHICLE_STATUS_LABEL) as VehicleStatus[]).map((s) => ({ value: s, label: VEHICLE_STATUS_LABEL[s] }))

export function isVehicleStatus(v: string): v is VehicleStatus {
  return v in VEHICLE_STATUS_LABEL
}

/** 离线车辆标记透明度 */
export const OFFLINE_OPACITY = 0.45

export function socBarClass(soc: number): string {
  return soc > 60 ? 'bg-emerald-400' : soc > 25 ? 'bg-amber-400' : 'bg-red-400'
}

export function socTextClass(soc: number): string {
  return soc > 60 ? 'text-emerald-600' : soc > 25 ? 'text-amber-500' : 'text-red-500'
}

export function formatPercent(v: number | null | undefined, digits = 0): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  return `${v.toFixed(digits)}%`
}

export function formatKm(v: number | null | undefined, digits = 0): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  return `${v.toLocaleString('zh-CN', { minimumFractionDigits: digits, maximumFractionDigits: digits })} km`
}

export function formatSpeed(v: number | null | undefined): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  return `${Math.round(v)} km/h`
}
