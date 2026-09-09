import { compactParams, del, get, post, put } from './client'
import type { Page, PageQuery, TelemetryPoint, Vehicle, VehicleBrief, VehicleCreate, VehicleLive, VehicleStatus, VehicleUpdate } from './types'

/** GET /assets/vehicles 的筛选条件（不含分页） */
export interface VehicleFilter {
  /** 车牌 / VIN / 品牌 / 型号 模糊 */
  keyword?: string
  status?: VehicleStatus | ''
  /** 归属部门（含子部门） */
  dept_id?: string
  online?: boolean
}

export type VehicleListParams = VehicleFilter & PageQuery

export interface TelemetryQuery {
  from: string
  to: string
  /** 最多 5000 */
  limit?: number
}

export const vehicleKeys = {
  all: ['vehicles'] as const,
  list: (params: VehicleListParams) => ['vehicles', 'list', params] as const,
  /** 全部车辆实时状态（地图用）；WebSocket vehicle.status 事件就地更新此缓存 */
  status: ['vehicles', 'status'] as const,
  options: (status?: VehicleStatus | '') => ['vehicles', 'options', status ?? ''] as const,
  detail: (id: string) => ['vehicles', 'detail', id] as const,
  live: (id: string) => ['vehicles', 'live', id] as const,
  telemetry: (id: string, q: TelemetryQuery) => ['vehicles', 'telemetry', id, q] as const,
}

export const listVehicles = (params: VehicleListParams): Promise<Page<Vehicle>> =>
  get<Page<Vehicle>>('/assets/vehicles', { params: compactParams(params) })

/** 车辆简表（下拉） */
export const listVehicleOptions = (status?: VehicleStatus | ''): Promise<VehicleBrief[]> =>
  get<VehicleBrief[]>('/assets/vehicles/options', { params: compactParams({ status: status || undefined }) })

/** 全部车辆实时状态（含车牌、型号、驾驶员） */
export const listVehicleLiveStatus = (): Promise<VehicleLive[]> => get<VehicleLive[]>('/assets/vehicles/status')

/** 详情：含实时状态、绑定设备、累计行程数 */
export const getVehicle = (id: string): Promise<Vehicle> => get<Vehicle>(`/assets/vehicles/${id}`)

export const getVehicleLiveStatus = (id: string): Promise<VehicleLive> => get<VehicleLive>(`/assets/vehicles/${id}/status`)

/** 历史遥测（按 ts 升序） */
export const listVehicleTelemetry = (id: string, q: TelemetryQuery): Promise<TelemetryPoint[]> =>
  get<TelemetryPoint[]>(`/assets/vehicles/${id}/telemetry`, { params: compactParams(q) })

export const createVehicle = (body: VehicleCreate): Promise<Vehicle> => post<Vehicle, VehicleCreate>('/assets/vehicles', body)

/** status 只允许 idle/maintenance/disabled 之间手动切换；在途车辆不可改状态（409） */
export const updateVehicle = (id: string, body: VehicleUpdate): Promise<Vehicle> =>
  put<Vehicle, VehicleUpdate>(`/assets/vehicles/${id}`, body)

/** 软删；在途或有未完成申请时 409 */
export const deleteVehicle = (id: string): Promise<unknown> => del<unknown>(`/assets/vehicles/${id}`)
