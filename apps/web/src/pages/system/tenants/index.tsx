import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Eye, Pencil, Plus, Power, PowerOff } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { listTenants, setTenantStatus, tenantKeys, type TenantListParams } from '../../../api/tenants'
import type { Tenant, TenantStatus } from '../../../api/types'
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
import { useAuthStore } from '../../../store/auth'
import { formatDateTime, text } from '../../../utils/format'
import { isTenantStatus, tenantStatusColor, tenantStatusLabel, tenantStatusOptions } from './schemas'
import TenantDrawer, { type TenantDrawerState } from './TenantDrawer'

interface Filters {
  keyword: string
  status: TenantStatus | ''
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '' }
const DEFAULT_PAGE_SIZE = 20

export default function TenantsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const profile = useAuthStore((s) => s.profile)
  const viewTenantId = useAuthStore((s) => s.viewTenantId)
  const setViewTenant = useAuthStore((s) => s.setViewTenant)

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)

  const [drawer, setDrawer] = useState<TenantDrawerState>(null)
  const [statusTarget, setStatusTarget] = useState<Tenant | null>(null)

  const params: TenantListParams = { keyword: applied.keyword || undefined, status: applied.status || undefined, page, pageSize, sort }
  const list = useQuery({
    queryKey: tenantKeys.list(params),
    queryFn: () => listTenants(params),
    placeholderData: keepPreviousData,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: tenantKeys.all })

  const toggleStatus = useMutation({
    mutationFn: (t: Tenant) => setTenantStatus(t.id, t.status === 'active' ? 'disabled' : 'active'),
    onSuccess: (t) => {
      toast.success(t.status === 'disabled' ? '租户已停用' : '租户已启用', t.status === 'disabled' ? '该租户所有用户会话已失效' : undefined)
      setStatusTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('操作失败', errorMessage(e)),
  })

  const search = () => {
    setApplied({ ...draft, keyword: draft.keyword.trim() })
    setPage(1)
  }
  const reset = () => {
    setDraft(EMPTY_FILTERS)
    setApplied(EMPTY_FILTERS)
    setPage(1)
    setSort(undefined)
  }

  const canUpdate = can('system:tenant:update')
  const canDisable = can('system:tenant:delete')
  const isSuper = profile?.is_super ?? false
  const hasRowActions = canUpdate || canDisable || isSuper

  const switchView = (t: Tenant) => {
    // 查看自己的平台租户 = 回到平台视角
    const target = t.id === profile?.tenant.id ? null : { id: t.id, code: t.code, name: t.name }
    setViewTenant(target)
    void queryClient.resetQueries()
    toast.info(target ? `已切换查看租户「${t.name}」` : '已回到平台视角')
  }

  const columns: Column<Tenant>[] = [
    {
      key: 'code',
      title: '代码',
      sortable: true,
      render: (t) => (
        <div className="flex items-center gap-1.5">
          <code className="font-mono text-xs text-slate-700">{t.code}</code>
          {t.is_platform && <Badge color="purple">平台</Badge>}
          {t.id === viewTenantId && <Badge color="blue">查看中</Badge>}
        </div>
      ),
    },
    { key: 'name', title: '名称', sortable: true, render: (t) => <span className="font-medium text-slate-800">{t.name}</span> },
    {
      key: 'contact',
      title: '联系人',
      render: (t) => (
        <div className="text-slate-600">
          <div>{text(t.contact_name)}</div>
          {t.contact_phone && <div className="font-mono text-xs text-slate-400">{t.contact_phone}</div>}
        </div>
      ),
    },
    { key: 'status', title: '状态', sortable: true, width: 80, render: (t) => <Badge color={tenantStatusColor[t.status]}>{tenantStatusLabel[t.status]}</Badge> },
    { key: 'user_count', title: '用户数', align: 'right', width: 80, render: (t) => <span className="tabular-nums">{t.user_count}</span> },
    { key: 'dept_count', title: '部门数', align: 'right', width: 80, render: (t) => <span className="tabular-nums">{t.dept_count}</span> },
    {
      key: 'expires_at',
      title: '到期时间',
      sortable: true,
      render: (t) => {
        if (!t.expires_at) return <span className="text-xs text-slate-400">长期有效</span>
        const expired = new Date(t.expires_at).getTime() < Date.now()
        return <span className={expired ? 'text-xs text-red-600' : 'text-xs text-slate-500'}>{formatDateTime(t.expires_at)}{expired ? '（已到期）' : ''}</span>
      },
    },
    { key: 'created_at', title: '创建时间', sortable: true, render: (t) => <span className="text-xs text-slate-500">{formatDateTime(t.created_at)}</span> },
  ]

  const renderActions = (t: Tenant) => {
    const active = t.status === 'active'
    return (
      <div className="flex items-center justify-end gap-0.5">
        {isSuper && (
          <Button
            variant="ghost"
            size="sm"
            icon={Eye}
            className="!px-2"
            title={t.id === viewTenantId ? '正在查看' : '切换查看该租户'}
            aria-label="切换查看"
            disabled={t.id === viewTenantId || t.status !== 'active'}
            onClick={() => switchView(t)}
          />
        )}
        {canUpdate && <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', tenant: t })} />}
        {(active ? canDisable : canUpdate) && (
          <Button
            variant="ghost"
            size="sm"
            icon={active ? PowerOff : Power}
            className={active ? '!px-2 text-red-500 hover:bg-red-50 hover:text-red-600' : '!px-2 text-emerald-600 hover:bg-emerald-50'}
            title={t.is_platform ? '平台租户不可停用' : active ? '停用' : '启用'}
            aria-label={active ? '停用' : '启用'}
            disabled={active && t.is_platform}
            onClick={() => setStatusTarget(t)}
          />
        )}
      </div>
    )
  }

  const disabling = statusTarget?.status === 'active'

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="租户管理"
        description="平台管理员维护入驻企业；停用后该租户用户立即无法登录"
        extra={
          <Can perm="system:tenant:create">
            <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
              新建租户
            </Button>
          </Can>
        }
      />

      <FilterBar
        keyword={draft.keyword}
        onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))}
        keywordPlaceholder="代码 / 名称 / 联系人"
        onSearch={search}
        onReset={reset}
        loading={list.isFetching}
      >
        <div className="w-full sm:w-36">
          <Select
            options={tenantStatusOptions}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isTenantStatus(v) ? v : '' }))
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
              actions={hasRowActions ? { render: renderActions, width: 130 } : undefined}
              empty={<Empty title="没有匹配的租户" description="调整筛选条件，或新建租户" />}
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

      <TenantDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
          // 顶栏下拉与之共用 tenants 前缀，一并刷新
        }}
      />
      <ConfirmDialog
        open={statusTarget !== null}
        danger={disabling}
        title={`${disabling ? '停用' : '启用'}租户「${statusTarget?.name ?? ''}」？`}
        description={
          disabling
            ? `停用后该租户 ${statusTarget?.user_count ?? 0} 个用户的登录会话立即失效且无法登录；数据保留，可随时重新启用。`
            : '启用后该租户用户可正常登录。'
        }
        confirmText={disabling ? '停用' : '启用'}
        loading={toggleStatus.isPending}
        onConfirm={() => {
          if (statusTarget) toggleStatus.mutate(statusTarget)
        }}
        onCancel={() => setStatusTarget(null)}
      />
    </div>
  )
}
