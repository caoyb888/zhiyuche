import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { errorMessage } from '../../../api/client'
import { deletePile, listAllPiles, listPiles, pileKeys, type PileFilter, type PileListParams } from '../../../api/piles'
import type { Pile, PileStatus, PileType } from '../../../api/types'
import Can from '../../../components/Can'
import MapView from '../../../components/map/MapView'
import type { MapMarker } from '../../../components/map/types'
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
import { usePermission } from '../../../hooks/usePermission'
import { formatDateTime, text } from '../../../utils/format'
import { toLngLat } from '../../../utils/geo'
import PileDrawer, { type PileDrawerState } from './PileDrawer'
import { isPileStatus, isPileType, pileStatusColor, pileStatusHex, pileStatusLabel, pileStatusOptions, pileTypeLabel, pileTypeOptions } from './schemas'

interface Filters {
  keyword: string
  type: PileType | ''
  status: PileStatus | ''
}

const EMPTY_FILTERS: Filters = { keyword: '', type: '', status: '' }
const DEFAULT_PAGE_SIZE = 20

function toFilter(f: Filters): PileFilter {
  return { keyword: f.keyword || undefined, type: f.type || undefined, status: f.status || undefined }
}

const LEGEND: PileStatus[] = ['available', 'charging', 'offline', 'faulted', 'disabled']

/** 列表顶部：全部桩位置小地图 */
function PilesMap({ selectedId, onSelect }: { selectedId: string | null; onSelect: (id: string | null) => void }) {
  const all = useQuery({ queryKey: pileKeys.map, queryFn: listAllPiles, staleTime: 60_000 })
  const piles = useMemo(() => all.data ?? [], [all.data])
  const located = useMemo(() => piles.filter((p) => toLngLat(p.lng, p.lat) !== null), [piles])
  const markers = useMemo<MapMarker[]>(
    () =>
      located.map((p) => {
        const lnglat = toLngLat(p.lng, p.lat) as [number, number]
        return {
          id: p.id,
          lnglat,
          icon: 'pile',
          label: p.name,
          color: pileStatusHex[p.status],
          title: `${p.pile_code} · ${pileTypeLabel[p.type]} ${p.power_kw}kW · ${pileStatusLabel[p.status]}`,
          selected: p.id === selectedId,
          zIndex: p.id === selectedId ? 10 : 1,
          onClick: (m) => onSelect(m.id),
        }
      }),
    [located, selectedId, onSelect],
  )
  const sel = selectedId ? piles.find((p) => p.id === selectedId) ?? null : null

  if (all.isError) {
    return (
      <div className="card p-4">
        <ErrorState size="sm" message={errorMessage(all.error)} onRetry={() => void all.refetch()} />
      </div>
    )
  }

  return (
    <div className="card p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-ink">桩位分布</h3>
        <span className="text-xs text-ink-faint">
          {all.isPending ? '加载中…' : `${located.length} / ${piles.length} 个桩有坐标`}
        </span>
      </div>
      <MapView
        height={260}
        markers={markers}
        fitKey={located.length}
        onClick={() => onSelect(null)}
        hint={!all.isPending && located.length === 0 ? '暂无带坐标的充电桩' : undefined}
        overlay={
          <>
            <div className="absolute bottom-8 left-2 flex flex-wrap items-center gap-x-2.5 gap-y-1 rounded-md bg-surface-2/90 px-2 py-1 text-[11px] text-ink shadow-sm">
              {LEGEND.map((s) => (
                <span key={s} className="inline-flex items-center gap-1">
                  <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ background: pileStatusHex[s] }} />
                  {pileStatusLabel[s]}
                </span>
              ))}
            </div>
            {sel && (
              <div className="absolute right-11 top-2 w-56 rounded-xl border border-line bg-surface-2/95 p-3 text-xs shadow-lg backdrop-blur">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-semibold text-ink-strong">{sel.name}</span>
                  <Badge color={pileStatusColor[sel.status]}>{pileStatusLabel[sel.status]}</Badge>
                </div>
                <div className="mt-1 font-mono text-ink-muted">{sel.pile_code}</div>
                <div className="mt-1 text-ink">
                  {pileTypeLabel[sel.type]} · {sel.power_kw} kW · {sel.connector_count} 枪
                </div>
                {sel.location && <div className="mt-1 truncate text-ink-muted">{sel.location}</div>}
                <div className="mt-1 text-ink-faint">心跳 {formatDateTime(sel.last_heartbeat_at)}</div>
              </div>
            )}
          </>
        }
      />
    </div>
  )
}

