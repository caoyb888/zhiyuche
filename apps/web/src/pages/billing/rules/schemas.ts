import { z } from 'zod'
import type { AccountLevel, BillingPenaltyRule, BillingPenaltyType, BillingRule, BillingRuleDoc, BillingTripInput, TripType } from '../../../api/types'
import type { SelectOption } from '../../../components/ui/Select'
import { localToRFC3339 } from '../../../utils/format'

/**
 * 缺省模板：与 apps/api/internal/billing/engine/engine.go 的 Default()（纯电车队标准套餐）一致。
 * GET /billing/rules/template 不可用时作为本地回退。
 */
export const DEFAULT_RULE_DOC: BillingRuleDoc = {
  rule_name: '纯电车队标准套餐',
  base_rate: { per_km: 0.6, per_hour: 5, daily_cap: 80 },
  ev_specific: {
    electricity_cost_per_kwh: 0.65,
    include_electricity_in_trip: true,
    charging_attribution: 'department',
    low_battery_surcharge: { threshold_soc: 20, surcharge_pct: 10 },
  },
  time_multipliers: [
    { name: '早高峰', range: '07:30-09:00', factor: 1.3 },
    { name: '晚高峰', range: '17:30-19:00', factor: 1.3 },
    { name: '深夜', range: '23:00-06:00', factor: 0.7 },
  ],
  penalty_rules: [
    { type: 'overspeed', threshold_kmh: 80, fine_per_event: 5, max_fine: 50 },
    { type: 'not_charging_on_return', min_soc_required: 30, fine: 10 },
  ],
  trip_attribution: { official: 'department', daily: 'employee' },
}

const TIME_RULE = /^([01]\d|2[0-3]):[0-5]\d$/
const DATE_RULE = /^\d{4}-\d{2}-\d{2}$/

/** 数字文本字段：允许空（视为 0 / 不填），否则须为 ≥ min（≤ max）的数字 */
function numField(message: string, min = 0, max?: number) {
  return z
    .string()
    .trim()
    .refine((v) => {
      if (v === '') return true
      const n = Number(v)
      return Number.isFinite(n) && n >= min && (max === undefined || n <= max)
    }, message)
}

const optionalDate = z.string().trim().refine((v) => v === '' || DATE_RULE.test(v), '日期格式为 YYYY-MM-DD')
const levelEnum = z.enum(['enterprise', 'department', 'employee'])
const penaltyTypeEnum = z.enum(['overspeed', 'not_charging_on_return', 'harsh_driving', 'late_return'])

export const multiplierSchema = z
  .object({
    name: z.string().trim().min(1, '请填写时段名称').max(32, '最多 32 个字符'),
    start: z.string().regex(TIME_RULE, '时间格式为 HH:MM'),
    end: z.string().regex(TIME_RULE, '时间格式为 HH:MM'),
    factor: z.string().trim().refine((v) => v !== '' && Number.isFinite(Number(v)) && Number(v) > 0, '系数必须大于 0'),
  })
  .superRefine((v, ctx) => {
    if (TIME_RULE.test(v.start) && TIME_RULE.test(v.end) && v.start === v.end) {
      ctx.addIssue({ code: 'custom', path: ['end'], message: '起止时间不能相同' })
    }
  })

export const penaltySchema = z.object({
  type: penaltyTypeEnum,
  threshold_kmh: numField('阈值须 ≥ 0'),
  fine_per_event: numField('每次罚金须 ≥ 0'),
  min_soc_required: numField('最低 SOC 为 0–100', 0, 100),
  fine: numField('罚金须 ≥ 0'),
  fine_per_hour: numField('每小时罚金须 ≥ 0'),
  max_fine: numField('上限须 ≥ 0（0 不限）'),
})

/** 与 engine.Validate 一致的前端校验 */
export const ruleFormSchema = z
  .object({
    name: z.string().trim().min(1, '请填写规则名称').max(64, '最多 64 个字符'),
    enabled: z.boolean(),
    effective_from: optionalDate,
    effective_to: optionalDate,
    rule_name: z.string().trim().min(1, 'rule_name 不能为空').max(64, '最多 64 个字符'),
    per_km: numField('每公里费率须 ≥ 0'),
    per_hour: numField('每小时费率须 ≥ 0'),
    daily_cap: numField('日封顶须 ≥ 0（0 不封顶）'),
    electricity_cost_per_kwh: numField('电价须 ≥ 0'),
    include_electricity_in_trip: z.boolean(),
    charging_attribution: levelEnum,
    threshold_soc: numField('阈值为 0–100', 0, 100),
    surcharge_pct: numField('附加百分比须 ≥ 0'),
    time_multipliers: z.array(multiplierSchema),
    penalty_rules: z.array(penaltySchema),
    official: levelEnum,
    daily: levelEnum,
  })
  .superRefine((v, ctx) => {
    if (num(v.per_km) === 0 && num(v.per_hour) === 0) {
      ctx.addIssue({ code: 'custom', path: ['per_km'], message: '每公里与每小时费率不能同时为 0' })
    }
    if (v.effective_from && v.effective_to && v.effective_from > v.effective_to) {
      ctx.addIssue({ code: 'custom', path: ['effective_to'], message: '生效结束不能早于开始' })
    }
    const seen = new Set<BillingPenaltyType>()
    v.penalty_rules.forEach((p, i) => {
      if (seen.has(p.type)) ctx.addIssue({ code: 'custom', path: ['penalty_rules', i, 'type'], message: '同类罚金规则重复' })
      seen.add(p.type)
    })
  })

