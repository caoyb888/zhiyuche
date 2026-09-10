import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { CheckCircle2, Download, Printer } from 'lucide-react'
import { Link } from 'react-router-dom'
import { billingKeys, getSettlement } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { Settlement, SettlementLine } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import DescriptionList from '../../../components/ui/DescriptionList'
import Drawer from '../../../components/ui/Drawer'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import Spinner from '../../../components/ui/Spinner'
import Table, { type Column } from '../../../components/ui/Table'
import { formatDateTime, formatMoney, formatNumber, text } from '../../../utils/format'
import { SETTLEMENT_LINE_KIND_BADGE, SETTLEMENT_LINE_KIND_LABEL, SETTLEMENT_STATUS_BADGE, SETTLEMENT_STATUS_LABEL, budgetUsage, lineQuantityUnit, refLink } from '../style'
import { groupLines } from './print'

interface SettlementDetailDrawerProps {
  id: string | null
  onClose: () => void
  canConfirm: boolean
  canExport: boolean
  onConfirm: (s: Settlement) => void
  onExport: (s: Settlement) => void
  onPrint: (s: Settlement) => void
  exporting?: boolean
}

function LineRef({ l }: { l: SettlementLine }) {
  const label = l.ref_no || `${l.ref_id.slice(0, 8)}…`
  const href = refLink(l.kind === 'charge' ? 'charge_transaction' : 'trip', l.ref_id)
  if (!href) return <span className="font-mono text-xs">{label}</span>
  return (
    <Link to={href} className="font-mono text-xs text-brand-600 hover:underline">
      {label}
    </Link>
  )
}

const lineColumns: Column<SettlementLine>[] = [
  { key: 'kind', title: '类型', width: 80, render: (l) => <Badge color={SETTLEMENT_LINE_KIND_BADGE[l.kind]}>{SETTLEMENT_LINE_KIND_LABEL[l.kind]}</Badge> },
  { key: 'ref', title: '单号', render: (l) => <LineRef l={l} /> },
  { key: 'occurred_at', title: '日期', render: (l) => <span className="text-xs text-ink whitespace-nowrap">{formatDateTime(l.occurred_at)}</span> },
  { key: 'user_name', title: '用车人', render: (l) => <span className="text-sm">{text(l.user_name)}</span> },
  { key: 'vehicle_plate', title: '车牌', render: (l) => <span className="font-mono text-xs">{text(l.vehicle_plate)}</span> },
  { key: 'quantity', title: '数量', align: 'right', render: (l) => <span className="font-mono text-xs">{l.quantity === null || l.quantity === undefined ? '—' : `${formatNumber(l.quantity, 1)} ${lineQuantityUnit(l.kind)}`}</span> },
  { key: 'amount', title: '金额（元）', align: 'right', render: (l) => <span className={clsx('font-mono font-medium', l.kind === 'penalty' ? 'text-danger-200' : 'text-ink-strong')}>{formatMoney(l.amount)}</span> },
]