export default function PilesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const [drawer, setDrawer] = useState<PileDrawerState>(null)
  const [deleteTarget, setDeleteTarget] = useState<Pile | null>(null)

  const params: PileListParams = { ...toFilter(applied), page, pageSize, sort }
  const list = useQuery({
    queryKey: pileKeys.list(params),
    queryFn: () => listPiles(params),
    placeholderData: keepPreviousData,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: pileKeys.all })

  const remove = useMutation({
    mutationFn: (id: string) => deletePile(id),
    onSuccess: () => {
      toast.success('充电桩已删除')
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

  const canUpdate = can('asset:pile:update')
  const canDelete = can('asset:pile:delete')

  const columns: Column<Pile>[] = [
    {
      key: 'pile_code',
      title: '编号 / 名称',
      sortable: true,
      render: (p) => (
        <div>
          <div className="font-medium text-ink-strong">{p.name}</div>
          <div className="font-mono text-xs text-ink-faint">{p.pile_code}</div>
        </div>
      ),
    },
    { key: 'type', title: '类型', render: (p) => <Badge color={p.type === 'fast' ? 'purple' : 'blue'}>{pileTypeLabel[p.type]}</Badge> },
    { key: 'power_kw', title: '功率', sortable: true, render: (p) => <span className="text-xs">{p.power_kw} kW</span> },
    { key: 'connector_count', title: '枪数', align: 'center', render: (p) => p.connector_count },
    { key: 'vendor', title: '厂商', render: (p) => text(p.vendor) },
    {
      key: 'location',
      title: '位置',
      render: (p) => (
        <div>
          <div className="line-clamp-1 max-w-xs text-xs text-ink">{text(p.location)}</div>
          {toLngLat(p.lng, p.lat) ? (
            <div className="font-mono text-[11px] text-ink-faint">
              {p.lng?.toFixed(5)}, {p.lat?.toFixed(5)}
            </div>
          ) : (
            <div className="text-[11px] text-ink-faint">无坐标</div>
          )}
        </div>
      ),
    },
    { key: 'status', title: '状态', sortable: true, render: (p) => <Badge color={pileStatusColor[p.status]}>{pileStatusLabel[p.status]}</Badge> },
    { key: 'last_heartbeat_at', title: '最近心跳', sortable: true, render: (p) => <span className="text-xs text-ink-muted">{formatDateTime(p.last_heartbeat_at)}</span> },
  ]

  const renderActions = (p: Pile) => (
    <div className="flex items-center justify-end gap-0.5">
      {canUpdate && <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', pile: p })} />}
      {canDelete && (
        <Button variant="ghost" size="sm" icon={Trash2} className="!px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200" title="删除" aria-label="删除" onClick={() => setDeleteTarget(p)} />
      )}
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="充电桩档案"
        description="桩位、功率与状态；空闲 / 充电中 / 故障由 OCPP 上报"
        extra={
          <Can perm="asset:pile:create">
            <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
              新建充电桩
            </Button>
          </Can>
        }
      />

      <PilesMap selectedId={selectedId} onSelect={setSelectedId} />

      <FilterBar keyword={draft.keyword} onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))} keywordPlaceholder="桩编号 / 名称 / 位置" onSearch={search} onReset={reset} loading={list.isFetching}>
        <div className="w-full sm:w-32">
          <Select
            options={pileTypeOptions}
            placeholder="全部类型"
            value={draft.type}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, type: isPileType(v) ? v : '' }))
            }}
            aria-label="类型"
          />
        </div>
        <div className="w-full sm:w-36">
          <Select
            options={pileStatusOptions}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isPileStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
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
              onRowClick={(p) => setSelectedId(p.id)}
              actions={canUpdate || canDelete ? { render: renderActions, width: 100 } : undefined}
              empty={<Empty title="没有匹配的充电桩" description="调整筛选条件，或新建充电桩" />}
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

      <PileDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
        }}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除充电桩「${deleteTarget?.name ?? ''}」？`}
        description={`编号 ${deleteTarget?.pile_code ?? ''}。软删除，删除后该桩的 OCPP 连接将被拒绝。`}
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
