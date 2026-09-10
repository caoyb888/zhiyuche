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

/**
 * 深色底图上的标记颜色：空闲绿 / 在途电光青 / 充电琥珀 / 维保红 / 停用灰。
 * 在途改用电光青而非品牌蓝——深蓝底图上蓝色标记辨不出来，青色同时承担「实时」语义。
 */
export const VEHICLE_STATUS_COLOR: Record<VehicleStatus, string> = {
  idle: '#10b981',
  in_use: '#3ccfe0',
  charging: '#f0a32b',
  maintenance: '#ef4b3c',
  disabled: '#7089a8',
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
  return soc > 60 ? 'bg-ev-500' : soc > 25 ? 'bg-warn-500' : 'bg-danger-500'
}

export function socTextClass(soc: number): string {
  return soc > 60 ? 'text-ev-200' : soc > 25 ? 'text-warn-200' : 'text-danger-200'
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