function Body({ id, canConfirm, canExport, onConfirm, onExport, onPrint, exporting }: { id: string } & Omit<SettlementDetailDrawerProps, 'id' | 'onClose'>) {
  const detail = useQuery({ queryKey: billingKeys.settlement(id), queryFn: () => getSettlement(id) })
  if (detail.isPending) {
    return (
      <div className="flex justify-center py-10">
        <Spinner label="加载结算单…" />
      </div>
    )
  }
  if (detail.isError) return <ErrorState message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />
  const s = detail.data
  const groups = groupLines(s.lines)
  const usage = budgetUsage(s.total, s.budget)

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h4 className="text-base font-semibold text-ink-strong">{s.dept_id ? (s.dept_name ?? '部门') : '企业汇总'}</h4>
            <Badge color={s.dept_id ? 'blue' : 'purple'}>{s.dept_id ? '部门' : '企业'}</Badge>
            <Badge color={SETTLEMENT_STATUS_BADGE[s.status]}>{SETTLEMENT_STATUS_LABEL[s.status]}</Badge>
          </div>
          <div className="mt-0.5 text-xs text-ink-faint">结算期 {s.period} · 生成于 {formatDateTime(s.generated_at)}</div>
        </div>
        <div className="flex flex-wrap items-center gap-2 shrink-0">
          {canConfirm && s.status === 'draft' && (
            <Button size="sm" icon={CheckCircle2} onClick={() => onConfirm(s)}>
              确认
            </Button>
          )}
          {canExport && (
            <Button variant="secondary" size="sm" icon={Download} loading={exporting} onClick={() => onExport(s)}>
              导出 xlsx
            </Button>
          )}
          <Button variant="secondary" size="sm" icon={Printer} onClick={() => onPrint(s)}>
            打印
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <div className="rounded-lg bg-brand-600/10 px-3 py-2 col-span-2 sm:col-span-1">
          <div className="text-[11px] text-brand-300/70">合计（元）</div>
          <div className="mt-0.5 font-mono text-lg font-semibold text-brand-300">{formatMoney(s.total)}</div>
        </div>
        <div className="rounded-lg bg-surface-3 px-3 py-2">
          <div className="text-[11px] text-ink-faint">行程费 · {s.trip_count} 次</div>
          <div className="mt-0.5 font-mono text-sm font-medium text-ink-strong">{formatMoney(s.trip_cost)}</div>
        </div>
        <div className="rounded-lg bg-surface-3 px-3 py-2">
          <div className="text-[11px] text-ink-faint">充电费 · {s.charge_count} 次</div>
          <div className="mt-0.5 font-mono text-sm font-medium text-ink-strong">{formatMoney(s.charge_cost)}</div>
        </div>
        <div className="rounded-lg bg-surface-3 px-3 py-2">
          <div className="text-[11px] text-ink-faint">罚金</div>
          <div className={clsx('mt-0.5 font-mono text-sm font-medium', s.penalty > 0 ? 'text-danger-200' : 'text-ink-strong')}>{formatMoney(s.penalty)}</div>
        </div>
      </div>

      <DescriptionList
        columns={2}
        items={[
          { label: '月度预算', value: s.budget > 0 ? `¥ ${formatMoney(s.budget)}` : '未设预算' },
          { label: '预算使用率', value: usage === null ? '—' : <span className={usage >= 100 ? 'text-danger-200 font-medium' : undefined}>{usage.toFixed(1)}%</span> },
          { label: '确认时间', value: s.confirmed_at ? formatDateTime(s.confirmed_at) : '—' },
          { label: '确认人', value: text(s.confirmed_by_name) },
          { label: '结算单号', value: <span className="font-mono text-xs">{s.id}</span>, span: 2 },
        ]}
      />

      <div className="space-y-4">
        <h4 className="text-sm font-semibold text-ink">费用明细</h4>
        {groups.length === 0 ? (
          <Empty size="sm" title="本期无明细" description="该期没有已计费的行程、充电或罚金" />
        ) : (
          groups.map((g) => (
            <div key={g.kind}>
              <div className="mb-1 flex items-center justify-between text-xs">
                <span className="font-medium text-ink">
                  {SETTLEMENT_LINE_KIND_LABEL[g.kind]} · {g.lines.length} 笔
                </span>
                <span className="text-ink-muted">
                  小计 <span className={clsx('font-mono font-medium', g.kind === 'penalty' ? 'text-danger-200' : 'text-ink-strong')}>{formatMoney(g.subtotal)}</span>
                </span>
              </div>
              <Table columns={lineColumns} data={g.lines} rowKey={(l) => String(l.id)} />
            </div>
          ))
        )}
      </div>
    </div>
  )
}

/** 结算单明细抽屉（由 URL ?id= 驱动） */
export default function SettlementDetailDrawer({ id, onClose, ...rest }: SettlementDetailDrawerProps) {
  return (
    <Drawer open={id !== null} onClose={onClose} title="结算单明细" description="按行程 / 充电 / 罚金分组，含小计" width="lg">
      {id && <Body key={id} id={id} {...rest} />}
    </Drawer>
  )
}
