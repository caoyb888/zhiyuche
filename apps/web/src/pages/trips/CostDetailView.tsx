import clsx from 'clsx'
import { AlertTriangle, ExternalLink } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { CostLine, CostLineKind, TripBillingExt } from '../../api/types'
import Badge, { type BadgeColor } from '../../components/ui/Badge'
import { formatMoney } from '../../utils/format'
import { ATTRIBUTION_LABEL } from '../charging/style'
import { isNum, parseCostDetail } from './costDetail'

const KIND_LABEL: Record<CostLineKind, string> = {
  base: '基础',
  multiplier: '时段',
  cap: '封顶',
  surcharge: '附加',
  electricity: '电费',
  penalty: '罚金',
}

const KIND_BADGE: Record<CostLineKind, BadgeColor> = {
  base: 'gray',
  multiplier: 'blue',
  cap: 'green',
  surcharge: 'amber',
  electricity: 'amber',
  penalty: 'red',
}

const BILLING_STATUS: Record<string, { label: string; color: BadgeColor }> = {
  pending: { label: '待计费', color: 'gray' },
  charged: { label: '已扣费', color: 'green' },
  skipped: { label: '不计费', color: 'gray' },
  failed: { label: '计费失败', color: 'red' },
}

/**
 * 数量列（与 billing/engine 的行结构对应）：
 * base / electricity / penalty → `12.3 km`、`1.5 小时`、`2 次`；cap → `1 天`；
 * multiplier（qty 恒为 1）→ 不显示数量；surcharge（qty 为百分比，unit 为 %）→ `10%`
 */
function qtyText(l: CostLine): string {
  if (l.kind === 'multiplier') return '—'
  if (l.unit === '%') return `${l.qty}%`
  if (!l.unit) return l.qty ? String(l.qty) : '—'
  const q = Number.isInteger(l.qty) ? String(l.qty) : l.qty.toFixed(l.unit === 'km' || l.unit === 'kWh' ? 1 : 2)
  return `${q} ${l.unit}`
}

/** 单价列：`0.80 元/km`；系数行为倍率 `× 1.5`；附加行的 unit_price 是计算基数 */
function priceText(l: CostLine): string {
  if (l.kind === 'multiplier') return `× ${l.unit_price}`
  if (l.kind === 'surcharge' && l.unit === '%') return `基数 ¥ ${formatMoney(l.unit_price)}`
  if (l.unit_price === 0) return '—'
  return `${formatMoney(l.unit_price)} 元${l.unit ? `/${l.unit}` : ''}`
}

function amountClass(l: CostLine): string {
  if (l.kind === 'penalty') return 'text-danger-200'
  if (l.kind === 'cap' || l.amount < 0) return 'text-ev-200'
  if (l.kind === 'surcharge') return 'text-warn-200'
  return 'text-ink-strong'
}

function amountText(v: number): string {
  return v < 0 ? `− ¥ ${formatMoney(Math.abs(v))}` : `¥ ${formatMoney(v)}`
}

interface CostDetailViewProps {
  cost: number | null | undefined
  detail: unknown
  /** 后端 trips 表的计费状态（契约尚未收录，返回时展示） */
  billing?: TripBillingExt | null
}

