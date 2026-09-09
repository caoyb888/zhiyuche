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
