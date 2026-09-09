import dayjs from 'dayjs'

export const EMPTY = '—'

/** RFC3339 → `YYYY-MM-DD HH:mm`，空值显示 — */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return EMPTY
  const d = dayjs(value)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm') : EMPTY
}

/** RFC3339 → `YYYY-MM-DD` */
export function formatDate(value: string | null | undefined): string {
  if (!value) return EMPTY
  const d = dayjs(value)
  return d.isValid() ? d.format('YYYY-MM-DD') : EMPTY
}

/** 空字符串 / null / undefined 统一显示 — */
export function text(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return EMPTY
  return String(value)
}

/** 金额，保留两位小数并加千分位 */
export function formatMoney(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return EMPTY
  return value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

/** `<input type="datetime-local">` 的值（本地时间 `YYYY-MM-DDTHH:mm`）→ RFC3339；空或非法返回 undefined */
export function localToRFC3339(value: string | null | undefined): string | undefined {
  if (!value) return undefined
  const d = dayjs(value)
  return d.isValid() ? d.toISOString() : undefined
}

/** RFC3339 → `<input type="datetime-local">` 的值；空或非法返回 '' */
export function rfc3339ToLocal(value: string | null | undefined): string {
  if (!value) return ''
  const d = dayjs(value)
  return d.isValid() ? d.format('YYYY-MM-DDTHH:mm') : ''
}

/** 时间段：同一天显示 `MM-DD HH:mm → HH:mm`，跨天显示两端完整时间；结束为空显示"进行中" */
export function formatTimeRange(start: string | null | undefined, end: string | null | undefined, ongoingText = '进行中'): string {
  const s = start ? dayjs(start) : null
  const e = end ? dayjs(end) : null
  if (!s || !s.isValid()) return EMPTY
  const left = s.format('MM-DD HH:mm')
  if (!e || !e.isValid()) return `${left} → ${ongoingText}`
  const right = s.isSame(e, 'day') ? e.format('HH:mm') : e.format('MM-DD HH:mm')
  return `${left} → ${right}`
}

/** 分钟数 → `x 小时 y 分` / `y 分钟`，空值显示 — */
export function formatMinutes(min: number | null | undefined): string {
  if (min === null || min === undefined || !Number.isFinite(min)) return EMPTY
  const m = Math.max(0, Math.round(min))
  if (m >= 60) {
    const h = Math.floor(m / 60)
    const r = m % 60
    return r > 0 ? `${h} 小时 ${r} 分` : `${h} 小时`
  }
  return `${m} 分钟`
}

/** 数字 → 固定小数位文本，空值显示 —；可附单位 */
export function formatNumber(value: number | null | undefined, digits = 1, unit = ''): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return EMPTY
  return `${value.toLocaleString('zh-CN', { minimumFractionDigits: digits, maximumFractionDigits: digits })}${unit ? ` ${unit}` : ''}`
}

/** 任意值 → 缩进 JSON 文本（不可序列化时退化为 String） */
export function stringifyJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? ''
  } catch {
    return String(value)
  }
}
