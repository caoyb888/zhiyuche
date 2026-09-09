import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ClipboardCheck, Eye } from 'lucide-react'
import { useState } from 'react'
import { chargingKeys, listChargeTransactions, type ChargeTxListParams } from '../../api/charging'
import { errorMessage } from '../../api/client'
import type { ChargeTransaction } from '../../api/types'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import Pagination from '../../components/ui/Pagination'
import Table, { type Column } from '../../components/ui/Table'
import { usePermission } from '../../hooks/usePermission'
import { DeviationCell, KwhCell, PileCell, StatusCell, TimeCell, TxNoCell, UserCell, VehicleCell } from './cells'
import { ReviewModal } from './ChargingModals'
import { reviewReason } from './style'

const DEFAULT_PAGE_SIZE = 20

interface ReviewTabProps {
  onOpen: (id: string) => void
}

function ReasonCell({ t }: { t: ChargeTransaction }) {
  const r = reviewReason(t)
  const color = r.kind === 'unbound' ? 'amber' : r.kind === 'deviation' ? 'red' : r.kind === 'pricing' ? 'purple' : 'gray'
  return <Badge color={color}>{r.label}</Badge>
}

/** 待复核页签：review_status=pending 的事务，按开始时间升序（先到先审） */
export default function ReviewTab({ onOpen }: ReviewTabProps) {
  const { can } = usePermission()
  const canReview = can('charging:review')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [target, setTarget] = useState<ChargeTransaction | null>(null)

  const params: ChargeTxListParams = { review_status: 'pending', page, pageSize, sort: 'start_at' }
  const list = useQuery({ queryKey: chargingKeys.txList(params), queryFn: () => listChargeTransactions(params), placeholderData: keepPreviousData, refetchInterval: 60_000 })

  const columns: Column<ChargeTransaction>[] = [
    { key: 'tx_no', title: '事务号', render: (t) => <TxNoCell t={t} /> },
    { key: 'reason', title: '原因', render: (t) => <ReasonCell t={t} /> },
    { key: 'pile', title: '桩 / 连接器', render: (t) => <PileCell t={t} /> },
    { key: 'user', title: '用户 / 部门', render: (t) => <UserCell t={t} /> },
    { key: 'vehicle', title: '车牌', render: (t) => <VehicleCell t={t} /> },
    { key: 'start_at', title: '开始 → 结束', render: (t) => <TimeCell t={t} /> },
    { key: 'kwh', title: '桩侧 / BMS', align: 'right', render: (t) => <KwhCell t={t} /> },
    { key: 'deviation', title: '偏差', align: 'right', render: (t) => <DeviationCell t={t} /> },
    { key: 'status', title: '状态', render: (t) => <StatusCell t={t} /> },
  ]

  return (
    <div className="space-y-4">
      <div className="card p-4">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <div className="text-xs text-slate-400">未绑定车辆、桩侧计量与 BMS 估算偏差超过 5%、或计价失败的事务进入待复核；通过后按当前规则计价扣费，拒绝则不计费。</div>
          {typeof list.data?.total === 'number' && <span className="text-xs text-slate-500">共 {list.data.total} 条</span>}
        </div>
        {list.isError ? (
          <ErrorState message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
        ) : (
          <>
            <Table
              columns={columns}
              data={list.data?.items}
              rowKey="id"
              loading={list.isFetching}
              onRowClick={(t) => onOpen(t.id)}
              actions={{
                width: canReview ? 130 : 60,
                render: (t) => (
                  <div className="flex items-center justify-end gap-0.5">
                    <Button variant="ghost" size="sm" icon={Eye} className="!px-2" title="查看" aria-label="查看" onClick={() => onOpen(t.id)} />
                    {canReview && (
                      <Button size="sm" icon={ClipboardCheck} onClick={() => setTarget(t)} disabled={t.status === 'charging'} title={t.status === 'charging' ? '充电结束后才能复核' : undefined}>
                        复核
                      </Button>
                    )}
                  </div>
                ),
              }}
              empty={<Empty title="没有待复核的充电事务" description="归属明确且计量偏差在阈值内的事务会自动结算" />}
            />
            <Pagination
              className="mt-4"
              page={page}
              pageSize={pageSize}
              total={list.data?.total ?? 0}
              onChange={(p, s) => {
                setPage(p)
                setPageSize(s)
              }}
            />
          </>
        )}
      </div>

      <ReviewModal transaction={target} onClose={() => setTarget(null)} />
    </div>
  )
}
