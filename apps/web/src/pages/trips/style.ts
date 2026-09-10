import { AlertTriangle, Ban, BatteryLow, Flag, Gauge, Lightbulb, LightbulbOff, Route, type LucideIcon } from 'lucide-react'
import type { RoofSignStatus, TripEvent, TripEventType, TripSource, TripStatus } from '../../api/types'
import type { BadgeColor } from '../../components/ui/Badge'
import type { SelectOption } from '../../components/ui/Select'

export const TRIP_STATUS_LABEL: Record<TripStatus, string> = {
  ongoing: '进行中',
  completed: '已完成',
  cancelled: '已作废',
}

export const TRIP_STATUS_BADGE: Record<TripStatus, BadgeColor> = {
  ongoing: 'blue',
  completed: 'green',
  cancelled: 'gray',
}

export const TRIP_STATUS_OPTIONS: SelectOption[] = (Object.keys(TRIP_STATUS_LABEL) as TripStatus[]).map((s) => ({ value: s, label: TRIP_STATUS_LABEL[s] }))

export function isTripStatus(v: string): v is TripStatus {
  return v in TRIP_STATUS_LABEL
}

export const TRIP_SOURCE_LABEL: Record<TripSource, string> = {
  device: '网关刷卡',
  web: '手动调度',
  simulator: '模拟器',
}

export const ROOF_SIGN_LABEL: Record<RoofSignStatus, string> = {
  on: '灯牌亮',
  off: '灯牌灭',
  unknown: '未知',
}

export const ROOF_SIGN_BADGE: Record<RoofSignStatus, BadgeColor> = {
  on: 'blue',
  off: 'gray',
  unknown: 'gray',
}

export type EventLevel = 'red' | 'amber' | 'blue' | 'gray'

export interface EventMeta {
  label: string
  level: EventLevel
  icon: LucideIcon
}

export const EVENT_META: Record<TripEventType, EventMeta> = {
  start: { label: '行程开始', level: 'blue', icon: Flag },
  end: { label: '行程结束', level: 'blue', icon: Flag },
  deviation: { label: '路线偏离', level: 'red', icon: Route },
  overspeed: { label: '超速', level: 'red', icon: Gauge },
  low_soc: { label: '低电量', level: 'amber', icon: BatteryLow },
  sign_on: { label: '灯牌亮起', level: 'gray', icon: Lightbulb },
  sign_off: { label: '灯牌熄灭', level: 'gray', icon: LightbulbOff },
  cancel: { label: '行程作废', level: 'gray', icon: Ban },
}

export const EVENT_LEVEL_CLASS: Record<EventLevel, { dot: string; text: string; bg: string }> = {
  red: { dot: 'bg-danger-500/20 text-danger-200', text: 'text-danger-200', bg: 'bg-danger-500/10' },
  amber: { dot: 'bg-warn-500/20 text-warn-200', text: 'text-warn-200', bg: 'bg-warn-500/10' },
  blue: { dot: 'bg-tech-400/20 text-brand-300', text: 'text-ink', bg: 'bg-surface-3' },
  gray: { dot: 'bg-surface-4 text-ink-muted', text: 'text-ink', bg: 'bg-surface-3' },
}

export function eventMeta(type: string): EventMeta {
  return (EVENT_META as Record<string, EventMeta>)[type] ?? { label: type, level: 'gray', icon: AlertTriangle }
}

function num(v: unknown): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

function meters(d: number): string {
  return d >= 1000 ? `${(d / 1000).toFixed(1)} km` : `${Math.round(d)} m`
}

/** 事件 payload → 一句话摘要，如 "超速 92 km/h（限 80）" */
export function describeEvent(e: Pick<TripEvent, 'type' | 'payload'>): string {
  const p = (e.payload ?? {}) as Record<string, unknown>
  switch (e.type) {
    case 'overspeed': {
      const speed = num(p.speed)
      const limit = num(p.limit)
      return speed !== null ? `超速 ${Math.round(speed)} km/h${limit !== null ? `（限 ${Math.round(limit)}）` : ''}` : '超速行驶'
    }
    case 'deviation': {
      const d = num(p.distance_m)
      return d !== null ? `偏离计划路线约 ${meters(d)}` : '偏离计划路线'
    }
    case 'low_soc': {
      const soc = num(p.soc)
      return soc !== null ? `电量仅剩 ${Math.round(soc)}%` : '电量过低'
    }
    case 'start': {
      const soc = num(p.soc)
      const odo = num(p.odometer_km)
      const parts = []
      if (odo !== null) parts.push(`里程表 ${odo.toFixed(1)} km`)
      if (soc !== null) parts.push(`SOC ${Math.round(soc)}%`)
      return parts.length > 0 ? parts.join(' · ') : '行程开始'
    }
    case 'end': {
      const dist = num(p.distance_km)
      const energy = num(p.energy_kwh)
      const soc = num(p.soc)
      const parts = []
      if (dist !== null) parts.push(`行驶 ${dist.toFixed(1)} km`)
      if (energy !== null) parts.push(`耗电 ${energy.toFixed(1)} kWh`)
      if (soc !== null) parts.push(`SOC ${Math.round(soc)}%`)
      return parts.length > 0 ? parts.join(' · ') : '行程结束'
    }
    case 'cancel': {
      const reason = typeof p.reason === 'string' && p.reason ? p.reason : null
      return reason ? `作废：${reason}` : '行程作废'
    }
    default:
      return eventMeta(e.type).label
  }
}

/** 抽稀：轨迹点超过阈值时按 step 抽取（GET /trips/{id}/track?step=） */
export const TRACK_MAX_POINTS = 2000

export function trackStep(pointCount: number | null | undefined): number {
  if (!pointCount || pointCount <= TRACK_MAX_POINTS) return 1
  return Math.ceil(pointCount / TRACK_MAX_POINTS)
}
