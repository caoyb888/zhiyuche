import dayjs from 'dayjs'
import { z } from 'zod'
import type { ApprovalCreate, PlannedRoute, UserBrief } from '../../api/types'
import { localToRFC3339 } from '../../utils/format'

const userBriefSchema = z.object({
  id: z.string(),
  name: z.string(),
  username: z.string().optional(),
  dept_name: z.string().nullable().optional(),
  phone: z.string().nullable().optional(),
})

const pointSchema = z.object({ lng: z.number(), lat: z.number(), address: z.string().optional() })

const URL_RULE = /^(https?:\/\/|\/)/i

export const approvalFormSchema = z
  .object({
    /** 代人发起（approval:manage）；null = 本人 */
    applicant: userBriefSchema.nullable(),
    trip_type: z.enum(['official', 'daily']),
    purpose_code: z.string().min(1, '请选择用车事由'),
    purpose_detail: z.string().trim().min(2, '具体说明 2–500 个字符').max(500, '具体说明 2–500 个字符'),
    /** 本地时间 YYYY-MM-DDTHH:mm */
    planned_start: z.string().min(1, '请选择开始时间'),
    planned_end: z.string().min(1, '请选择结束时间'),
    destination: z.string().trim().min(1, '请填写目的地').max(200, '目的地最多 200 个字符'),
    dest: pointSchema.nullable(),
    planned_km: z.string().trim().refine((v) => v === '' || (Number.isFinite(Number(v)) && Number(v) >= 0), '预计里程须为 ≥ 0 的数字'),
    vehicle_id: z.string(),
    passengers: z.array(userBriefSchema),
    urgency: z.enum(['normal', 'urgent']),
    attachments: z.array(
      z.object({
        name: z.string().trim().min(1, '请填写附件名称').max(100, '名称最多 100 个字符'),
        url: z.string().trim().min(1, '请填写附件地址').refine((v) => URL_RULE.test(v), '地址须以 http(s):// 或 / 开头'),
      }),
    ),
  })
  .superRefine((v, ctx) => {
    const s = dayjs(v.planned_start)
    const e = dayjs(v.planned_end)
    if (s.isValid() && e.isValid() && !e.isAfter(s)) {
      ctx.addIssue({ code: 'custom', path: ['planned_end'], message: '结束时间须晚于开始时间' })
    }
  })

export type ApprovalFormValues = z.infer<typeof approvalFormSchema>

/** 缺省时段：今天下一整点起 3 小时 */
export function defaultPeriod(): { start: string; end: string } {
  const start = dayjs().add(1, 'hour').startOf('hour')
  return { start: start.format('YYYY-MM-DDTHH:mm'), end: start.add(3, 'hour').format('YYYY-MM-DDTHH:mm') }
}

export function defaultApprovalForm(): ApprovalFormValues {
  const { start, end } = defaultPeriod()
  return {
    applicant: null,
    trip_type: 'official',
    purpose_code: '',
    purpose_detail: '',
    planned_start: start,
    planned_end: end,
    destination: '',
    dest: null,
    planned_km: '',
    vehicle_id: '',
    passengers: [],
    urgency: 'normal',
    attachments: [],
  }
}

/** 表单值 → ApprovalCreate 请求体（route 为前端规划结果，可为空） */
export function toApprovalCreate(v: ApprovalFormValues, route: PlannedRoute | null): ApprovalCreate {
  const km = v.planned_km.trim() === '' ? undefined : Number(v.planned_km)
  return {
    applicant_id: v.applicant?.id,
    trip_type: v.trip_type,
    purpose_code: v.purpose_code,
    purpose_detail: v.purpose_detail.trim(),
    planned_start: localToRFC3339(v.planned_start) ?? v.planned_start,
    planned_end: localToRFC3339(v.planned_end) ?? v.planned_end,
    destination: v.destination.trim(),
    dest_lng: v.dest?.lng,
    dest_lat: v.dest?.lat,
    planned_route: route ?? undefined,
    planned_km: km,
    passenger_ids: v.passengers.length > 0 ? v.passengers.map((p: UserBrief) => p.id) : undefined,
    attachments: v.attachments.length > 0 ? v.attachments.map((f) => ({ name: f.name.trim(), url: f.url.trim() })) : undefined,
    vehicle_id: v.vehicle_id || undefined,
    urgency: v.urgency,
  }
}
