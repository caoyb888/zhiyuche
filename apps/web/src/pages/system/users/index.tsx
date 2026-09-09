import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, KeyRound, Pencil, Plus, ShieldCheck, Trash2, Upload } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import type { User, UserStatus } from '../../../api/types'
import { deleteUser, exportUsers, listUsers, userKeys, type UserFilter, type UserListParams } from '../../../api/users'
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
import TreeSelect from '../../../components/ui/TreeSelect'
import { useDeptTree } from '../../../hooks/useDeptTree'
import { usePermission } from '../../../hooks/usePermission'
import { useRoleOptions } from '../../../hooks/useRoleOptions'
import { useAuthStore } from '../../../store/auth'
import { saveBlob } from '../../../utils/download'
import { formatDateTime, text } from '../../../utils/format'
import AssignRolesModal from './AssignRolesModal'
import ImportModal from './ImportModal'
import ResetPasswordModal from './ResetPasswordModal'
import { isUserStatus, userStatusColor, userStatusLabel, userStatusOptions } from './schemas'
import UserDrawer, { type UserDrawerState } from './UserDrawer'

interface Filters {
  keyword: string
  dept_id: string | null
  status: UserStatus | ''
  role_id: string
}

const EMPTY_FILTERS: Filters = { keyword: '', dept_id: null, status: '', role_id: '' }
const DEFAULT_PAGE_SIZE = 20

function toFilter(f: Filters): UserFilter {
  return {
    keyword: f.keyword || undefined,
    dept_id: f.dept_id ?? undefined,
    status: f.status || undefined,
    role_id: f.role_id || undefined,
  }
}

export default function UsersPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const me = useAuthStore((s) => s.profile)

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)

  const [drawer, setDrawer] = useState<UserDrawerState>(null)
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [assignTarget, setAssignTarget] = useState<User | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null)
  const [importOpen, setImportOpen] = useState(false)

  const params: UserListParams = { ...toFilter(applied), page, pageSize, sort }
  const list = useQuery({
    queryKey: userKeys.list(params),
    queryFn: () => listUsers(params),
    placeholderData: keepPreviousData,
  })

  const { nodes: deptNodes, query: deptQuery } = useDeptTree()
  const { options: roleOptions, query: roleQuery } = useRoleOptions()

  const invalidate = () => queryClient.invalidateQueries({ queryKey: userKeys.all })

  const remove = useMutation({
    mutationFn: (id: string) => deleteUser(id),
    onSuccess: () => {
      toast.success('用户已删除')
      setDeleteTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  const exportMutation = useMutation({
    mutationFn: () => exportUsers(toFilter(applied)),
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
    setSort(undefined)
  }

  const canUpdate = can('system:user:update')
  const canReset = can('system:user:reset-password')
  const canAssign = can('system:user:assign-roles')
  const canDelete = can('system:user:delete')
  const hasRowActions = canUpdate || canReset || canAssign || canDelete

  const columns: Column<User>[] = [
    {
      key: 'username',
      title: '用户名',
      sortable: true,
      render: (u) => (
        <div className="flex items-center gap-1.5">
          <span className="font-medium text-slate-800">{u.username}</span>
          {u.is_super && <Badge color="purple">平台</Badge>}
        </div>
      ),
    },
    { key: 'name', title: '姓名', sortable: true },
    { key: 'dept_name', title: '部门', render: (u) => text(u.dept_name) },
    { key: 'phone', title: '手机', render: (u) => <span className="font-mono text-xs">{text(u.phone)}</span> },
    {
      key: 'roles',
      title: '角色',
      render: (u) =>
        u.roles.length === 0 ? (
          <span className="text-slate-400">—</span>
        ) : (
          <div className="flex flex-wrap gap-1">
            {u.roles.map((r) => (
              <Badge key={r.id} color="blue">
                {r.name}
              </Badge>
            ))}
          </div>
        ),
    },
    {
      key: 'status',
      title: '状态',
      sortable: true,
      render: (u) => <Badge color={userStatusColor[u.status]}>{userStatusLabel[u.status]}</Badge>,
    },
    { key: 'last_login_at', title: '最后登录', sortable: true, render: (u) => <span className="text-xs text-slate-500">{formatDateTime(u.last_login_at)}</span> },
  ]

  const renderActions = (u: User) => {
    const isSelf = me?.id === u.id
    return (
      <div className="flex items-center justify-end gap-0.5">
        {canUpdate && (
          <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', user: u })} />
        )}
        {canReset && (
          <Button variant="ghost" size="sm" icon={KeyRound} className="!px-2" title="重置密码" aria-label="重置密码" onClick={() => setResetTarget(u)} />
        )}
        {canAssign && (
          <Button variant="ghost" size="sm" icon={ShieldCheck} className="!px-2" title="分配角色" aria-label="分配角色" onClick={() => setAssignTarget(u)} />
        )}
        {canDelete && (
          <Button
            variant="ghost"
            size="sm"
            icon={Trash2}
            className="!px-2 text-red-500 hover:bg-red-50 hover:text-red-600"
            title={isSelf ? '不能删除自己的账号' : '删除'}
            aria-label="删除"
            disabled={isSelf}
            onClick={() => setDeleteTarget(u)}
          />
        )}
      </div>
    )
  }

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="用户管理"
        description="维护账号、部门归属与角色分配"
        extra={
          <>
            <Can perm="system:user:import">
              <Button variant="secondary" icon={Upload} onClick={() => setImportOpen(true)}>
                导入
              </Button>
            </Can>
            <Can perm="system:user:export">
              <Button variant="secondary" icon={Download} loading={exportMutation.isPending} onClick={() => exportMutation.mutate()}>
                导出
              </Button>
            </Can>
            <Can perm="system:user:create">
              <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
                新建用户
              </Button>
            </Can>
          </>
        }
      />

      <FilterBar
        keyword={draft.keyword}
        onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))}
        keywordPlaceholder="用户名 / 姓名 / 手机 / 工号"
        onSearch={search}
        onReset={reset}
        loading={list.isFetching}
      >
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
        <div className="w-full sm:w-36">
          <Select
            options={userStatusOptions}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isUserStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
        </div>
        <div className="w-full sm:w-44">
          <Select
            options={roleOptions}
            placeholder={roleQuery.isError ? '角色数据暂不可用' : '全部角色'}
            value={draft.role_id}
            onChange={(e) => setDraft((d) => ({ ...d, role_id: e.target.value }))}
            aria-label="角色"
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
              actions={hasRowActions ? { render: renderActions, width: 160 } : undefined}
              empty={<Empty title="没有匹配的用户" description="调整筛选条件，或新建用户" />}
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

      <UserDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
        }}
        deptNodes={deptNodes}
        deptLoading={deptQuery.isLoading}
        deptUnavailable={deptQuery.isError}
        roleOptions={roleOptions}
        rolesUnavailable={roleQuery.isError}
      />
      <ResetPasswordModal user={resetTarget} onClose={() => setResetTarget(null)} />
      <AssignRolesModal
        user={assignTarget}
        roleOptions={roleOptions}
        rolesUnavailable={roleQuery.isError}
        onClose={() => setAssignTarget(null)}
        onSaved={() => {
          setAssignTarget(null)
          void invalidate()
        }}
      />
      <ImportModal open={importOpen} onClose={() => setImportOpen(false)} onImported={() => void invalidate()} />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除用户「${deleteTarget?.name ?? ''}」？`}
        description={`用户名 ${deleteTarget?.username ?? ''}。删除后该账号立即失效且不可恢复。`}
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