export type RuleFormValues = z.infer<typeof ruleFormSchema>
export type MultiplierFormValues = RuleFormValues['time_multipliers'][number]
export type PenaltyFormValues = RuleFormValues['penalty_rules'][number]

/** '' → 0；其它按 Number 解析（非法视为 0，由 zod 兜底拦截） */
export function num(v: string): number {
  const t = v.trim()
  if (t === '') return 0
  const n = Number(t)
  return Number.isFinite(n) ? n : 0
}

function str(v: number | null | undefined): string {
  return v === null || v === undefined ? '' : String(v)
}

/** "07:30-09:00" → { start, end }；非法时保留原文便于校验提示 */
export function splitRange(range: string): { start: string; end: string } {
  const [a = '', b = ''] = range.split('-').map((s) => s.trim())
  const pad = (s: string) => {
    const m = /^(\d{1,2}):(\d{1,2})$/.exec(s)
    return m ? `${m[1].padStart(2, '0')}:${m[2].padStart(2, '0')}` : s
  }
  return { start: pad(a), end: pad(b) }
}

export function emptyPenalty(type: BillingPenaltyType = 'overspeed'): PenaltyFormValues {
  return { type, threshold_kmh: '', fine_per_event: '', min_soc_required: '', fine: '', fine_per_hour: '', max_fine: '' }
}

export function emptyMultiplier(): MultiplierFormValues {
  return { name: '', start: '', end: '', factor: '1' }
}

/** 规则文档（+ 元信息）→ 表单值 */
export function docToForm(doc: BillingRuleDoc, meta?: Pick<BillingRule, 'name' | 'enabled' | 'effective_from' | 'effective_to'>): RuleFormValues {
  const ev = doc.ev_specific ?? {}
  const lb = ev.low_battery_surcharge ?? {}
  return {
    name: meta?.name ?? doc.rule_name,
    enabled: meta?.enabled ?? true,
    effective_from: meta?.effective_from ?? '',
    effective_to: meta?.effective_to ?? '',
    rule_name: doc.rule_name,
    per_km: str(doc.base_rate?.per_km),
    per_hour: str(doc.base_rate?.per_hour),
    daily_cap: str(doc.base_rate?.daily_cap),
    electricity_cost_per_kwh: str(ev.electricity_cost_per_kwh),
    include_electricity_in_trip: ev.include_electricity_in_trip ?? false,
    charging_attribution: ev.charging_attribution ?? 'department',
    threshold_soc: str(lb.threshold_soc),
    surcharge_pct: str(lb.surcharge_pct),
    time_multipliers: (doc.time_multipliers ?? []).map((m) => ({ name: m.name, ...splitRange(m.range), factor: str(m.factor) })),
    penalty_rules: (doc.penalty_rules ?? []).map((p) => ({
      type: p.type,
      threshold_kmh: str(p.threshold_kmh),
      fine_per_event: str(p.fine_per_event),
      min_soc_required: str(p.min_soc_required),
      fine: str(p.fine),
      fine_per_hour: str(p.fine_per_hour),
      max_fine: str(p.max_fine),
    })),
    official: doc.trip_attribution?.official ?? 'department',
    daily: doc.trip_attribution?.daily ?? 'employee',
  }
}

function penaltyToDoc(p: PenaltyFormValues): BillingPenaltyRule {
  const out: BillingPenaltyRule = { type: p.type }
  const maxFine = num(p.max_fine)
  if (maxFine > 0) out.max_fine = maxFine
  switch (p.type) {
    case 'overspeed':
      out.threshold_kmh = num(p.threshold_kmh)
      out.fine_per_event = num(p.fine_per_event)
      break
    case 'harsh_driving':
      out.fine_per_event = num(p.fine_per_event)
      break
    case 'not_charging_on_return':
      out.min_soc_required = num(p.min_soc_required)
      out.fine = num(p.fine)
      break
    case 'late_return':
      out.fine = num(p.fine)
      out.fine_per_hour = num(p.fine_per_hour)
      break
  }
  return out
}

