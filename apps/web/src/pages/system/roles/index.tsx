import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { deleteRole, listRoles, roleKeys, type RoleListParams } from '../../../api/roles'
import type { Role } from '../../../api/types'
import Can from '../../../components/Can'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import FilterBar from '../../../components/ui/FilterBar'
import PageHeader from '../../../components/ui/PageHeader'
import Pagination from '../../../components/ui/Pagination'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { usePermission } from '../../../hooks/usePermission'
import { formatDateTime, text } from '../../../utils/format'
import PermissionsModal from './PermissionsModal'
import RoleDrawer, { type RoleDrawerState } from './RoleDrawer'

const DEFAULT_PAGE_SIZE = 20

export default function RolesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()

  const [draftKeyword, setDraftKeyword] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)

  const [drawer, setDrawer] = useState<RoleDrawerState>(null)
  const [permTarget, setPermTarget] = useState<Role | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Role | null>(null)

  const params: RoleListParams = { keyword: keyword || undefined, page, pageSize, sort }
  const list = useQuery({
    queryKey: roleKeys.list(params),
    queryFn: () => listRoles(params),
    placeholderData: keepPreviousData,
  })

  // 角色变化会影响用户页的角色下拉与用户的角色标签
  const invalidate = () => Promise.all([queryClient.invalidateQueries({ queryKey: roleKeys.all }), queryClient.invalidateQueries({ queryKey: ['users'] })])

  const remove = useMutation({
    mutationFn: (id: string) => deleteRole(id),
    onSuccess: () => {
      toast.success('角色已删除')
      setDeleteTarget(null)
      void invalidate()
    },
    // 409：内置角色或仍有用户持有，直接展示后端原因
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  const search = () => {
    setKeyword(draftKeyword.trim())
    setPage(1)
  }
  const reset = () => {
    setDraftKeyword('')
    setKeyword('')
    setPage(1)
    setSort(undefined)
  }

  const canUpdate = can('system:role:update')
  const canAssign = can('system:role:assign-perms')
  const canDelete = can('system:role:delete')
  const hasRowActions = canUpdate || canAssign || canDelete

  const columns: Column<Role>[] = [
    {
      key: 'code',
      title: '代码',
      sortable: true,
      render: (r) => (
        <div className="flex items-center gap-1.5">
          <code className="font-mono text-xs text-ink">{r.code}</code>
          {r.tenant_id === null && <Badge color="purple">平台级</Badge>}
        </div>
      ),
    },
    { key: 'name', title: '名称', sortable: true, render: (r) => <span className="font-medium text-ink-strong">{r.name}</span> },
    { key: 'description', title: '描述', render: (r) => <span className="text-ink-muted">{text(r.description)}</span> },
    {
      key: 'is_system',
      title: '类型',
      render: (r) => (r.is_system ? <Badge color="amber">内置</Badge> : <Badge color="gray">自定义</Badge>),
    },
    { key: 'permissions', title: '权限数', align: 'right', render: (r) => <span className="tabular-nums">{r.permissions.length}</span> },
    { key: 'user_count', title: '用户数', align: 'right', render: (r) => <span className="tabular-nums">{r.user_count}</span> },
    { key: 'created_at', title: '创建时间', sortable: true, render: (r) => <span className="text-xs text-ink-muted">{formatDateTime(r.created_at)}</span> },
  ]

  const renderActions = (r: Role) => (
    <div className="flex items-center justify-end gap-0.5">
      {canUpdate && (
        <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', role: r })} />
      )}
      {canAssign && (
        <Button variant="ghost" size="sm" icon={KeyRound} className="!px-2" title="分配权限" aria-label="分配权限" onClick={() => setPermTarget(r)} />
      )}
      {canDelete && (
        <Button
          variant="ghost"
          size="sm"
          icon={Trash2}
          className="!px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200"
          title={r.is_system ? '内置角色不能删除' : r.user_count > 0 ? '仍有用户持有该角色' : '删除'}
          aria-label="删除"
          onClick={() => setDeleteTarget(r)}
        />
      )}
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="角色管理"
        description="角色持有动作权限码；菜单可见性由权限推导"
        extra={
          <Can perm="system:role:create">
            <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
              新建角色
            </Button>
          </Can>
        }
      />

      <FilterBar keyword={draftKeyword} onKeywordChange={setDraftKeyword} keywordPlaceholder="代码 / 名称" onSearch={search} onReset={reset} loading={list.isFetching} />

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
              empty={<Empty title="没有匹配的角色" description="调整关键词，或新建角色" />}
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

      <RoleDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
        }}
      />
      <PermissionsModal
        role={permTarget}
        onClose={() => setPermTarget(null)}
        onSaved={() => {
          setPermTarget(null)
          void invalidate()
        }}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除角色「${deleteTarget?.name ?? ''}」？`}
        description={
          deleteTarget?.is_system
            ? '这是内置角色，后端将拒绝删除。'
            : deleteTarget && deleteTarget.user_count > 0
              ? `仍有 ${deleteTarget.user_count} 个用户持有该角色，后端将拒绝删除；请先调整用户角色。`
              : `代码 ${deleteTarget?.code ?? ''}。删除后不可恢复。`
        }
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