/** 行程费用区：cost_detail 明细表（项目 / 数量 / 单价 / 金额）+ 合计 + 规则、归属、扣费流水 */
export default function CostDetailView({ cost, detail, billing }: CostDetailViewProps) {
  const d = parseCostDetail(detail)
  const status = billing?.billing_status ? BILLING_STATUS[billing.billing_status] ?? { label: billing.billing_status, color: 'gray' as BadgeColor } : null
  const txnId = billing?.account_txn_id

  if (!isNum(cost)) {
    const failed = billing?.billing_status === 'failed'
    return (
      <div className={clsx('flex items-start gap-2 rounded-xl px-4 py-3 text-sm', failed ? 'bg-danger-500/10 text-danger-200' : 'bg-surface-3 text-ink-faint')}>
        {failed && <AlertTriangle size={16} className="mt-0.5 shrink-0" />}
        <div>
          <div className="flex items-center gap-2">
            {status ? <Badge color={status.color}>{status.label}</Badge> : <span>待计费</span>}
            {!status && <span className="text-xs">行程结束后按生效的计费规则自动计价</span>}
            {billing?.billing_status === 'skipped' && <span className="text-xs text-ink-faint">该行程不计费（无生效规则或规则不适用）</span>}
          </div>
          {failed && <div className="mt-1 text-xs">{billing?.billing_error || '计价或扣费失败，请检查计费规则与归属账户后重试'}</div>}
        </div>
      </div>
    )
  }

  const attribution = d?.attribution ? (ATTRIBUTION_LABEL as Record<string, string>)[d.attribution] ?? d.attribution : null

  return (
    <div className="overflow-hidden rounded-xl border border-line">
      {d && d.lines.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th>项目</th>
                <th className="text-right">数量</th>
                <th className="text-right">单价</th>
                <th className="text-right">金额</th>
              </tr>
            </thead>
            <tbody>
              {d.lines.map((l, i) => (
                <tr key={`${l.kind}-${l.item}-${i}`}>
                  <td>
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span className={clsx('text-sm', l.kind === 'penalty' ? 'text-danger-200' : 'text-ink')}>{l.item}</span>
                      {l.kind !== 'base' && (
                        <Badge color={KIND_BADGE[l.kind]}>
                          {l.kind === 'cap' ? '封顶减免' : KIND_LABEL[l.kind]}
                          {l.kind === 'multiplier' && l.note && <span className="font-mono">{l.note}</span>}
                        </Badge>
                      )}
                    </div>
                    {l.note && l.kind !== 'multiplier' && <div className="mt-0.5 text-[11px] text-ink-faint">{l.note}</div>}
                  </td>
                  <td className="whitespace-nowrap text-right text-xs text-ink">{qtyText(l)}</td>
                  <td className="whitespace-nowrap text-right text-xs text-ink-muted">{priceText(l)}</td>
                  <td className={clsx('whitespace-nowrap text-right text-sm font-medium', amountClass(l))}>{amountText(l.amount)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="px-4 py-3 text-xs text-ink-faint">{d ? '无明细行' : '无计费明细'}</div>
      )}

      {d && (d.multiplier !== 1 || d.cap_applied || d.penalty > 0 || d.surcharge > 0 || d.electricity > 0) && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-line-soft px-4 py-2 text-[11px] text-ink-muted">
          <span>基础 ¥ {formatMoney(d.base)}</span>
          {d.multiplier !== 1 && <span className="text-brand-300">时段系数 ×{d.multiplier}</span>}
          {d.cap_applied && <span className="text-ev-200">已触发日封顶</span>}
          {d.surcharge > 0 && <span className="text-warn-200">附加 ¥ {formatMoney(d.surcharge)}</span>}
          {d.electricity > 0 && <span className="text-warn-200">电费 ¥ {formatMoney(d.electricity)}</span>}
          {d.penalty > 0 && <span className="text-danger-200">罚金 ¥ {formatMoney(d.penalty)}</span>}
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 bg-brand-600/12 px-4 py-3">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-brand-800/80">
          {d?.rule_name && <span>规则：{d.rule_name}</span>}
          {attribution && <span>扣费：{attribution}</span>}
          {status && <Badge color={status.color}>{status.label}</Badge>}
          {typeof txnId === 'number' && (
            <Link to={`/billing/accounts?txn=${txnId}`} className="inline-flex items-center gap-0.5 text-brand-300 hover:underline">
              流水 #{txnId}
              <ExternalLink size={11} />
            </Link>
          )}
        </div>
        <div className="flex items-baseline gap-2">
          <span className="font-semibold text-brand-800">合计</span>
          <span className="text-lg font-bold text-brand-300">¥ {formatMoney(cost)}</span>
        </div>
      </div>
    </div>
  )
}
