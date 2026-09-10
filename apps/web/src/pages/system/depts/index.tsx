import { useMutation, useQueryClient } from '@tanstack/react-query'
import { FolderTree, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { deleteDept, deptKeys } from '../../../api/depts'
import type { DeptNode } from '../../../api/types'
import Can from '../../../components/Can'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import DescriptionList from '../../../components/ui/DescriptionList'
import Drawer from '../../../components/ui/Drawer'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import PageHeader from '../../../components/ui/PageHeader'
import Spinner from '../../../components/ui/Spinner'
import Tabs from '../../../components/ui/Tabs'
import { useToast } from '../../../components/ui/toast-context'
import Tree from '../../../components/ui/Tree'
import { findDept, useDeptTree } from '../../../hooks/useDeptTree'
import { usePermission } from '../../../hooks/usePermission'
import { formatDateTime, formatMoney, text } from '../../../utils/format'
import DeptForm from './DeptForm'

type DetailTab = 'detail' | 'edit'

export default function DeptsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const { query, depts, nodes } = useDeptTree()

  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [tab, setTab] = useState<DetailTab>('detail')
  const [create, setCreate] = useState<{ parentId: string | null } | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const selected = selectedId ? findDept(depts, selectedId) : undefined
  const parent = selected?.parent_id ? findDept(depts, selected.parent_id) : undefined
  const canUpdate = can('system:dept:update')
  const activeTab: DetailTab = canUpdate ? tab : 'detail'

  const invalidate = () => queryClient.invalidateQueries({ queryKey: deptKeys.all })

  const remove = useMutation({
    mutationFn: (id: string) => deleteDept(id),
    onSuccess: () => {
      toast.success('部门已删除')
      setConfirmDelete(false)
      setSelectedId(null)
      void invalidate()
    },
    // 409：有子部门或在职用户时后端拒绝，直接展示原因
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  const createParentName = create?.parentId ? findDept(depts, create.parentId)?.name : undefined

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="部门管理"
        description="组织架构、负责人与月度预算"
        extra={
          <Can perm="system:dept:create">
            <Button icon={Plus} onClick={() => setCreate({ parentId: null })}>
              新建根部门
            </Button>
          </Can>
        }
      />

      <div className="grid gap-4 lg:grid-cols-[320px_minmax(0,1fr)] items-start">
        {/* ── 左：部门树 ── */}
        <div className="card p-3">
          <div className="flex items-center justify-between px-1 pb-2 border-b border-line">
            <div className="flex items-center gap-1.5 text-sm font-medium text-ink">
              <FolderTree size={15} className="text-ink-faint" />
              部门树
            </div>
            <span className="text-xs text-ink-faint">{depts.length > 0 ? `${countDepts(depts)} 个部门` : ''}</span>
          </div>
          <div className="pt-2 max-h-[70vh] overflow-y-auto">
            {query.isPending ? (
              <div className="flex justify-center py-10">
                <Spinner label="加载中" />
              </div>
            ) : query.isError ? (
              <ErrorState size="sm" message={errorMessage(query.error)} onRetry={() => void query.refetch()} />
            ) : depts.length === 0 ? (
              <Empty
                size="sm"
                title="暂无部门"
                description="先创建一个根部门"
                action={
                  <Can perm="system:dept:create">
                    <Button size="sm" variant="secondary" icon={Plus} onClick={() => setCreate({ parentId: null })}>
                      新建根部门
                    </Button>
                  </Can>
                }
              />
            ) : (
              <Tree
                nodes={nodes}
                selectedKey={selectedId}
                onSelect={(key) => {
                  setSelectedId(key)
                  setTab('detail')
                }}
              />
            )}
          </div>
        </div>

        {/* ── 右：详情 / 编辑 ── */}
        <div className="card p-5 min-h-[20rem]">
          {!selected ? (
            <Empty icon={FolderTree} title="选择左侧部门" description="查看详情、编辑或新建子部门" />
          ) : (
            <div className="space-y-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h3 className="text-base font-semibold text-ink-strong truncate">{selected.name}</h3>
                    <Badge color={selected.status === 'active' ? 'green' : 'gray'}>{selected.status === 'active' ? '启用' : '停用'}</Badge>
                  </div>
                  <div className="mt-0.5 text-xs text-ink-faint">
                    {selected.code ? `编码 ${selected.code} · ` : ''}直属 {selected.user_count} 人
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Can perm="system:dept:create">
                    <Button variant="secondary" size="sm" icon={Plus} onClick={() => setCreate({ parentId: selected.id })}>
                      新建子部门
                    </Button>
                  </Can>
                  <Can perm="system:dept:delete">
                    <Button variant="danger" size="sm" icon={Trash2} onClick={() => setConfirmDelete(true)}>
                      删除
                    </Button>
                  </Can>
                </div>
              </div>

              {canUpdate && (
                <Tabs<DetailTab>
                  items={[
                    { key: 'detail', label: '详情' },
                    { key: 'edit', label: '编辑' },
                  ]}
                  value={activeTab}
                  onChange={setTab}
                  size="sm"
                />
              )}

              {activeTab === 'detail' ? (
                <DescriptionList
                  columns={2}
                  items={[
                    { label: '部门名称', value: selected.name },
                    { label: '部门编码', value: text(selected.code) },
                    { label: '上级部门', value: selected.parent_id ? (parent?.name ?? '—') : '（根部门）' },
                    { label: '负责人', value: text(selected.leader_name) },
                    { label: '排序', value: selected.sort },
                    { label: '月度预算', value: `¥ ${formatMoney(selected.monthly_budget)}` },
                    { label: '直属在职人数', value: `${selected.user_count} 人` },
                    { label: '子部门数', value: `${selected.children?.length ?? 0} 个` },
                    { label: '创建时间', value: formatDateTime(selected.created_at) },
                    { label: '更新时间', value: formatDateTime(selected.updated_at) },
                  ]}
                />
              ) : (
                <DeptForm
                  key={selected.id}
                  mode="edit"
                  dept={selected}
                  depts={depts}
                  nodes={nodes}
                  onSaved={() => {
                    setTab('detail')
                    void invalidate()
                  }}
                />
              )}
            </div>
          )}
        </div>
      </div>

      <Drawer
        open={create !== null}
        onClose={() => setCreate(null)}
        title={createParentName ? `新建子部门 · ${createParentName}` : '新建根部门'}
        width="md"
      >
        {create && (
          <DeptForm
            mode="create"
            parentId={create.parentId}
            depts={depts}
            nodes={nodes}
            onCancel={() => setCreate(null)}
            onSaved={(d) => {
              setCreate(null)
              setSelectedId(d.id)
              setTab('detail')
              void invalidate()
            }}
          />
        )}
      </Drawer>

      <ConfirmDialog
        open={confirmDelete && selected !== undefined}
        danger
        title={`删除部门「${selected?.name ?? ''}」？`}
        description="存在子部门或在职用户时将被拒绝；删除后不可恢复。"
        confirmText="删除"
        loading={remove.isPending}
        onConfirm={() => {
          if (selected) remove.mutate(selected.id)
        }}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  )
}

function countDepts(list: DeptNode[]): number {
  return list.reduce((n, d) => n + 1 + countDepts(d.children ?? []), 0)
}
