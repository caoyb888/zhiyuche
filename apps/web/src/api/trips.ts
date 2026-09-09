import { compactParams, download, get, post, type DownloadedFile } from './client'
import type { Page, PageQuery, Trip, TripEvent, TripStatus, TripSummary, TripTrack, TripType } from './types'

/** 列表视角：我驾驶的 / 全部（trip:manage 或 trip:export） */
export type TripScope = 'mine' | 'all'

export function isTripScope(v: string | null | undefined): v is TripScope {
  return v === 'mine' || v === 'all'
}

/** GET /trips 的筛选条件（不含分页） */
export interface TripFilter {
  scope?: TripScope
  status?: TripStatus | ''
  trip_type?: TripType | ''
  vehicle_id?: string
  driver_id?: string
  /** 驾驶员部门（含子部门） */
  dept_id?: string
  /** start_at ≥ */
  from?: string
  /** start_at ≤ */
  to?: string
  /** 行程号 / 车牌 / 驾驶员 / 事由 */
  keyword?: string
  /** 仅偏离 */
  deviation?: boolean
}

export type TripListParams = TripFilter & PageQuery

export interface StartTripBody {
  approval_id: string
  /** 缺省为申请人 */
  driver_id?: string
  remark?: string
}

export interface EndTripBody {
  end_odometer?: number
  end_soc?: number
  remark?: string
}

export interface CancelTripBody {
  reason?: string
}

/** query key 统一以 ['trips'] 开头：WebSocket trip.started / trip.ended / trip.event 到达时整体失效 */
export const tripKeys = {
  all: ['trips'] as const,
  list: (params: TripListParams) => ['trips', 'list', params] as const,
  detail: (id: string) => ['trips', 'detail', id] as const,
  track: (id: string, step: number) => ['trips', 'track', id, step] as const,
  events: (id: string) => ['trips', 'events', id] as const,
  summary: (date?: string) => ['trips', 'summary', date ?? 'today'] as const,
}

export const listTrips = (params: TripListParams): Promise<Page<Trip>> => get<Page<Trip>>('/trips', { params: compactParams(params) })

/** 详情：含车辆、驾驶员、申请摘要、事件列表 */
export const getTrip = (id: string): Promise<Trip> => get<Trip>(`/trips/${id}`)

/** 轨迹点（ts 升序）；step=n 每 n 点取 1 */
export const getTripTrack = (id: string, step = 1): Promise<TripTrack> =>
  get<TripTrack>(`/trips/${id}/track`, { params: step > 1 ? { step } : undefined, timeout: 60_000 })

export const getTripEvents = (id: string): Promise<TripEvent[]> => get<TripEvent[]>(`/trips/${id}/events`)

/** 日汇总；date 缺省今天 */
export const getTripSummary = (date?: string): Promise<TripSummary> => get<TripSummary>('/trips/summary', { params: compactParams({ date }) })

/** 导出 xlsx（与列表相同筛选，最多 5000 行） */
export const exportTrips = (filter: TripFilter): Promise<DownloadedFile> => download('/trips/export', '行程记录.xlsx', { params: compactParams(filter) })

/** 手动开始行程（基于已批准的申请） */
export const startTrip = (body: StartTripBody): Promise<Trip> => post<Trip, StartTripBody>('/trips/start', body)

/** 手动结束行程：汇总里程 / 能耗 / 速度 / 急加减速 / 偏离 */
export const endTrip = (id: string, body: EndTripBody = {}): Promise<Trip> => post<Trip, EndTripBody>(`/trips/${id}/end`, body)

/** 作废进行中的行程（车辆回到 idle，申请回到 approved） */
export const cancelTrip = (id: string, body: CancelTripBody = {}): Promise<Trip> => post<Trip, CancelTripBody>(`/trips/${id}/cancel`, body)
