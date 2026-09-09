import dayjs from 'dayjs'
import { z } from 'zod'
import type { Booking, BookingCreate, BookingSource, BookingUpdate } from '../../api/types'
import { localToRFC3339, rfc3339ToLocal } from '../../utils/format'

const userBriefSchema = z.object({
  id: z.string(),
  name: z.string(),
  username: z.string().optional(),
  dept_name: z.string().nullable().optional(),
  phone: z.string().nullable().optional(),
})

/** 与后端 internal/booking/rules.go 保持一致 */
export const MIN_DURATION_MIN = 5
export const MAX_DURATION_DAYS = 7

export const bookingFormSchema = z
  .object({
    source: z.enum(['phone', 'direct']),
    contact_name: z.string().trim().min(1, '请填写来电人 / 预约人姓名').max(64, '最多 64 个字符'),
    contact_phone: z.string().trim().max(32, '最多 32 个字符'),
    /** 用车人（系统用户，可选） */
    passenger: userBriefSchema.nullable(),
    /** 本地时间 YYYY-MM-DDTHH:mm */
    reserve_start: z.string().min(1, '请选择预约开始时间'),
    reserve_end: z.string().min(1, '请选择预计结束时间'),
    /** 空串 = 待派车 */
    vehicle_id: z.string(),
    origin: z.string().trim().min(1, '请填写出发地').max(200, '最多 200 个字符'),
    destination: z.string().trim().min(1, '请填写目的地').max(200, '最多 200 个字符'),
    purpose: z.string().trim().max(200, '最多 200 个字符'),
    remark: z.string().trim().max(500, '最多 500 个字符'),
  })
  .superRefine((v, ctx) => {
    const s = dayjs(v.reserve_start)
    const e = dayjs(v.reserve_end)
    if (!s.isValid() || !e.isValid()) return
    const minutes = e.diff(s, 'minute')
    if (minutes <= 0) {
      ctx.addIssue({ code: 'custom', path: ['reserve_end'], message: '结束时间须晚于开始时间' })
      return
    }
    if (minutes < MIN_DURATION_MIN) {
      ctx.addIssue({ code: 'custom', path: ['reserve_end'], message: `预约时长至少 ${MIN_DURATION_MIN} 分钟` })
    }
    if (minutes > MAX_DURATION_DAYS * 24 * 60) {
      ctx.addIssue({ code: 'custom', path: ['reserve_end'], message: `预约时长不能超过 ${MAX_DURATION_DAYS} 天` })
    }
  })

export type BookingFormValues = z.infer<typeof bookingFormSchema>

/** 缺省时段：下一整点起 2 小时 */
export function defaultPeriod(): { start: string; end: string } {
  const start = dayjs().add(1, 'hour').startOf('hour')
  return { start: start.format('YYYY-MM-DDTHH:mm'), end: start.add(2, 'hour').format('YYYY-MM-DDTHH:mm') }
}

export function defaultBookingForm(source: BookingSource): BookingFormValues {
  const { start, end } = defaultPeriod()
  return {
    source,
    contact_name: '',
    contact_phone: '',
    passenger: null,
    reserve_start: start,
    reserve_end: end,
    vehicle_id: '',
    origin: '',
    destination: '',
    purpose: '',
    remark: '',
  }
}

export function bookingToForm(b: Booking): BookingFormValues {
  return {
    source: b.source,
    contact_name: b.contact_name,
    contact_phone: b.contact_phone ?? '',
    passenger: b.passenger ?? null,
    reserve_start: rfc3339ToLocal(b.reserve_start),
    reserve_end: rfc3339ToLocal(b.reserve_end),
    vehicle_id: b.vehicle?.id ?? '',
    origin: b.origin,
    destination: b.destination,
    purpose: b.purpose ?? '',
    remark: b.remark ?? '',
  }
}

export function toBookingCreate(v: BookingFormValues): BookingCreate {
  return {
    source: v.source,
    contact_name: v.contact_name.trim(),
    contact_phone: v.contact_phone.trim() || undefined,
    passenger_id: v.passenger?.id,
    reserve_start: localToRFC3339(v.reserve_start) ?? v.reserve_start,
    reserve_end: localToRFC3339(v.reserve_end) ?? v.reserve_end,
    vehicle_id: v.vehicle_id || undefined,
    origin: v.origin.trim(),
    destination: v.destination.trim(),
    purpose: v.purpose.trim() || undefined,
    remark: v.remark.trim() || undefined,
  }
}

/** 只带上有变化的键；清空用车人 / 车辆走 clear_* 标记 */
export function toBookingUpdate(v: BookingFormValues, before: Booking): BookingUpdate {
  const body: BookingUpdate = {}
  if (v.source !== before.source) body.source = v.source
  if (v.contact_name.trim() !== before.contact_name) body.contact_name = v.contact_name.trim()
  if (v.contact_phone.trim() !== (before.contact_phone ?? '')) body.contact_phone = v.contact_phone.trim()

  const passengerId = v.passenger?.id ?? ''
  const beforePassengerId = before.passenger?.id ?? ''
  if (passengerId !== beforePassengerId) {
    if (passengerId) body.passenger_id = passengerId
    else body.clear_passenger = true
  }

  const start = localToRFC3339(v.reserve_start)
  if (start && !dayjs(start).isSame(before.reserve_start)) body.reserve_start = start
  const end = localToRFC3339(v.reserve_end)
  if (end && !dayjs(end).isSame(before.reserve_end)) body.reserve_end = end

  const beforeVehicleId = before.vehicle?.id ?? ''
  if (v.vehicle_id !== beforeVehicleId) {
    if (v.vehicle_id) body.vehicle_id = v.vehicle_id
    else body.clear_vehicle = true
  }

  if (v.origin.trim() !== before.origin) body.origin = v.origin.trim()
  if (v.destination.trim() !== before.destination) body.destination = v.destination.trim()
  if (v.purpose.trim() !== (before.purpose ?? '')) body.purpose = v.purpose.trim()
  if (v.remark.trim() !== (before.remark ?? '')) body.remark = v.remark.trim()
  return body
}
