import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { BookOpen, Pencil, Plus, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { deleteDictItem, deleteDictType, dictKeys, getDictType, listDictTypes, type DictTypeListParams } from '../../../api/dicts'
import type { DictItem, DictType } from '../../../api/types'
import Can from '../../../components/Can'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import Input from '../../../components/ui/Input'
import PageHeader from '../../../components/ui/PageHeader'
import Pagination from '../../../components/ui/Pagination'
import Spinner from '../../../components/ui/Spinner'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { useDebounce } from '../../../hooks/useDebounce'
import { usePermission } from '../../../hooks/usePermission'
import { useAuthStore } from '../../../store/auth'
import { formatDateTime, text } from '../../../utils/format'
import DictItemModal, { type DictItemModalState } from './DictItemModal'
import DictTypeDrawer, { type DictTypeDrawerState } from './DictTypeDrawer'
import { dictItemStatusColor, dictItemStatusLabel } from './schemas'

const PAGE_SIZE = 20

function ScopeBadges({ t }: { t: DictType }) {
  return (
    <>
      {t.is_global ? <Badge color="purple">全局</Badge> : <Badge color="blue">租户</Badge>}
      {t.is_system && <Badge color="amber">系统</Badge>}
    </>
  )
}

export default function DictsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can, isSuper } = usePermission()
  const viewTenantId = useAuthStore((s) => s.viewTenantId)

  const [keyword, setKeyword] = useState('')
  const kw = useDebounce(keyword.trim(), 300)
  const [page, setPage] = useState(1)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const [typeDrawer, setTypeDrawer] = useState<DictTypeDrawerState>(null)
  const [itemModal, setItemModal] = useState<DictItemModalState>(null)
  const [confirmDeleteType, setConfirmDeleteType] = useState(false)
  const [deleteItemTarget, setDeleteItemTarget] = useState<DictItem | null>(null)

  const params: DictTypeListParams = { keyword: kw || undefined, page, pageSize: PAGE_SIZE, sort: 'code' }
  const list = useQuery({
    queryKey: dictKeys.list(params),
    queryFn: () => listDictTypes(params),
    placeholderData: keepPreviousData,
  })

  const detail = useQuery({
    queryKey: dictKeys.detail(selectedId ?? ''),
    queryFn: () => getDictType(selectedId ?? ''),
    enabled: selectedId !== null,
  })
  const selected = detail.data
  const items = selected?.items ?? []

  const invalidate = () => queryClient.invalidateQueries({ queryKey: dictKeys.all })

  const removeType = useMutation({
    mutationFn: (id: string) => deleteDictType(id),
    onSuccess: () => {
      toast.success('字典已删除')
      setConfirmDeleteType(false)
      setSelectedId(null)
      void invalidate()
    },
    // 409：系统字典不可删除；403：全局字典仅平台管理员可删
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  const removeItem = useMutation({
    mutationFn: (id: string) => deleteDictItem(id),
    onSuccess: () => {
      toast.success('条目已删除')
      setDeleteItemTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  // 全局字典对非超级管理员只读
  const writable = (t: DictType) => !t.is_global || isSuper
  const canEditType = selected !== undefined && can('system:dict:update') && writable(selected)
  const canDeleteType = selected !== undefined && can('system:dict:delete') && writable(selected)
  const canEditItems = canEditType

  const itemColumns: Column<DictItem>[] = [
    { key: 'label', title: '显示名', render: (it) => <span className="font-medium text-slate-800">{it.label}</span> },
    { key: 'value', title: '值', render: (it) => <code className="font-mono text-xs text-slate-700">{it.value}</code> },
    { key: 'sort', title: '排序', align: 'right', width: 70, render: (it) => <span className="tabular-nums">{it.sort}</span> },
    {
      key: 'color',
      title: '颜色',
      width: 120,
      render: (it) =>
        it.color ? (
          <span className="inline-flex items-center gap-1.5">
            <span className="h-4 w-4 rounded border border-slate-200" style={{ backgroundColor: it.color }} />
            <code className="font-mono text-xs text-slate-500">{it.color}</code>
          </span>
        ) : (
          <span className="text-slate-400">—</span>
        ),
    },
    { key: 'status', title: '状态', width: 80, render: (it) => <Badge color={dictItemStatusColor[it.status]}>{dictItemStatusLabel[it.status]}</Badge> },
    {
      key: 'extra',
      title: '附加数据',
      render: (it) => {
        const keys = Object.keys(it.extra ?? {})
        return keys.length === 0 ? <span className="text-slate-400">—</span> : <code className="font-mono text-xs text-slate-500">{keys.length} 个字段</code>
      },
    },
  ]

  const renderItemActions = (it: DictItem) => (
    <div className="flex items-center justify-end gap-0.5">
      <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setItemModal({ mode: 'edit', item: it })} />
      <Button
        variant="ghost"
        size="sm"
        icon={Trash2}
        className="!px-2 text-red-500 hover:bg-red-50 hover:text-red-600"
        title="删除"
        aria-label="删除"
        onClick={() => setDeleteItemTarget(it)}
      />
    </div>
  )

  const nextSort = items.reduce((m, it) => Math.max(m, it.sort), -1) + 1

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="字典管理"
        description="全局字典对所有租户可见；租户可用同代码字典覆盖"
        extra={
          <Can perm="system:dict:create">
            <Button icon={Plus} onClick={() => setTypeDrawer({ mode: 'create' })}>
              新建字典
            </Button>
          </Can>
        }
      />

      <div className="grid gap-4 lg:grid-cols-[360px_minmax(0,1fr)] items-start">
        {/* ── 左：字典类型列表 ── */}
        <div className="card p-3 space-y-2">
          <Input
            icon={Search}
            value={keyword}
            onChange={(e) => {
              setKeyword(e.target.value)
              setPage(1)
            }}
            placeholder="搜索代码 / 名称"
            aria-label="搜索字典"
          />
          <div className="max-h-[60vh] overflow-y-auto -mx-1">
            {list.isPending ? (
              <div className="flex justify-center py-10">
                <Spinner label="加载中" />
              </div>
            ) : list.isError ? (
              <ErrorState size="sm" message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
            ) : (list.data?.items.length ?? 0) === 0 ? (
              <Empty size="sm" title={kw ? '没有匹配的字典' : '暂无字典'} description={kw ? '换个关键词试试' : '点击右上角新建'} />
            ) : (
              <ul className={clsx('space-y-0.5', list.isFetching && 'opacity-60')}>
                {list.data?.items.map((t) => {
                  const active = t.id === selectedId
                  return (
                    <li key={t.id}>
                      <button
                        type="button"
                        onClick={() => setSelectedId(t.id)}
                        aria-current={active || undefined}
                        className={clsx(
                          'w-full rounded-lg px-3 py-2 text-left transition-colors',
                          active ? 'bg-brand-50 text-brand-700' : 'hover:bg-slate-50 text-slate-700',
                        )}
                      >
                        <div className="flex items-center gap-2">
                          <span className={clsx('truncate text-sm', active ? 'font-medium' : 'font-medium text-slate-800')}>{t.name}</span>
                          <span className="ml-auto flex shrink-0 items-center gap-1">
                            <ScopeBadges t={t} />
                          </span>
                        </div>
                        <div className="mt-0.5 font-mono text-xs text-slate-400 truncate">{t.code}</div>
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
          {(list.data?.total ?? 0) > PAGE_SIZE && (
            <Pagination className="pt-2 border-t border-slate-100" page={page} pageSize={PAGE_SIZE} total={list.data?.total ?? 0} pageSizeOptions={[PAGE_SIZE]} onChange={(p) => setPage(p)} />
          )}
        </div>

        {/* ── 右：条目 ── */}
        <div className="card p-5 min-h-[20rem]">
          {selectedId === null ? (
            <Empty icon={BookOpen} title="选择左侧字典" description="查看与维护条目" />
          ) : detail.isPending ? (
            <div className="flex justify-center py-12">
              <Spinner label="加载条目" />
            </div>
          ) : detail.isError ? (
            <ErrorState message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />
          ) : selected ? (
            <div className="space-y-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-base font-semibold text-slate-800 truncate">{selected.name}</h3>
                    <code className="font-mono text-xs text-slate-400">{selected.code}</code>
                    <ScopeBadges t={selected} />
                  </div>
                  <div className="mt-0.5 text-xs text-slate-400">
                    {text(selected.description)} · {items.length} 个条目 · 更新于 {formatDateTime(selected.updated_at)}
                  </div>
                  {selected.is_global && !isSuper && <div className="mt-1 text-xs text-amber-700">全局字典仅平台管理员可修改</div>}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {canEditItems && (
                    <Button variant="secondary" size="sm" icon={Plus} onClick={() => setItemModal({ mode: 'create', typeId: selected.id, nextSort })}>
                      新增条目
                    </Button>
                  )}
                  {canEditType && (
                    <Button variant="secondary" size="sm" icon={Pencil} onClick={() => setTypeDrawer({ mode: 'edit', dictType: selected })}>
                      编辑
                    </Button>
                  )}
                  {canDeleteType && (
                    <Button variant="danger" size="sm" icon={Trash2} title={selected.is_system ? '系统字典不可删除' : '删除字典及其条目'} onClick={() => setConfirmDeleteType(true)}>
                      删除
                    </Button>
                  )}
                </div>
              </div>

              <Table
                columns={itemColumns}
                data={items}
                rowKey="id"
                loading={detail.isFetching}
                actions={canEditItems ? { render: renderItemActions, width: 90 } : undefined}
                empty={<Empty size="sm" title="暂无条目" description={canEditItems ? '点击"新增条目"添加' : undefined} />}
              />
            </div>
          ) : null}
        </div>
      </div>

      <DictTypeDrawer
        state={typeDrawer}
        createsGlobal={isSuper && !viewTenantId}
        onClose={() => setTypeDrawer(null)}
        onSaved={(t) => {
          setTypeDrawer(null)
          setSelectedId(t.id)
          void invalidate()
        }}
      />
      <DictItemModal
        state={itemModal}
        onClose={() => setItemModal(null)}
        onSaved={() => {
          setItemModal(null)
          void invalidate()
        }}
      />
      <ConfirmDialog
        open={confirmDeleteType && selected !== undefined}
        danger
        title={`删除字典「${selected?.name ?? ''}」？`}
        description={selected?.is_system ? '这是系统字典，后端将拒绝删除。' : `将同时删除其 ${items.length} 个条目，删除后不可恢复。`}
        confirmText="删除"
        loading={removeType.isPending}
        onConfirm={() => {
          if (selected) removeType.mutate(selected.id)
        }}
        onCancel={() => setConfirmDeleteType(false)}
      />
      <ConfirmDialog
        open={deleteItemTarget !== null}
        danger
        title={`删除条目「${deleteItemTarget?.label ?? ''}」？`}
        description={`值 ${deleteItemTarget?.value ?? ''}。删除后不可恢复。`}
        confirmText="删除"
        loading={removeItem.isPending}
        onConfirm={() => {
          if (deleteItemTarget) removeItem.mutate(deleteItemTarget.id)
        }}
        onCancel={() => setDeleteItemTarget(null)}
      />
    </div>
  )
}
