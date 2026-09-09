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

/** 任意值 → 缩进 JSON 文本（不可序列化时退化为 String） */
export function stringifyJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? ''
  } catch {
    return String(value)
  }
}
