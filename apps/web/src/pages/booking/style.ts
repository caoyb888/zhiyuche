import type { BookingSource, BookingStatus } from '../../api/types'
import type { BadgeColor } from '../../components/ui/Badge'
import type { SelectOption } from '../../components/ui/Select'

export const BOOKING_SOURCE_LABEL: Record<BookingSource, string> = {
  phone: '电话预约',
  direct: '直接预约',
}

export const BOOKING_SOURCE_BADGE: Record<BookingSource, BadgeColor> = {
  phone: 'amber',
  direct: 'blue',
}

export const BOOKING_SOURCE_OPTIONS: SelectOption[] = (Object.keys(BOOKING_SOURCE_LABEL) as BookingSource[]).map((s) => ({
  value: s,
  label: BOOKING_SOURCE_LABEL[s],
}))

export function isBookingSource(v: string): v is BookingSource {
  return v in BOOKING_SOURCE_LABEL
}

export const BOOKING_STATUS_LABEL: Record<BookingStatus, string> = {
  reserved: '待出车',
  departed: '已出车',
  completed: '已完成',
  cancelled: '已取消',
}

export const BOOKING_STATUS_BADGE: Record<BookingStatus, BadgeColor> = {
  reserved: 'amber',
  departed: 'blue',
  completed: 'green',
  cancelled: 'gray',
}

export const BOOKING_STATUS_OPTIONS: SelectOption[] = (Object.keys(BOOKING_STATUS_LABEL) as BookingStatus[]).map((s) => ({
  value: s,
  label: BOOKING_STATUS_LABEL[s],
}))

export function isBookingStatus(v: string): v is BookingStatus {
  return v in BOOKING_STATUS_LABEL
}
