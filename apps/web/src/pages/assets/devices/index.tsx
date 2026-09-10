import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { KeyRound, Link2, Link2Off, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { deleteDevice, deviceKeys, listDevices, rotateDeviceKey, unbindDevice, type DeviceFilter, type DeviceListParams } from '../../../api/devices'
import type { Device, DeviceStatus, DeviceWithKey } from '../../../api/types'
import { vehicleKeys } from '../../../api/vehicles'
import Can from '../../../components/Can'
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
import { useVehicleOptions } from '../../../hooks/useVehicleOptions'
import { formatDateTime, text } from '../../../utils/format'
import ApiKeyModal from './ApiKeyModal'
import BindVehicleModal from './BindVehicleModal'
import DeviceDrawer, { type DeviceDrawerState } from './DeviceDrawer'
import { boundOptions, deviceStatusColor, deviceStatusLabel, deviceStatusOptions, isDeviceStatus, onlineOptions, parseBool } from './schemas'

interface Filters {
  keyword: string
  status: DeviceStatus | ''
  bound: string
  online: string
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '', bound: '', online: '' }
const DEFAULT_PAGE_SIZE = 20

function toFilter(f: Filters): DeviceFilter {
  return { keyword: f.keyword || undefined, status: f.status || undefined, bound: parseBool(f.bound), online: parseBool(f.online) }
}

type KeyResult = { reason: 'create' | 'rotate'; device: DeviceWithKey } | null

export default function DevicesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)

  const [drawer, setDrawer] = useState<DeviceDrawerState>(null)
  const [keyResult, setKeyResult] = useState<KeyResult>(null)
  const [bindTarget, setBindTarget] = useState<Device | null>(null)
  const [unbindTarget, setUnbindTarget] = useState<Device | null>(null)
  const [rotateTarget, setRotateTarget] = useState<Device | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Device | null>(null)

  const params: DeviceListParams = { ...toFilter(applied), page, pageSize, sort }
  const list = useQuery({
    queryKey: deviceKeys.list(params),
    queryFn: () => listDevices(params),
    placeholderData: keepPreviousData,
  })

  const canUpdate = can('asset:device:update')
  const canDelete = can('asset:device:delete')
  const canCreate = can('asset:device:create')
  const { options: vehicleOptions, query: vehicleQuery } = useVehicleOptions(canCreate || canUpdate)

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: deviceKeys.all })
    // 绑定关系变化会影响车辆的 device_serial
    void queryClient.invalidateQueries({ queryKey: vehicleKeys.all })
  }

  const unbind = useMutation({
    mutationFn: (id: string) => unbindDevice(id),
    onSuccess: () => {
      toast.success('已解绑车辆')
      setUnbindTarget(null)
      invalidate()
    },
    onError: (e) => toast.error('解绑失败', errorMessage(e)),
  })

  const rotate = useMutation({
    mutationFn: (id: string) => rotateDeviceKey(id),
    onSuccess: (d) => {
      setRotateTarget(null)
      setKeyResult({ reason: 'rotate', device: d })
      invalidate()
    },
    onError: (e) => toast.error('重置密钥失败', errorMessage(e)),
  })

  const remove = useMutation({
    mutationFn: (id: string) => deleteDevice(id),
    onSuccess: () => {
      toast.success('设备已删除')
      setDeleteTarget(null)
      invalidate()
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

  const columns: Column<Device>[] = [
    {
      key: 'serial_no',
      title: '序列号',
      sortable: true,
      render: (d) => (
        <div>
          <div className="font-mono text-sm font-medium text-ink-strong">{d.serial_no}</div>
          <div className="text-xs text-ink-faint">
            {d.model}
            {d.firmware ? ` · v${d.firmware}` : ''}
          </div>
        </div>
      ),
    },
    { key: 'iccid', title: 'ICCID', render: (d) => <span className="font-mono text-xs">{text(d.iccid)}</span> },
    {
      key: 'vehicle_plate',
      title: '绑定车辆',
      render: (d) => (d.vehicle_id ? <Badge color="blue">{d.vehicle_plate ?? d.vehicle_id.slice(0, 8)}</Badge> : <span className="text-xs text-ink-faint">未绑定</span>),
    },
    { key: 'status', title: '状态', sortable: true, render: (d) => <Badge color={deviceStatusColor[d.status]}>{deviceStatusLabel[d.status]}</Badge> },
    {
      key: 'online',
      title: '在线',
      render: (d) => (
        <div>
          <span className={clsx('inline-flex items-center gap-1 text-xs', d.online ? 'text-ev-200' : 'text-ink-faint')}>
            <span className={clsx('inline-block h-1.5 w-1.5 rounded-full', d.online ? 'bg-ev-500 pulse-dot' : 'bg-ink-disabled')} />
            {d.online ? '在线' : '离线'}
          </span>
          <div className="text-[11px] text-ink-faint">{formatDateTime(d.last_online_at)}</div>
        </div>
      ),
    },
    { key: 'last_ip', title: '最近 IP', render: (d) => <span className="font-mono text-xs">{text(d.last_ip)}</span> },
    { key: 'created_at', title: '创建时间', sortable: true, render: (d) => <span className="text-xs text-ink-muted">{formatDateTime(d.created_at)}</span> },
  ]

  const renderActions = (d: Device) => (
    <div className="flex items-center justify-end gap-0.5">
      {canUpdate && <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', device: d })} />}
      {canUpdate &&
        (d.vehicle_id ? (
          <Button variant="ghost" size="sm" icon={Link2Off} className="!px-2" title="解绑车辆" aria-label="解绑车辆" onClick={() => setUnbindTarget(d)} />
        ) : (
          <Button variant="ghost" size="sm" icon={Link2} className="!px-2" title="绑定车辆" aria-label="绑定车辆" onClick={() => setBindTarget(d)} />
        ))}
      {canUpdate && <Button variant="ghost" size="sm" icon={KeyRound} className="!px-2" title="重置密钥" aria-label="重置密钥" onClick={() => setRotateTarget(d)} />}
      {canDelete && (
        <Button variant="ghost" size="sm" icon={Trash2} className="!px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200" title="删除" aria-label="删除" onClick={() => setDeleteTarget(d)} />
      )}
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="网关设备"
        description="车载网关档案与接入密钥；一车一网关"
        extra={
          <Can perm="asset:device:create">
            <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
              新建设备
            </Button>
          </Can>
        }
      />

      <FilterBar keyword={draft.keyword} onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))} keywordPlaceholder="序列号 / ICCID / 车牌" onSearch={search} onReset={reset} loading={list.isFetching}>
        <div className="w-full sm:w-36">
          <Select
            options={deviceStatusOptions}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isDeviceStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
        </div>
        <div className="w-full sm:w-32">
          <Select options={boundOptions} placeholder="绑定情况" value={draft.bound} onChange={(e) => setDraft((d) => ({ ...d, bound: e.target.value }))} aria-label="是否绑定" />
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
              actions={canUpdate || canDelete ? { render: renderActions, width: 170 } : undefined}
              empty={<Empty title="没有匹配的设备" description="调整筛选条件，或新建设备" />}
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

      <DeviceDrawer
        state={drawer}
        vehicleOptions={vehicleOptions}
        vehiclesUnavailable={vehicleQuery.isError}
        onClose={() => setDrawer(null)}
        onCreated={(d) => {
          setDrawer(null)
          setKeyResult({ reason: 'create', device: d })
          invalidate()
        }}
        onUpdated={() => {
          setDrawer(null)
          invalidate()
        }}
      />
      <ApiKeyModal device={keyResult?.device ?? null} reason={keyResult?.reason ?? 'create'} onClose={() => setKeyResult(null)} />
      <BindVehicleModal
        device={bindTarget}
        vehicleOptions={vehicleOptions}
        vehiclesUnavailable={vehicleQuery.isError}
        onClose={() => setBindTarget(null)}
        onSaved={() => {
          setBindTarget(null)
          invalidate()
        }}
      />
      <ConfirmDialog
        open={unbindTarget !== null}
        title={`解绑设备「${unbindTarget?.serial_no ?? ''}」？`}
        description={`将与车辆 ${unbindTarget?.vehicle_plate ?? ''} 解除绑定；车辆在途时无法解绑。`}
        confirmText="解绑"
        loading={unbind.isPending}
        onConfirm={() => {
          if (unbindTarget) unbind.mutate(unbindTarget.id)
        }}
        onCancel={() => setUnbindTarget(null)}
      />
      <ConfirmDialog
        open={rotateTarget !== null}
        danger
        title={`重置设备「${rotateTarget?.serial_no ?? ''}」的接入密钥？`}
        description="旧密钥立即失效，网关需更新为新密钥后才能继续上报；新密钥只显示一次。"
        confirmText="重置密钥"
        loading={rotate.isPending}
        onConfirm={() => {
          if (rotateTarget) rotate.mutate(rotateTarget.id)
        }}
        onCancel={() => setRotateTarget(null)}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除设备「${deleteTarget?.serial_no ?? ''}」？`}
        description={deleteTarget?.vehicle_id ? `该设备当前绑定车辆 ${deleteTarget.vehicle_plate ?? ''}，删除时会先解绑。` : '软删除，删除后该设备无法再接入。'}
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
