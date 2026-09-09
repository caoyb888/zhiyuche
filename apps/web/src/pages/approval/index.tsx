import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Check, Eye, FilePlus2, Settings2, X, Zap } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { approvalKeys, getApprovalTodoCount, listApprovals, type ApprovalFilter, type ApprovalListParams, type ApprovalScope } from '../../api/approvals'
import { errorMessage } from '../../api/client'
import type { Approval, ApprovalStatus, TripType } from '../../api/types'
import Can from '../../components/Can'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import DateTimeInput from '../../components/ui/DateTimeInput'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import FilterBar from '../../components/ui/FilterBar'
import PageHeader from '../../components/ui/PageHeader'
import Pagination from '../../components/ui/Pagination'
import Select from '../../components/ui/Select'
import Table, { type Column } from '../../components/ui/Table'
import Tabs, { type TabItem } from '../../components/ui/Tabs'
import TreeSelect from '../../components/ui/TreeSelect'
import { useDeptTree } from '../../hooks/useDeptTree'
import { usePermission } from '../../hooks/usePermission'
import { useVehicleOptions } from '../../hooks/useVehicleOptions'
import { formatTimeRange, localToRFC3339, text } from '../../utils/format'
import { ApproveModal, RejectModal } from './ApprovalActionModals'
import ApprovalDetailDrawer from './ApprovalDetailDrawer'
import CreateApprovalDrawer from './CreateApprovalDrawer'
import { APPROVAL_STATUS_BADGE, APPROVAL_STATUS_LABEL, APPROVAL_STATUS_OPTIONS, TRIP_TYPE_BADGE, TRIP_TYPE_LABEL, TRIP_TYPE_OPTIONS, currentApproverName, isApprovalStatus, isTripType, purposeText } from './style'

interface Filters {
  keyword: string
  status: ApprovalStatus | ''
  trip_type: TripType | ''
  dept_id: string | null
  vehicle_id: string
  /** 本地时间 YYYY-MM-DDTHH:mm */
  from: string
  to: string
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '', trip_type: '', dept_id: null, vehicle_id: '', from: '', to: '' }
const DEFAULT_PAGE_SIZE = 20
const DEFAULT_SORT = '-created_at'

function toFilter(f: Filters, scope: ApprovalScope): ApprovalFilter {
  return {
    scope,
    keyword: f.keyword || undefined,
    status: f.status || undefined,
    trip_type: f.trip_type || undefined,
    dept_id: f.dept_id ?? undefined,
    vehicle_id: f.vehicle_id || undefined,
    from: localToRFC3339(f.from),
    to: localToRFC3339(f.to),
  }
}

