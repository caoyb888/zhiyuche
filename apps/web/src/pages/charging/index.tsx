import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { Zap } from 'lucide-react'
import { useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import { chargingKeys, getChargingSummary } from '../../api/charging'
import { errorMessage } from '../../api/client'
import PageHeader from '../../components/ui/PageHeader'
import Tabs, { type TabItem } from '../../components/ui/Tabs'
import { formatMoney } from '../../utils/format'
import PilesLiveTab from './PilesLiveTab'
import ReviewTab from './ReviewTab'
import TransactionDetailDrawer from './TransactionDetailDrawer'
import TransactionsTab from './TransactionsTab'

type TabKey = 'piles' | 'records' | 'review'

function isTabKey(v: string | null): v is TabKey {
  return v === 'piles' || v === 'records' || v === 'review'
}

/** 顶部汇总条：GET /charging/summary，30s 刷新（WS charging.updated 也会失效） */
function SummaryBar() {
  const summary = useQuery({ queryKey: chargingKeys.summary(), queryFn: () => getChargingSummary(), staleTime: 15_000, refetchInterval: 30_000 })
  const s = summary.data
  const groups: Array<{ title: string; items: Array<{ label: string; value: string; cls?: string }> }> = [
    {
      title: '今日',
      items: [
        { label: '次', value: s ? String(s.today?.sessions ?? 0) : '—' },
        { label: 'kWh', value: s ? (s.today?.kwh ?? 0).toFixed(1) : '—', cls: 'text-amber-600' },
        { label: '元', value: s ? formatMoney(s.today?.cost ?? 0) : '—', cls: 'text-emerald-700' },
      ],
    },
    {
      title: '本月',
      items: [
        { label: '次', value: s ? String(s.month?.sessions ?? 0) : '—' },
        { label: 'kWh', value: s ? (s.month?.kwh ?? 0).toFixed(1) : '—', cls: 'text-amber-600' },
        { label: '元', value: s ? formatMoney(s.month?.cost ?? 0) : '—', cls: 'text-emerald-700' },
      ],
    },
    {
      title: '事务',
      items: [
        { label: '进行中', value: s ? String(s.ongoing) : '—', cls: s && s.ongoing > 0 ? 'text-amber-600' : undefined },
        { label: '待复核', value: s ? String(s.pending_review) : '—', cls: s && s.pending_review > 0 ? 'text-red-600' : undefined },
      ],
    },
    {
      title: '充电桩',
      items: [
        { label: '在线', value: s ? `${s.piles?.online ?? 0} / ${s.piles?.total ?? 0}` : '—', cls: 'text-emerald-600' },
        { label: '充电中', value: s ? String(s.piles?.charging ?? 0) : '—', cls: 'text-amber-600' },
        { label: '故障', value: s ? String(s.piles?.faulted ?? 0) : '—', cls: s && (s.piles?.faulted ?? 0) > 0 ? 'text-red-600' : undefined },
      ],
    },
  ]
  return (
    <div className="card flex flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-500">
        <Zap size={14} className="text-brand-600" />
        {s ? `${s.date} 汇总` : '充电汇总'}
      </span>
      {groups.map((g) => (
        <div key={g.title} className="flex items-baseline gap-2">
          <span className="text-[11px] uppercase tracking-wide text-slate-400">{g.title}</span>
          {g.items.map((it) => (
            <span key={it.label} className="flex items-baseline gap-1">
              <span className={clsx('text-lg font-semibold', it.cls ?? 'text-slate-800')}>{it.value}</span>
              <span className="text-xs text-slate-400">{it.label}</span>
            </span>
          ))}
        </div>
      ))}
      {summary.isError && <span className="text-xs text-amber-600">汇总暂不可用：{errorMessage(summary.error)}</span>}
    </div>
  )
}

/** 充电管理：桩实时状态与远程启停、充电记录与详情、待复核队列 */
export default function ChargingPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const rawTab = searchParams.get('tab')
  const tab: TabKey = isTabKey(rawTab) ? rawTab : 'piles'
  const detailId = searchParams.get('tx')

  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set(key, value)
    else next.delete(key)
    setSearchParams(next, { replace: true })
  }
  const openTx = (id: string) => setParam('tx', id)

  const summary = useQuery({ queryKey: chargingKeys.summary(), queryFn: () => getChargingSummary(), staleTime: 15_000 })
  const pendingCount = summary.data?.pending_review

  const tabs = useMemo<TabItem<TabKey>[]>(
    () => [
      { key: 'piles', label: '桩状态' },
      { key: 'records', label: '充电记录' },
      { key: 'review', label: '待复核', count: typeof pendingCount === 'number' ? pendingCount : undefined },
    ],
    [pendingCount],
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader title="充电管理" description="充电桩实时状态与远程启停、充电事务、计量交叉校验与人工复核" />

      <SummaryBar />

      <div className="card px-4 pt-1">
        <Tabs items={tabs} value={tab} onChange={(k) => setParam('tab', k === 'piles' ? null : k)} />
      </div>

      {tab === 'piles' && <PilesLiveTab onOpenTransaction={openTx} />}
      {tab === 'records' && <TransactionsTab onOpen={openTx} />}
      {tab === 'review' && <ReviewTab onOpen={openTx} />}

      <TransactionDetailDrawer id={detailId} onClose={() => setParam('tx', null)} />
    </div>
  )
}
