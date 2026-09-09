import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { AlertTriangle, Download, Eye, Lightbulb, Route } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { errorMessage } from '../../api/client'
import { exportTrips, getTripSummary, listTrips, tripKeys, type TripFilter, type TripListParams, type TripScope } from '../../api/trips'
import type { Trip, TripStatus, TripType, UserBrief } from '../../api/types'
import { formatKm, formatSpeed } from '../../components/map/vehicleStyle'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import DateTimeInput from '../../components/ui/DateTimeInput'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import FilterBar from '../../components/ui/FilterBar'
import PageHeader from '../../components/ui/PageHeader'
import Pagination from '../../components/ui/Pagination'
import Select, { type SelectOption } from '../../components/ui/Select'
import Table, { type Column } from '../../components/ui/Table'
import Tabs, { type TabItem } from '../../components/ui/Tabs'
import { useToast } from '../../components/ui/toast-context'
import TreeSelect from '../../components/ui/TreeSelect'
import UserPicker from '../../components/UserPicker'
import { useDeptTree } from '../../hooks/useDeptTree'
import { usePermission } from '../../hooks/usePermission'
import { useVehicleOptions } from '../../hooks/useVehicleOptions'
import { saveBlob } from '../../utils/download'
import { formatMinutes, formatNumber, formatTimeRange, localToRFC3339, text } from '../../utils/format'
import { TRIP_TYPE_BADGE, TRIP_TYPE_LABEL, TRIP_TYPE_OPTIONS, isTripType } from '../approval/style'
import { ROOF_SIGN_BADGE, ROOF_SIGN_LABEL, TRIP_STATUS_BADGE, TRIP_STATUS_LABEL, TRIP_STATUS_OPTIONS, isTripStatus } from './style'
import TripDetailDrawer from './TripDetailDrawer'

interface Filters {
  keyword: string
  status: TripStatus | ''
  trip_type: TripType | ''
  vehicle_id: string
  driver: UserBrief | null
  dept_id: string | null
  from: string
  to: string
  /** '' | 'true' */
  deviation: string
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '', trip_type: '', vehicle_id: '', driver: null, dept_id: null, from: '', to: '', deviation: '' }
const DEFAULT_PAGE_SIZE = 20
const DEFAULT_SORT = '-start_at'

const DEVIATION_OPTIONS: SelectOption[] = [{ value: 'true', label: '仅偏离路线' }]

function toFilter(f: Filters, scope: TripScope): TripFilter {
  return {
    scope,
    keyword: f.keyword || undefined,
    status: f.status || undefined,
    trip_type: f.trip_type || undefined,
    vehicle_id: f.vehicle_id || undefined,
    driver_id: f.driver?.id,
    dept_id: f.dept_id ?? undefined,
    from: localToRFC3339(f.from),
    to: localToRFC3339(f.to),
    deviation: f.deviation === 'true' ? true : undefined,
  }
}

function SummaryBar() {
  const summary = useQuery({ queryKey: tripKeys.summary(), queryFn: () => getTripSummary(), staleTime: 30_000, refetchInterval: 60_000 })
  const s = summary.data
  const items: Array<{ label: string; value: string; cls?: string }> = [
    { label: '今日行程', value: s ? String(s.trips) : '—' },
    { label: '进行中', value: s ? String(s.ongoing) : '—', cls: s && s.ongoing > 0 ? 'text-blue-600' : undefined },
    { label: '总里程', value: s ? `${s.distance_km.toFixed(1)} km` : '—' },
    { label: '总耗电', value: s ? `${s.energy_kwh.toFixed(1)} kWh` : '—', cls: 'text-amber-600' },
    { label: '公务行程', value: s && typeof s.official_trips === 'number' ? String(s.official_trips) : '—' },
    { label: '偏离行程', value: s && typeof s.deviation_trips === 'number' ? String(s.deviation_trips) : '—', cls: s && (s.deviation_trips ?? 0) > 0 ? 'text-red-600' : undefined },
  ]
  return (
    <div className="card flex flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-500">
        <Route size={14} className="text-brand-600" />
        {s ? `${s.date} 汇总` : '当日汇总'}
      </span>
      {items.map((it) => (
        <div key={it.label} className="flex items-baseline gap-1.5">
          <span className={clsx('text-lg font-semibold', it.cls ?? 'text-slate-800')}>{it.value}</span>
          <span className="text-xs text-slate-400">{it.label}</span>
        </div>
      ))}
      {summary.isError && <span className="text-xs text-amber-600">汇总暂不可用：{errorMessage(summary.error)}</span>}
    </div>
  )
}

