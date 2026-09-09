import { compactParams, get, post, put } from './client'
import type { Booking, BookingCreate, BookingSource, BookingStatus, BookingUpdate, Page, PageQuery, VehicleBrief } from './types'

/** GET /bookings 的筛选条件（不含分页） */
export interface BookingFilter {
  source?: BookingSource | ''
  status?: BookingStatus | ''
  vehicle_id?: string
  /** 用车人部门（含子部门） */
  dept_id?: string
  /** reserve_start ≥ */
  from?: string
  /** reserve_start ≤ */
  to?: string
  /** 单号 / 联系人 / 电话 / 出发地 / 目的地 / 事由 */
  keyword?: string
}

export type BookingListParams = BookingFilter & PageQuery

export interface BookingAvailableVehiclesQuery {
  start: string
  end: string
  /** 编辑 / 改派时排除自身 */
  exclude_booking_id?: string
}

export interface BookingCancelBody {
  reason?: string
}

export const bookingKeys = {
  all: ['bookings'] as const,
  list: (params: BookingListParams) => ['bookings', 'list', params] as const,
  detail: (id: string) => ['bookings', 'detail', id] as const,
  availableVehicles: (q: BookingAvailableVehiclesQuery) => ['bookings', 'available-vehicles', q] as const,
}

export const listBookings = (params: BookingListParams): Promise<Page<Booking>> =>
  get<Page<Booking>>('/bookings', { params: compactParams(params) })

export const getBooking = (id: string): Promise<Booking> => get<Booking>(`/bookings/${id}`)

/** 录入即生效；车辆时段冲突返回 409 */
export const createBooking = (body: BookingCreate): Promise<Booking> => post<Booking, BookingCreate>('/bookings', body)

/** 仅「待出车」可改；只提交有变化的键 */
export const updateBooking = (id: string, body: BookingUpdate): Promise<Booking> => put<Booking, BookingUpdate>(`/bookings/${id}`, body)

/** 待出车 → 已出车（须已派车） */
export const departBooking = (id: string): Promise<Booking> => post<Booking>(`/bookings/${id}/depart`)

/** 已出车 → 已完成 */
export const completeBooking = (id: string): Promise<Booking> => post<Booking>(`/bookings/${id}/complete`)

/** 仅「待出车」可取消 */
export const cancelBooking = (id: string, body: BookingCancelBody = {}): Promise<Booking> =>
  post<Booking, BookingCancelBody>(`/bookings/${id}/cancel`, body)

/** 时段内空闲且无冲突预约 / 申请的车辆（含 SOC、续航） */
export const listBookingAvailableVehicles = (q: BookingAvailableVehiclesQuery): Promise<VehicleBrief[]> =>
  get<VehicleBrief[]>('/bookings/available-vehicles', { params: compactParams(q) })
