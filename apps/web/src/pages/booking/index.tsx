import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, Pencil, PhoneCall, Play, UserPlus, X } from 'lucide-react'
import { useState } from 'react'
import { bookingKeys, cancelBooking, completeBooking, departBooking, listBookings, type BookingFilter, type BookingListParams } from '../../api/bookings'
import { errorMessage } from '../../api/client'
import type { Booking, BookingSource, BookingStatus } from '../../api/types'
import Can from '../../components/Can'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import ConfirmDialog from '../../components/ui/ConfirmDialog'
import DateTimeInput from '../../components/ui/DateTimeInput'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import FilterBar from '../../components/ui/FilterBar'
import PageHeader from '../../components/ui/PageHeader'
import Pagination from '../../components/ui/Pagination'
import Select from '../../components/ui/Select'
import Table, { type Column } from '../../components/ui/Table'
import Tabs, { type TabItem } from '../../components/ui/Tabs'
import { useToast } from '../../components/ui/toast-context'
import TreeSelect from '../../components/ui/TreeSelect'
import { useDeptTree } from '../../hooks/useDeptTree'
import { usePermission } from '../../hooks/usePermission'
import { useVehicleOptions } from '../../hooks/useVehicleOptions'
import { formatTimeRange, localToRFC3339, text } from '../../utils/format'
import BookingDrawer, { type BookingDrawerState } from './BookingDrawer'
import { BOOKING_SOURCE_BADGE, BOOKING_SOURCE_LABEL, BOOKING_STATUS_BADGE, BOOKING_STATUS_LABEL, BOOKING_STATUS_OPTIONS, isBookingStatus } from './style'

interface Filters {
  keyword: string
  status: BookingStatus | ''
  dept_id: string | null
  vehicle_id: string
  /** 本地时间 YYYY-MM-DDTHH:mm */
  from: string
  to: string
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '', dept_id: null, vehicle_id: '', from: '', to: '' }
const DEFAULT_PAGE_SIZE = 20
const DEFAULT_SORT = '-created_at'

/** 页签：全部 / 按来源 */
type Tab = 'all' | BookingSource

const TABS: TabItem<Tab>[] = [
  { key: 'all', label: '全部' },
  { key: 'phone', label: BOOKING_SOURCE_LABEL.phone },
  { key: 'direct', label: BOOKING_SOURCE_LABEL.direct },
]

function toFilter(f: Filters, tab: Tab): BookingFilter {
  return {
    source: tab === 'all' ? undefined : tab,
    keyword: f.keyword || undefined,
    status: f.status || undefined,
    dept_id: f.dept_id ?? undefined,
    vehicle_id: f.vehicle_id || undefined,
    from: localToRFC3339(f.from),
    to: localToRFC3339(f.to),
  }
}

