import type { CostDetail, CostLine, CostLineKind, Trip, TripBillingExt } from '../../api/types'

const KIND_SET: Record<CostLineKind, true> = { base: true, multiplier: true, cap: true, surcharge: true, electricity: true, penalty: true }

export function isNum(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v)
}

function isKind(v: unknown): v is CostLineKind {
  return typeof v === 'string' && v in KIND_SET
}

/** 把后端 jsonb 宽松地解析成 CostDetail；缺关键字段返回 null（退化为只显示合计） */
export function parseCostDetail(raw: unknown): CostDetail | null {
  if (typeof raw !== 'object' || raw === null) return null
  const r = raw as Record<string, unknown>
  const rawLines = Array.isArray(r.lines) ? r.lines : []
  const lines: CostLine[] = []
  for (const l of rawLines) {
    if (typeof l !== 'object' || l === null) continue
    const x = l as Record<string, unknown>
    if (typeof x.item !== 'string' || !isNum(x.amount)) continue
    lines.push({
      item: x.item,
      kind: isKind(x.kind) ? x.kind : 'base',
      qty: isNum(x.qty) ? x.qty : 0,
      unit: typeof x.unit === 'string' ? x.unit : '',
      unit_price: isNum(x.unit_price) ? x.unit_price : 0,
      amount: x.amount,
      note: typeof x.note === 'string' && x.note ? x.note : undefined,
    })
  }
  if (lines.length === 0 && !isNum(r.total)) return null
  return {
    rule_name: typeof r.rule_name === 'string' ? r.rule_name : '',
    lines,
    base: isNum(r.base) ? r.base : 0,
    multiplier: isNum(r.multiplier) ? r.multiplier : 1,
    cap_applied: r.cap_applied === true,
    surcharge: isNum(r.surcharge) ? r.surcharge : 0,
    electricity: isNum(r.electricity) ? r.electricity : 0,
    penalty: isNum(r.penalty) ? r.penalty : 0,
    total: isNum(r.total) ? r.total : lines.reduce((s, l) => s + l.amount, 0),
    attribution: typeof r.attribution === 'string' ? r.attribution : '',
  }
}

/** 从 Trip 上读取契约外的计费扩展字段 */
export function tripBilling(t: Trip): TripBillingExt {
  const x = t as Trip & TripBillingExt
  return { billing_status: x.billing_status ?? null, billing_error: x.billing_error ?? null, account_id: x.account_id ?? null, account_txn_id: typeof x.account_txn_id === 'number' ? x.account_txn_id : null, billed_at: x.billed_at ?? null }
}
