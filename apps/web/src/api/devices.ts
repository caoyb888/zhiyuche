import { compactParams, del, get, post, put } from './client'
import type { Device, DeviceCreate, DeviceStatus, DeviceUpdate, DeviceWithKey, Page, PageQuery } from './types'

/** GET /assets/devices 的筛选条件（不含分页） */
export interface DeviceFilter {
  /** 序列号 / ICCID / 车牌 */
  keyword?: string
  status?: DeviceStatus | ''
  /** 是否已绑定车辆 */
  bound?: boolean
  online?: boolean
}

export type DeviceListParams = DeviceFilter & PageQuery

export const deviceKeys = {
  all: ['devices'] as const,
  list: (params: DeviceListParams) => ['devices', 'list', params] as const,
  detail: (id: string) => ['devices', 'detail', id] as const,
}

export const listDevices = (params: DeviceListParams): Promise<Page<Device>> =>
  get<Page<Device>>('/assets/devices', { params: compactParams(params) })

export const getDevice = (id: string): Promise<Device> => get<Device>(`/assets/devices/${id}`)

/** 新建设备：返回一次性 api_key（网关鉴权用，之后不可再查看） */
export const createDevice = (body: DeviceCreate): Promise<DeviceWithKey> => post<DeviceWithKey, DeviceCreate>('/assets/devices', body)

/** 型号 / 固件 / ICCID / 状态 / 备注；status=disabled 后上报被拒 */
export const updateDevice = (id: string, body: DeviceUpdate): Promise<Device> =>
  put<Device, DeviceUpdate>(`/assets/devices/${id}`, body)

/** 软删；已绑定车辆时后端先解绑 */
export const deleteDevice = (id: string): Promise<unknown> => del<unknown>(`/assets/devices/${id}`)

/** 一车一网关；目标车辆已有设备时 409 */
export const bindDevice = (id: string, vehicle_id: string): Promise<Device> =>
  post<Device>(`/assets/devices/${id}/bind`, { vehicle_id })

/** 车辆在途时 409 */
export const unbindDevice = (id: string): Promise<Device> => post<Device>(`/assets/devices/${id}/unbind`)

/** 旧 key 立即失效，新 key 只返回一次 */
export const rotateDeviceKey = (id: string): Promise<DeviceWithKey> => post<DeviceWithKey>(`/assets/devices/${id}/rotate-key`)