export default function BookingPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const canCreate = can('booking:create')
  const canUpdate = can('booking:update')
  const canCancel = can('booking:cancel')

  const [tab, setTab] = useState<Tab>('all')
  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(DEFAULT_SORT)
  const [drawer, setDrawer] = useState<BookingDrawerState>(null)
  const [cancelTarget, setCancelTarget] = useState<Booking | null>(null)

  const params: BookingListParams = { ...toFilter(applied, tab), page, pageSize, sort }
  const list = useQuery({ queryKey: bookingKeys.list(params), queryFn: () => listBookings(params), placeholderData: keepPreviousData })

  const { nodes: deptNodes, query: deptQuery } = useDeptTree()
  const { options: vehicleOptions, query: vehicleQuery } = useVehicleOptions(can('asset:vehicle:view'))

  const invalidate = () => queryClient.invalidateQueries({ queryKey: bookingKeys.all })

  const depart = useMutation({
    mutationFn: (id: string) => departBooking(id),
    onSuccess: (b) => {
      toast.success(`${b.booking_no} 已出车`)
      void invalidate()
    },
    onError: (e) => toast.error('操作失败', errorMessage(e)),
  })
  const complete = useMutation({
    mutationFn: (id: string) => completeBooking(id),
    onSuccess: (b) => {
      toast.success(`${b.booking_no} 已完成`)
      void invalidate()
    },
    onError: (e) => toast.error('操作失败', errorMessage(e)),
  })
  const cancel = useMutation({
    mutationFn: (id: string) => cancelBooking(id),
    onSuccess: (b) => {
      toast.success(`${b.booking_no} 已取消`)
      setCancelTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('取消失败', errorMessage(e)),
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

  const columns: Column<Booking>[] = [
    {
      key: 'booking_no',
      title: '单号',
      sortable: true,
      render: (b) => (
        <div>
          <div className="font-mono text-xs font-medium text-slate-800">{b.booking_no}</div>
          <Badge className="mt-0.5" color={BOOKING_SOURCE_BADGE[b.source]}>
            {BOOKING_SOURCE_LABEL[b.source]}
          </Badge>
        </div>
      ),
    },
    {
      key: 'contact',
      title: '联系人',
      render: (b) => (
        <div>
          <div className="text-slate-800">{b.contact_name}</div>
          <div className="text-xs text-slate-400">{text(b.contact_phone)}</div>
        </div>
      ),
    },
    {
      key: 'passenger',
      title: '用车人',
      render: (b) =>
        b.passenger ? (
          <div>
            <div className="text-slate-800">{b.passenger.name}</div>
            <div className="text-xs text-slate-400">{text(b.dept_name)}</div>
          </div>
        ) : (
          <span className="text-xs text-slate-300">—</span>
        ),
    },
    {
      key: 'reserve_start',
      title: '预约时间',
      sortable: true,
      render: (b) => <span className="whitespace-nowrap text-xs">{formatTimeRange(b.reserve_start, b.reserve_end)}</span>,
    },
    {
      key: 'vehicle',
      title: '车辆',
      width: 110,
      render: (b) => (b.vehicle ? <span className="whitespace-nowrap">{b.vehicle.plate_no}</span> : <span className="text-xs text-amber-600">待派车</span>),
    },
    {
      key: 'route',
      title: '出发地 → 目的地',
      render: (b) => (
        <div className="max-w-[18rem]">
          <div className="truncate text-slate-800" title={`${b.origin} → ${b.destination}`}>
            {b.origin} → {b.destination}
          </div>
          {b.purpose && (
            <div className="truncate text-xs text-slate-400" title={b.purpose}>
              {b.purpose}
            </div>
          )}
        </div>
      ),
    },
    { key: 'status', title: '状态', width: 88, sortable: true, render: (b) => <Badge color={BOOKING_STATUS_BADGE[b.status]}>{BOOKING_STATUS_LABEL[b.status]}</Badge> },
    {
      key: 'created_by',
      title: '记录人',
      render: (b) => (b.created_by ? <span className="text-xs text-slate-700">{b.created_by.name}</span> : <span className="text-xs text-slate-300">—</span>),
    },
  ]

  const renderActions = (b: Booking) => (
    <div className="flex items-center justify-end gap-0.5">
      {canUpdate && b.status === 'reserved' && (
        <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑 / 改派" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', booking: b })} />
      )}
      {canUpdate && b.status === 'reserved' && (
        <Button
          variant="ghost"
          size="sm"
          icon={Play}
          className="!px-2 text-blue-600 hover:bg-blue-50"
          title={b.vehicle ? '确认出车' : '请先派车'}
          aria-label="确认出车"
          disabled={!b.vehicle || depart.isPending}
          onClick={() => depart.mutate(b.id)}
        />
      )}
      {canUpdate && b.status === 'departed' && (
        <Button
          variant="ghost"
          size="sm"
          icon={CheckCircle2}
          className="!px-2 text-emerald-600 hover:bg-emerald-50"
          title="完成"
          aria-label="完成"
          disabled={complete.isPending}
          onClick={() => complete.mutate(b.id)}
        />
      )}
      {canCancel && b.status === 'reserved' && (
        <Button variant="ghost" size="sm" icon={X} className="!px-2 text-red-500 hover:bg-red-50" title="取消" aria-label="取消" onClick={() => setCancelTarget(b)} />
      )}
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="预约派车"
        description="电话预约与直接预约：记录联系人、预约时段、派车与出发地/目的地；登记即占用车辆，不走审批"
        extra={
          <Can perm="booking:create">
            <Button variant="secondary" icon={PhoneCall} onClick={() => setDrawer({ mode: 'create', source: 'phone' })}>
              电话预约
            </Button>
            <Button icon={UserPlus} onClick={() => setDrawer({ mode: 'create', source: 'direct' })}>
              直接预约
            </Button>
          </Can>
        }
      />

      <div className="card px-4 pt-1">
        <Tabs
          items={TABS}
          value={tab}
          onChange={(k) => {
            setTab(k)
            setPage(1)
          }}
        />
      </div>

      <FilterBar
        keyword={draft.keyword}
        onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))}
        keywordPlaceholder="单号 / 联系人 / 电话 / 出发地 / 目的地"
        onSearch={search}
        onReset={reset}
        loading={list.isFetching}
      >
        <div className="w-full sm:w-32">
          <Select
            options={BOOKING_STATUS_OPTIONS}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isBookingStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
        </div>
        <div className="w-full sm:w-48">
          <TreeSelect
            nodes={deptNodes}
            value={draft.dept_id}
            onChange={(v) => setDraft((d) => ({ ...d, dept_id: v }))}
            placeholder="全部部门"
            loading={deptQuery.isLoading}
            emptyText={deptQuery.isError ? '部门数据暂不可用' : '暂无部门'}
          />
        </div>
        {can('asset:vehicle:view') && (
          <div className="w-full sm:w-48">
            <Select
              options={vehicleOptions}
              placeholder={vehicleQuery.isError ? '车辆数据暂不可用' : '全部车辆'}
              value={draft.vehicle_id}
              onChange={(e) => setDraft((d) => ({ ...d, vehicle_id: e.target.value }))}
              aria-label="车辆"
            />
          </div>
        )}
        <div className="flex w-full items-center gap-1 sm:w-auto">
          <DateTimeInput value={draft.from} onChange={(v) => setDraft((d) => ({ ...d, from: v }))} aria-label="预约开始起" className="sm:w-44" />
          <span className="text-xs text-slate-400">至</span>
          <DateTimeInput value={draft.to} onChange={(v) => setDraft((d) => ({ ...d, to: v }))} aria-label="预约开始止" className="sm:w-44" />
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
              onRowClick={canUpdate ? (b) => { if (b.status === 'reserved') setDrawer({ mode: 'edit', booking: b }) } : undefined}
              actions={canUpdate || canCancel ? { render: renderActions, width: 130 } : undefined}
              empty={
                <Empty
                  title="还没有预约"
                  description={canCreate ? '点击右上角「电话预约」或「直接预约」登记一条' : '调整筛选条件后再试'}
                />
              }
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

      <BookingDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
        }}
      />
      <ConfirmDialog
        open={cancelTarget !== null}
        danger
        title={`取消预约「${cancelTarget?.booking_no ?? ''}」？`}
        description={`${cancelTarget?.contact_name ?? ''} 的用车预约将被取消，占用的车辆时段随之释放。`}
        confirmText="取消预约"
        loading={cancel.isPending}
        onConfirm={() => {
          if (cancelTarget) cancel.mutate(cancelTarget.id)
        }}
        onCancel={() => setCancelTarget(null)}
      />
    </div>
  )
}