export default function TripsPage() {
  const toast = useToast()
  const { can } = usePermission()
  const [searchParams, setSearchParams] = useSearchParams()
  const canAll = can(['trip:manage', 'trip:export'])
  const canExport = can('trip:export')
  const canPickDriver = can('trip:manage') || can('trip:export')
  const canPickVehicle = can('asset:vehicle:view')

  const rawScope = searchParams.get('scope')
  // 车队管理员/财务默认看"全部"：他们通常不亲自驾驶，落在"我的行程"会是一片空白
  const defaultScope: TripScope = canAll ? 'all' : 'mine'
  const scope: TripScope = rawScope === 'all' && canAll ? 'all' : rawScope === 'mine' ? 'mine' : defaultScope
  const detailId = searchParams.get('id')
  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set(key, value)
    else next.delete(key)
    setSearchParams(next, { replace: true })
  }

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(DEFAULT_SORT)

  const changeScope = (k: TripScope) => {
    setParam('scope', k === defaultScope ? null : k)
    setPage(1)
  }

  const params: TripListParams = { ...toFilter(applied, scope), page, pageSize, sort }
  const list = useQuery({ queryKey: tripKeys.list(params), queryFn: () => listTrips(params), placeholderData: keepPreviousData })

  const { nodes: deptNodes, query: deptQuery } = useDeptTree()
  const { options: vehicleOptions, query: vehicleQuery } = useVehicleOptions(canPickVehicle)

  const exportMutation = useMutation({
    mutationFn: () => exportTrips(toFilter(applied, scope)),
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

  const tabs = useMemo<TabItem<TripScope>[]>(() => [{ key: 'mine', label: '我的行程' }, ...(canAll ? [{ key: 'all' as const, label: '全部' }] : [])], [canAll])

  const columns: Column<Trip>[] = [
    {
      key: 'trip_no',
      title: '行程号',
      render: (t) => (
        <span className="inline-flex items-center gap-1.5 font-mono text-xs font-medium text-slate-800">
          {t.status === 'ongoing' && <span className="inline-block h-1.5 w-1.5 rounded-full bg-blue-500 pulse-dot" aria-label="进行中" />}
          {t.trip_no}
        </span>
      ),
    },
    {
      key: 'vehicle',
      title: '车辆',
      render: (t) => (
        <div>
          <div className="text-slate-800">{t.vehicle.plate_no}</div>
          <div className="text-xs text-slate-400">{text(t.vehicle.model)}</div>
        </div>
      ),
    },
    {
      key: 'driver',
      title: '驾驶员',
      render: (t) => (
        <div>
          <div className="text-slate-800">{t.driver?.name ?? <span className="text-slate-400">未识别</span>}</div>
          <div className="text-xs text-slate-400">{text(t.driver?.dept_name)}</div>
        </div>
      ),
    },
    { key: 'trip_type', title: '类型', width: 88, render: (t) => <Badge color={TRIP_TYPE_BADGE[t.trip_type]}>{TRIP_TYPE_LABEL[t.trip_type]}</Badge> },
    { key: 'start_at', title: '开始 → 结束', sortable: true, render: (t) => <span className="whitespace-nowrap text-xs">{formatTimeRange(t.start_at, t.end_at)}</span> },
    { key: 'duration_min', title: '时长', render: (t) => <span className="whitespace-nowrap text-xs">{formatMinutes(t.duration_min)}</span> },
    { key: 'distance_km', title: '里程', sortable: true, align: 'right', render: (t) => <span className="whitespace-nowrap text-xs">{formatKm(t.distance_km, 1)}</span> },
    {
      key: 'energy_kwh',
      title: '耗电',
      sortable: true,
      align: 'right',
      render: (t) => (
        <div className="whitespace-nowrap text-right text-xs">
          <div>{formatNumber(t.energy_kwh, 1, 'kWh')}</div>
          {typeof t.energy_per_100km === 'number' && <div className="text-[11px] text-slate-400">{t.energy_per_100km.toFixed(1)} /100km</div>}
        </div>
      ),
    },
    { key: 'max_speed', title: '最高速', sortable: true, align: 'right', render: (t) => <span className={clsx('whitespace-nowrap text-xs', typeof t.max_speed === 'number' && t.max_speed > 100 && 'font-medium text-red-600')}>{formatSpeed(t.max_speed)}</span> },
    {
      key: 'harsh',
      title: '急加/减速',
      align: 'center',
      render: (t) => (
        <span className={clsx('text-xs', t.harsh_accel + t.harsh_brake > 8 ? 'text-red-600' : 'text-slate-600')}>
          {t.harsh_accel} / {t.harsh_brake}
        </span>
      ),
    },
    {
      key: 'roof_sign_status',
      title: '灯牌',
      render: (t) => (
        <Badge color={ROOF_SIGN_BADGE[t.roof_sign_status]} icon={t.roof_sign_status === 'on' ? Lightbulb : undefined}>
          {ROOF_SIGN_LABEL[t.roof_sign_status]}
        </Badge>
      ),
    },
    {
      key: 'deviation_flag',
      title: '偏离',
      align: 'center',
      render: (t) =>
        t.deviation_flag ? (
          <span className="inline-flex items-center gap-0.5 text-xs font-medium text-red-600" title={typeof t.deviation_max_m === 'number' ? `最大偏离 ${Math.round(t.deviation_max_m)} m` : undefined}>
            <AlertTriangle size={12} /> 偏离
          </span>
        ) : (
          <span className="text-xs text-slate-300">—</span>
        ),
    },
    { key: 'status', title: '状态', render: (t) => <Badge color={TRIP_STATUS_BADGE[t.status]}>{TRIP_STATUS_LABEL[t.status]}</Badge> },
  ]

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="行程管理"
        description="行程记录、轨迹回放与驾驶事件；进行中的行程实时刷新"
        extra={
          canExport && (
            <Button variant="secondary" icon={Download} loading={exportMutation.isPending} onClick={() => exportMutation.mutate()}>
              导出
            </Button>
          )
        }
      />

      <SummaryBar />

      {tabs.length > 1 && (
        <div className="card px-4 pt-1">
          <Tabs items={tabs} value={scope} onChange={changeScope} />
        </div>
      )}

      <FilterBar keyword={draft.keyword} onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))} keywordPlaceholder="行程号 / 车牌 / 驾驶员 / 事由" onSearch={search} onReset={reset} loading={list.isFetching}>
        <div className="w-full sm:w-32">
          <Select
            options={TRIP_STATUS_OPTIONS}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isTripStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
        </div>
        <div className="w-full sm:w-32">
          <Select
            options={TRIP_TYPE_OPTIONS}
            placeholder="全部类型"
            value={draft.trip_type}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, trip_type: isTripType(v) ? v : '' }))
            }}
            aria-label="类型"
          />
        </div>
        {canPickVehicle && (
          <div className="w-full sm:w-48">
            <Select options={vehicleOptions} placeholder={vehicleQuery.isError ? '车辆数据暂不可用' : '全部车辆'} value={draft.vehicle_id} onChange={(e) => setDraft((d) => ({ ...d, vehicle_id: e.target.value }))} aria-label="车辆" />
          </div>
        )}
        {canPickDriver && (
          <div className="w-full sm:w-44">
            <UserPicker value={draft.driver} onChange={(u) => setDraft((d) => ({ ...d, driver: u }))} placeholder="全部驾驶员" />
          </div>
        )}
        <div className="w-full sm:w-44">
          <TreeSelect nodes={deptNodes} value={draft.dept_id} onChange={(v) => setDraft((d) => ({ ...d, dept_id: v }))} placeholder="全部部门" loading={deptQuery.isLoading} emptyText={deptQuery.isError ? '部门数据暂不可用' : '暂无部门'} />
        </div>
        <div className="flex w-full items-center gap-1 sm:w-auto">
          <DateTimeInput value={draft.from} onChange={(v) => setDraft((d) => ({ ...d, from: v }))} aria-label="开始时间起" className="sm:w-44" />
          <span className="text-xs text-slate-400">至</span>
          <DateTimeInput value={draft.to} onChange={(v) => setDraft((d) => ({ ...d, to: v }))} aria-label="开始时间止" className="sm:w-44" />
        </div>
        <div className="w-full sm:w-32">
          <Select options={DEVIATION_OPTIONS} placeholder="全部路线" value={draft.deviation} onChange={(e) => setDraft((d) => ({ ...d, deviation: e.target.value }))} aria-label="偏离" />
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
              onRowClick={(t) => setParam('id', t.id)}
              actions={{ render: (t) => <Button variant="ghost" size="sm" icon={Eye} className="!px-2" title="查看" aria-label="查看" onClick={() => setParam('id', t.id)} />, width: 60 }}
              empty={<Empty title={scope === 'mine' ? '还没有你驾驶的行程' : '没有匹配的行程'} description={scope === 'mine' ? '刷卡取车或手动开始行程后会显示在这里' : '调整筛选条件后再试'} />}
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

      <TripDetailDrawer id={detailId} onClose={() => setParam('id', null)} />
    </div>
  )
}