export default function ApprovalPage() {
  const { can } = usePermission()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const canApprove = can('approval:approve')
  const canManage = can('approval:manage')

  // 页签（scope）与详情（id）都由 URL 驱动，支持总览 / 通知深链
  const allowedScopes = useMemo<ApprovalScope[]>(() => {
    const s: ApprovalScope[] = ['mine']
    if (canApprove) s.push('todo')
    if (canManage) s.push('all')
    return s
  }, [canApprove, canManage])
  const rawScope = searchParams.get('scope')
  const scope: ApprovalScope = rawScope && (allowedScopes as string[]).includes(rawScope) ? (rawScope as ApprovalScope) : 'mine'
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
  const [createOpen, setCreateOpen] = useState(false)
  const [quick, setQuick] = useState<{ action: 'approve' | 'reject'; approval: Approval } | null>(null)

  const changeScope = (k: ApprovalScope) => {
    setParam('scope', k === 'mine' ? null : k)
    setPage(1)
  }

  const params: ApprovalListParams = { ...toFilter(applied, scope), page, pageSize, sort }
  const list = useQuery({ queryKey: approvalKeys.list(params), queryFn: () => listApprovals(params), placeholderData: keepPreviousData })
  const todoCount = useQuery({ queryKey: approvalKeys.todoCount, queryFn: getApprovalTodoCount, enabled: canApprove, staleTime: 15_000, refetchInterval: 60_000 })

  const { nodes: deptNodes, query: deptQuery } = useDeptTree()
  const { options: vehicleOptions, query: vehicleQuery } = useVehicleOptions(can('asset:vehicle:view'))

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

  const tabs: TabItem<ApprovalScope>[] = [
    { key: 'mine', label: '我的申请' },
    ...(canApprove ? [{ key: 'todo' as const, label: '待我审批', count: todoCount.data ?? undefined }] : []),
    ...(canManage ? [{ key: 'all' as const, label: '全部' }] : []),
  ]

  const columns: Column<Approval>[] = [
    {
      key: 'apply_no',
      title: '单号',
      sortable: true,
      render: (a) => (
        <div>
          <div className="font-mono text-xs font-medium text-slate-800">{a.apply_no}</div>
          {a.urgency === 'urgent' && (
            <span className="mt-0.5 inline-flex items-center gap-0.5 text-[11px] font-medium text-red-600">
              <Zap size={11} /> 紧急
            </span>
          )}
        </div>
      ),
    },
    {
      key: 'applicant',
      title: '申请人',
      render: (a) => (
        <div>
          <div className="text-slate-800">{a.applicant.name}</div>
          <div className="text-xs text-slate-400">{text(a.dept_name)}</div>
        </div>
      ),
    },
    { key: 'trip_type', title: '类型', render: (a) => <Badge color={TRIP_TYPE_BADGE[a.trip_type]}>{TRIP_TYPE_LABEL[a.trip_type]}</Badge> },
    {
      key: 'purpose',
      title: '事由',
      render: (a) => (
        <div className="max-w-[16rem]">
          <div className="truncate text-slate-800" title={a.purpose_detail}>
            {purposeText(a)}
          </div>
          <div className="truncate text-xs text-slate-400" title={a.purpose_detail}>
            {a.purpose_detail}
          </div>
        </div>
      ),
    },
    { key: 'planned_start', title: '计划时段', sortable: true, render: (a) => <span className="whitespace-nowrap text-xs">{formatTimeRange(a.planned_start, a.planned_end)}</span> },
    {
      key: 'destination',
      title: '目的地',
      render: (a) => (
        <div className="max-w-[12rem] truncate" title={a.destination}>
          {a.destination}
        </div>
      ),
    },
    { key: 'vehicle', title: '车辆', render: (a) => (a.vehicle ? <span className="whitespace-nowrap">{a.vehicle.plate_no}</span> : <span className="text-xs text-slate-400">待指派</span>) },
    { key: 'status', title: '状态', sortable: true, render: (a) => <Badge color={APPROVAL_STATUS_BADGE[a.status]}>{APPROVAL_STATUS_LABEL[a.status]}</Badge> },
    {
      key: 'approver',
      title: '当前审批人',
      render: (a) => {
        const name = currentApproverName(a)
        return name ? <span className="text-xs text-slate-700">{name}</span> : <span className="text-xs text-slate-300">—</span>
      },
    },
  ]

  const renderActions = (a: Approval) => {
    const quickable = canApprove && a.can_approve
    return (
      <div className="flex items-center justify-end gap-0.5">
        <Button variant="ghost" size="sm" icon={Eye} className="!px-2" title="查看" aria-label="查看" onClick={() => setParam('id', a.id)} />
        {quickable && (
          <>
            <Button variant="ghost" size="sm" icon={Check} className="!px-2 text-emerald-600 hover:bg-emerald-50" title="通过" aria-label="通过" onClick={() => setQuick({ action: 'approve', approval: a })} />
            <Button variant="ghost" size="sm" icon={X} className="!px-2 text-red-500 hover:bg-red-50" title="驳回" aria-label="驳回" onClick={() => setQuick({ action: 'reject', approval: a })} />
          </>
        )}
      </div>
    )
  }

  const emptyText: Record<ApprovalScope, { title: string; description: string }> = {
    mine: { title: '还没有申请', description: '点击右上角「发起申请」提交用车申请' },
    todo: { title: '没有待办审批', description: '指派给你的审批步骤会显示在这里' },
    all: { title: '没有匹配的申请', description: '调整筛选条件后再试' },
  }

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="公务审批"
        description="用车申请、审批流与车辆指派；审批结果实时推送"
        extra={
          <>
            <Can perm="approval:rule">
              <Button variant="secondary" icon={Settings2} onClick={() => navigate('/approval/rules')}>
                审批规则
              </Button>
            </Can>
            <Can perm="approval:create">
              <Button icon={FilePlus2} onClick={() => setCreateOpen(true)}>
                发起申请
              </Button>
            </Can>
          </>
        }
      />

      <div className="card px-4 pt-1">
        <Tabs items={tabs} value={scope} onChange={changeScope} />
      </div>

      <FilterBar keyword={draft.keyword} onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))} keywordPlaceholder="单号 / 事由 / 目的地 / 申请人" onSearch={search} onReset={reset} loading={list.isFetching}>
        <div className="w-full sm:w-36">
          <Select
            options={APPROVAL_STATUS_OPTIONS}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isApprovalStatus(v) ? v : '' }))
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
            aria-label="用车类型"
          />
        </div>
        <div className="w-full sm:w-48">
          <TreeSelect nodes={deptNodes} value={draft.dept_id} onChange={(v) => setDraft((d) => ({ ...d, dept_id: v }))} placeholder="全部部门" loading={deptQuery.isLoading} emptyText={deptQuery.isError ? '部门数据暂不可用' : '暂无部门'} />
        </div>
        {can('asset:vehicle:view') && (
          <div className="w-full sm:w-48">
            <Select options={vehicleOptions} placeholder={vehicleQuery.isError ? '车辆数据暂不可用' : '全部车辆'} value={draft.vehicle_id} onChange={(e) => setDraft((d) => ({ ...d, vehicle_id: e.target.value }))} aria-label="车辆" />
          </div>
        )}
        <div className="flex w-full items-center gap-1 sm:w-auto">
          <DateTimeInput value={draft.from} onChange={(v) => setDraft((d) => ({ ...d, from: v }))} aria-label="计划开始起" className="sm:w-44" />
          <span className="text-xs text-slate-400">至</span>
          <DateTimeInput value={draft.to} onChange={(v) => setDraft((d) => ({ ...d, to: v }))} aria-label="计划开始止" className="sm:w-44" />
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
              onRowClick={(a) => setParam('id', a.id)}
              actions={{ render: renderActions, width: 120 }}
              empty={<Empty title={emptyText[scope].title} description={emptyText[scope].description} />}
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

      <ApprovalDetailDrawer id={detailId} onClose={() => setParam('id', null)} />
      <CreateApprovalDrawer
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(a) => {
          setCreateOpen(false)
          const next = new URLSearchParams(searchParams)
          next.delete('scope')
          next.set('id', a.id)
          setSearchParams(next, { replace: true })
        }}
      />
      <ApproveModal approval={quick?.action === 'approve' ? quick.approval : null} onClose={() => setQuick(null)} />
      <RejectModal approval={quick?.action === 'reject' ? quick.approval : null} onClose={() => setQuick(null)} />
    </div>
  )
}
