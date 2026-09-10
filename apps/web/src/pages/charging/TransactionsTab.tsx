import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query'
import { Download, Eye } from 'lucide-react'
import { useState } from 'react'
import { chargingKeys, exportChargeTransactions, listChargeTransactions, type ChargeTxFilter, type ChargeTxListParams } from '../../api/charging'
import { errorMessage } from '../../api/client'
import type { ChargeReviewStatus, ChargeStatus, ChargeTransaction, UserBrief } from '../../api/types'
import Button from '../../components/ui/Button'
import DateTimeInput from '../../components/ui/DateTimeInput'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import FilterBar from '../../components/ui/FilterBar'
import Pagination from '../../components/ui/Pagination'
import Select from '../../components/ui/Select'
import Table, { type Column } from '../../components/ui/Table'
import { useToast } from '../../components/ui/toast-context'
import TreeSelect from '../../components/ui/TreeSelect'
import UserPicker from '../../components/UserPicker'
import { useDeptTree } from '../../hooks/useDeptTree'
import { usePermission } from '../../hooks/usePermission'
import { useVehicleOptions } from '../../hooks/useVehicleOptions'
import { saveBlob } from '../../utils/download'
import { localToRFC3339 } from '../../utils/format'
import { AttributionCell, CostCell, DeviationCell, DurationCell, KwhCell, PileCell, PowerCell, ReviewCell, StatusCell, TimeCell, TxNoCell, UserCell, VehicleCell } from './cells'
import { usePileOptions } from './hooks'
import { CHARGE_STATUS_OPTIONS, REVIEW_STATUS_OPTIONS, isChargeStatus, isReviewStatus } from './style'

interface Filters {
  keyword: string
  status: ChargeStatus | ''
  review_status: ChargeReviewStatus | ''
  pile_id: string
  vehicle_id: string
  user: UserBrief | null
  dept_id: string | null
  from: string
  to: string
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '', review_status: '', pile_id: '', vehicle_id: '', user: null, dept_id: null, from: '', to: '' }
const DEFAULT_PAGE_SIZE = 20
const DEFAULT_SORT = '-start_at'

function toFilter(f: Filters): ChargeTxFilter {
  return {
    keyword: f.keyword || undefined,
    status: f.status || undefined,
    review_status: f.review_status || undefined,
    pile_id: f.pile_id || undefined,
    vehicle_id: f.vehicle_id || undefined,
    user_id: f.user?.id,
    dept_id: f.dept_id ?? undefined,
    from: localToRFC3339(f.from),
    to: localToRFC3339(f.to),
  }
}

interface TransactionsTabProps {
  onOpen: (id: string) => void
}

