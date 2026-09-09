import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { Eye, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { errorMessage } from '../../../api/client'
import type { Vehicle, VehicleStatus } from '../../../api/types'
import { deleteVehicle, listVehicles, vehicleKeys, type VehicleFilter, type VehicleListParams } from '../../../api/vehicles'
import Can from '../../../components/Can'
import { VEHICLE_STATUS_BADGE, VEHICLE_STATUS_LABEL, VEHICLE_STATUS_OPTIONS, formatKm, formatPercent, isVehicleStatus, socBarClass, socTextClass } from '../../../components/map/vehicleStyle'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import FilterBar from '../../../components/ui/FilterBar'
import PageHeader from '../../../components/ui/PageHeader'
import Pagination from '../../../components/ui/Pagination'
import Select from '../../../components/ui/Select'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import TreeSelect from '../../../components/ui/TreeSelect'
import { useDeptTree } from '../../../hooks/useDeptTree'
import { usePermission } from '../../../hooks/usePermission'
import { text } from '../../../utils/format'
import { onlineOptions, parseBool } from './schemas'
import VehicleDetailDrawer from './VehicleDetailDrawer'
import VehicleDrawer, { type VehicleDrawerState } from './VehicleDrawer'

interface Filters {
  keyword: string
  status: VehicleStatus | ''
  dept_id: string | null
  online: string
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '', dept_id: null, online: '' }
const DEFAULT_PAGE_SIZE = 20

function toFilter(f: Filters): VehicleFilter {
  return {
    keyword: f.keyword || undefined,
    status: f.status || undefined,
    dept_id: f.dept_id ?? undefined,
    online: parseBool(f.online),
  }
}

function OnlineDot({ online }: { online: boolean | undefined }) {
  if (online === undefined) return <span className="text-slate-300">—</span>
  return (
    <span className={clsx('inline-flex items-center gap-1 text-xs', online ? 'text-emerald-600' : 'text-slate-400')}>
      <span className={clsx('inline-block h-1.5 w-1.5 rounded-full', online ? 'bg-emerald-500' : 'bg-slate-300')} />
      {online ? '在线' : '离线'}
    </span>
  )
}

export default function VehiclesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const [searchParams, setSearchParams] = useSearchParams()

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)

  const [drawer, setDrawer] = useState<VehicleDrawerState>(null)
  const [deleteTarget, setDeleteTarget] = useState<Vehicle | null>(null)
  // 详情由 URL ?id= 驱动（通知跳转 /assets/vehicles?id= 直接打开）
  const detailId = searchParams.get('id')
  const openDetail = (id: string | null) => {
    const next = new URLSearchParams(searchParams)
    if (id) next.set('id', id)
    else next.delete('id')
    setSearchParams(next, { replace: true })
  }

  const params: VehicleListParams = { ...toFilter(applied), page, pageSize, sort }
  const list = useQuery({
    queryKey: vehicleKeys.list(params),
    queryFn: () => listVehicles(params),
    placeholderData: keepPreviousData,
  })

  const { nodes: deptNodes, query: deptQuery } = useDeptTree()

  const invalidate = () => queryClient.invalidateQueries({ queryKey: vehicleKeys.all })

  const remove = useMutation({
    mutationFn: (id: string) => deleteVehicle(id),
    onSuccess: () => {
      toast.success('车辆已删除')
      setDeleteTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  const search = () => {
    setApplied(draft)
    setPage(1)
  }
  const reset = () => {
    setDraft(EMPTY_FILTERS)
    setApplied(EMPTY_FILTERS)
    setPage(1)
    setSort(undefined)
  }

  const canUpdate = can('asset:vehicle:update')
  const canDelete = can('asset:vehicle:delete')

  const columns: Column<Vehicle>[] = [
    {
      key: 'plate_no',
      title: '车牌',
      sortable: true,
      render: (v) => (
        <div>
          <div className="font-medium text-slate-800">{v.plate_no}</div>
          <div className="text-xs text-slate-400">{[v.brand, v.model].filter(Boolean).join(' ') || '—'}</div>
        </div>
      ),
    },
    {
      key: 'status',
      title: '状态',
      sortable: true,
      render: (v) => {
        const s = v.live?.status ?? v.status
        return <Badge color={VEHICLE_STATUS_BADGE[s]}>{VEHICLE_STATUS_LABEL[s]}</Badge>
      },
    },
    { key: 'online', title: '在线', render: (v) => <OnlineDot online={v.live?.online} /> },
    {
      key: 'soc',
      title: '电量',
      width: 140,
      render: (v) => {
        const soc = v.live?.soc ?? v.soc ?? null
        if (soc === null) return <span className="text-slate-300">—</span>
        return (
          <div className="flex items-center gap-2">
            <div className="h-1.5 w-16 overflow-hidden rounded-full bg-slate-100">
              <div className={clsx('h-full rounded-full transition-all', socBarClass(soc))} style={{ width: `${Math.max(0, Math.min(100, soc))}%` }} />
            </div>
            <span className={clsx('font-mono text-xs', socTextClass(soc))}>{formatPercent(soc)}</span>
          </div>
        )
      },
    },
    { key: 'range_km', title: '续航', render: (v) => <span className="text-xs">{formatKm(v.live?.range_km)}</span> },
    { key: 'odometer_km', title: '里程', sortable: true, render: (v) => <span className="text-xs">{formatKm(v.live?.odometer_km ?? v.odometer_km)}</span> },
    { key: 'home_dept_name', title: '归属部门', render: (v) => text(v.home_dept_name) },
    {
      key: 'device_serial',
      title: '设备',
      render: (v) => (v.device_serial ? <span className="font-mono text-xs">{v.device_serial}</span> : <span className="text-xs text-slate-400">未绑定</span>),
    },
  ]

  const renderActions = (v: Vehicle) => (
    <div className="flex items-center justify-end gap-0.5">
      <Button variant="ghost" size="sm" icon={Eye} className="!px-2" title="详情" aria-label="详情" onClick={() => openDetail(v.id)} />
      {canUpdate && <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', vehicle: v })} />}
      {canDelete && (
        <Button
          variant="ghost"
          size="sm"
          icon={Trash2}
          className="!px-2 text-red-500 hover:bg-red-50 hover:text-red-600"
          title="删除"
          aria-label="删除"
          onClick={() => setDeleteTarget(v)}
        />
      )}
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="车辆档案"
        description="车辆基础信息与实时状态；状态、电量、里程由网关遥测实时更新"
        extra={
          <Can perm="asset:vehicle:create">
            <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
              新建车辆
            </Button>
          </Can>
        }
      />

      <FilterBar keyword={draft.keyword} onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))} keywordPlaceholder="车牌 / VIN / 品牌 / 型号" onSearch={search} onReset={reset} loading={list.isFetching}>
        <div className="w-full sm:w-36">
          <Select
            options={VEHICLE_STATUS_OPTIONS}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isVehicleStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
        </div>
        <div className="w-full sm:w-52">
          <TreeSelect
            nodes={deptNodes}
            value={draft.dept_id}
            onChange={(v) => setDraft((d) => ({ ...d, dept_id: v }))}
            placeholder="全部部门"
            loading={deptQuery.isLoading}
            emptyText={deptQuery.isError ? '部门数据暂不可用' : '暂无部门'}
          />
        </div>
        <div className="w-full sm:w-32">
          <Select options={onlineOptions} placeholder="在线 / 离线" value={draft.online} onChange={(e) => setDraft((d) => ({ ...d, online: e.target.value }))} aria-label="在线状态" />
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
              onRowClick={(v) => openDetail(v.id)}
              actions={{ render: renderActions, width: 130 }}
              empty={<Empty title="没有匹配的车辆" description="调整筛选条件，或新建车辆" />}
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

      <VehicleDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
        }}
        deptNodes={deptNodes}
        deptLoading={deptQuery.isLoading}
        deptUnavailable={deptQuery.isError}
      />
      <VehicleDetailDrawer
        id={detailId}
        onClose={() => openDetail(null)}
        onEdit={
          canUpdate
            ? (v) => {
                openDetail(null)
                setDrawer({ mode: 'edit', vehicle: v })
              }
            : undefined
        }
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除车辆「${deleteTarget?.plate_no ?? ''}」？`}
        description="软删除；在途或有未完成申请的车辆无法删除，已绑定的网关将自动解绑。"
        confirmText="删除"
        loading={remove.isPending}
        onConfirm={() => {
          if (deleteTarget) remove.mutate(deleteTarget.id)
        }}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