/** 表单值 → 规则文档（提交 / JSON 视图 / 模拟计算共用） */
export function formToDoc(v: RuleFormValues): BillingRuleDoc {
  return {
    rule_name: v.rule_name.trim(),
    base_rate: { per_km: num(v.per_km), per_hour: num(v.per_hour), daily_cap: num(v.daily_cap) },
    ev_specific: {
      electricity_cost_per_kwh: num(v.electricity_cost_per_kwh),
      include_electricity_in_trip: v.include_electricity_in_trip,
      charging_attribution: v.charging_attribution,
      low_battery_surcharge: { threshold_soc: num(v.threshold_soc), surcharge_pct: num(v.surcharge_pct) },
    },
    time_multipliers: v.time_multipliers.map((m) => ({ name: m.name.trim(), range: `${m.start}-${m.end}`, factor: num(m.factor) })),
    penalty_rules: v.penalty_rules.map(penaltyToDoc),
    trip_attribution: { official: v.official, daily: v.daily },
  }
}

/** 罚金类型 → 该类型使用的字段 */
export const PENALTY_FIELDS: Record<BillingPenaltyType, Array<keyof Omit<PenaltyFormValues, 'type'>>> = {
  overspeed: ['threshold_kmh', 'fine_per_event', 'max_fine'],
  not_charging_on_return: ['min_soc_required', 'fine'],
  harsh_driving: ['fine_per_event', 'max_fine'],
  late_return: ['fine', 'fine_per_hour', 'max_fine'],
}

export const PENALTY_FIELD_LABEL: Record<keyof Omit<PenaltyFormValues, 'type'>, string> = {
  threshold_kmh: '超速阈值（km/h）',
  fine_per_event: '每次罚金（元）',
  min_soc_required: '最低 SOC（%）',
  fine: '固定罚金（元）',
  fine_per_hour: '每小时罚金（元）',
  max_fine: '单次上限（元，0 不限）',
}

export const ACCOUNT_LEVEL_SELECT: SelectOption[] = [
  { value: 'enterprise', label: '企业账户' },
  { value: 'department', label: '部门账户' },
  { value: 'employee', label: '员工账户' },
]

export function isLevel(v: string): v is AccountLevel {
  return v === 'enterprise' || v === 'department' || v === 'employee'
}

// ── 模拟计算 ──────────────────────────────────────────

const localDateTime = z.string().trim().min(1, '请选择时间')

export const simulateSchema = z
  .object({
    trip_type: z.enum(['official', 'daily']),
    start_at: localDateTime,
    end_at: localDateTime,
    distance_km: z.string().trim().refine((v) => v !== '' && Number.isFinite(Number(v)) && Number(v) >= 0, '里程须为 ≥ 0 的数字'),
    energy_kwh: numField('耗电须 ≥ 0'),
    end_soc: numField('SOC 为 0–100', 0, 100),
    overspeed_events: z.string().trim().refine((v) => v === '' || /^\d+$/.test(v), '须为非负整数'),
    harsh_events: z.string().trim().refine((v) => v === '' || /^\d+$/.test(v), '须为非负整数'),
    planned_end: z.string().trim(),
  })
  .superRefine((v, ctx) => {
    if (v.start_at && v.end_at && v.end_at < v.start_at) {
      ctx.addIssue({ code: 'custom', path: ['end_at'], message: '结束时间不能早于开始时间' })
    }
  })

export type SimulateFormValues = z.infer<typeof simulateSchema>

/** 缺省示例：两小时前出发、现在还车，里程 / 耗电取方案 §7.5 示例值 */
export function defaultSimulateValues(): SimulateFormValues {
  const end = new Date()
  end.setSeconds(0, 0)
  const start = new Date(end.getTime() - 2 * 3600_000 - 43 * 60_000)
  const local = (d: Date) => {
    const p = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
  }
  return {
    trip_type: 'official',
    start_at: local(start),
    end_at: local(end),
    distance_km: '47.3',
    energy_kwh: '6.8',
    end_soc: '45',
    overspeed_events: '0',
    harsh_events: '0',
    planned_end: '',
  }
}

export function simulateToTrip(v: SimulateFormValues): BillingTripInput {
  const trip: BillingTripInput = {
    trip_type: v.trip_type as TripType,
    start_at: localToRFC3339(v.start_at) ?? v.start_at,
    end_at: localToRFC3339(v.end_at) ?? v.end_at,
    distance_km: num(v.distance_km),
  }
  if (v.energy_kwh.trim() !== '') trip.energy_kwh = num(v.energy_kwh)
  if (v.end_soc.trim() !== '') trip.end_soc = num(v.end_soc)
  if (v.overspeed_events.trim() !== '') trip.overspeed_events = Number(v.overspeed_events)
  if (v.harsh_events.trim() !== '') trip.harsh_events = Number(v.harsh_events)
  const planned = localToRFC3339(v.planned_end)
  if (planned) trip.planned_end = planned
  return trip
}