/** 充电记录页签：筛选 + 分页排序表格 + 导出 */
export default function TransactionsTab({ onOpen }: TransactionsTabProps) {
  const toast = useToast()
  const { can } = usePermission()
  const canExport = can('charging:export')
  const canPickVehicle = can('asset:vehicle:view')

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(DEFAULT_SORT)

  const params: ChargeTxListParams = { ...toFilter(applied), page, pageSize, sort }
  const list = useQuery({ queryKey: chargingKeys.txList(params), queryFn: () => listChargeTransactions(params), placeholderData: keepPreviousData })

  const { nodes: deptNodes, query: deptQuery } = useDeptTree()
  const { options: vehicleOptions, query: vehicleQuery } = useVehicleOptions(canPickVehicle)
  const piles = usePileOptions()

  const exportMutation = useMutation({
    mutationFn: () => exportChargeTransactions(toFilter(applied)),
    onSuccess: (file) => {
      saveBlob(file.blob, file.filename)
      toast.success('导出完成')
    },
    onError: (e) => toast.error('导出失败', errorMessage(e)),
  })

  const search = () => {
    setApplied(draft)
    setPage(1)
  }
  const reset = () => {
    setDraft(EMPTY_FILTERS)
    setApplied(EMPTY_FILTERS)
    setPage(1)
    setSort(DEFAULT_SORT)
  }

  const columns: Column<ChargeTransaction>[] = [
    { key: 'tx_no', title: '事务号', render: (t) => <TxNoCell t={t} /> },
    { key: 'pile', title: '桩 / 连接器', render: (t) => <PileCell t={t} /> },
    { key: 'user', title: '用户 / 部门', render: (t) => <UserCell t={t} /> },
    { key: 'vehicle', title: '车牌', render: (t) => <VehicleCell t={t} /> },
    { key: 'start_at', title: '开始 → 结束', sortable: true, render: (t) => <TimeCell t={t} /> },
    { key: 'duration', title: '时长', render: (t) => <DurationCell t={t} /> },
    { key: 'kwh', title: '电量', sortable: true, align: 'right', render: (t) => <KwhCell t={t} /> },
    { key: 'power', title: '功率', align: 'right', render: (t) => <PowerCell t={t} /> },
    { key: 'cost', title: '费用', sortable: true, align: 'right', render: (t) => <CostCell t={t} /> },
    { key: 'attribution', title: '归属', render: (t) => <AttributionCell t={t} /> },
    { key: 'deviation', title: '偏差', align: 'right', render: (t) => <DeviationCell t={t} /> },
    { key: 'status', title: '状态', render: (t) => <StatusCell t={t} /> },
    { key: 'review', title: '复核', render: (t) => <ReviewCell t={t} /> },
  ]

  return (
    <div className="space-y-4">
      <FilterBar
        keyword={draft.keyword}
        onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))}
        keywordPlaceholder="事务号 / 桩 / 车牌 / 用户"
        onSearch={search}
        onReset={reset}
        loading={list.isFetching}
        extra={
          canExport && (
            <Button variant="secondary" icon={Download} loading={exportMutation.isPending} onClick={() => exportMutation.mutate()}>
              导出
            </Button>
          )
        }
      >
        <div className="w-full sm:w-32">
          <Select
            options={CHARGE_STATUS_OPTIONS}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isChargeStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
        </div>
        <div className="w-full sm:w-32">
          <Select
            options={REVIEW_STATUS_OPTIONS}
            placeholder="全部复核"
            value={draft.review_status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, review_status: isReviewStatus(v) ? v : '' }))
            }}
            aria-label="复核状态"
          />
        </div>
        <div className="w-full sm:w-40">
          <Select options={piles.options} placeholder={piles.isError ? '桩数据暂不可用' : '全部桩'} value={draft.pile_id} onChange={(e) => setDraft((d) => ({ ...d, pile_id: e.target.value }))} aria-label="充电桩" />
        </div>
        {canPickVehicle && (
          <div className="w-full sm:w-44">
            <Select options={vehicleOptions} placeholder={vehicleQuery.isError ? '车辆数据暂不可用' : '全部车辆'} value={draft.vehicle_id} onChange={(e) => setDraft((d) => ({ ...d, vehicle_id: e.target.value }))} aria-label="车辆" />
          </div>
        )}
        <div className="w-full sm:w-40">
          <UserPicker value={draft.user} onChange={(u) => setDraft((d) => ({ ...d, user: u }))} placeholder="全部用户" />
        </div>
        <div className="w-full sm:w-40">
          <TreeSelect nodes={deptNodes} value={draft.dept_id} onChange={(v) => setDraft((d) => ({ ...d, dept_id: v }))} placeholder="全部部门" loading={deptQuery.isLoading} emptyText={deptQuery.isError ? '部门数据暂不可用' : '暂无部门'} />
        </div>
        <div className="flex w-full items-center gap-1 sm:w-auto">
          <DateTimeInput value={draft.from} onChange={(v) => setDraft((d) => ({ ...d, from: v }))} aria-label="开始时间起" className="sm:w-44" />
          <span className="text-xs text-ink-faint">至</span>
          <DateTimeInput value={draft.to} onChange={(v) => setDraft((d) => ({ ...d, to: v }))} aria-label="开始时间止" className="sm:w-44" />
        </div>
      </FilterBar>

      <div className="card p-4">
        {list.isError ? (
          <ErrorState message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
        ) : (
          <>
            <Table
              columns={columns}
              data={list.data?.items}
              rowKey="id"
              loading={list.isFetching}
              sort={sort}
              onSortChange={(s) => {
                setSort(s)
                setPage(1)
              }}
              onRowClick={(t) => onOpen(t.id)}
              actions={{ render: (t) => <Button variant="ghost" size="sm" icon={Eye} className="!px-2" title="查看" aria-label="查看" onClick={() => onOpen(t.id)} />, width: 60 }}
              empty={<Empty title="没有匹配的充电记录" description="桩上报 StartTransaction 后会显示在这里；调整筛选条件再试" />}
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
    </div>
  )
}
