import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { deleteTemplate, listTemplates, templateKeys, updateTemplate, type TemplateListParams } from '../../../api/templates'
import type { NotifyChannel, Template } from '../../../api/types'
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
import Switch from '../../../components/ui/Switch'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { usePermission } from '../../../hooks/usePermission'
import { useAuthStore } from '../../../store/auth'
import { formatDateTime } from '../../../utils/format'
import { channelColor, channelLabel, channelOptions, isNotifyChannel } from './schemas'
import TemplateDrawer, { type TemplateDrawerState } from './TemplateDrawer'

interface Filters {
  keyword: string
  channel: NotifyChannel | ''
}

const EMPTY_FILTERS: Filters = { keyword: '', channel: '' }
const DEFAULT_PAGE_SIZE = 20

export default function TemplatesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can, isSuper } = usePermission()
  const viewTenantId = useAuthStore((s) => s.viewTenantId)

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)

  const [drawer, setDrawer] = useState<TemplateDrawerState>(null)
  const [deleteTarget, setDeleteTarget] = useState<Template | null>(null)
  const [togglingId, setTogglingId] = useState<string | null>(null)

  const params: TemplateListParams = { keyword: applied.keyword || undefined, channel: applied.channel || undefined, page, pageSize, sort }
  const list = useQuery({
    queryKey: templateKeys.list(params),
    queryFn: () => listTemplates(params),
    placeholderData: keepPreviousData,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: templateKeys.all })

  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => updateTemplate(id, { enabled }),
    onMutate: ({ id }) => setTogglingId(id),
    onSuccess: (t) => toast.success(t.enabled ? '模板已启用' : '模板已停用'),
    onError: (e) => toast.error('操作失败', errorMessage(e)),
    onSettled: () => {
      setTogglingId(null)
      void invalidate()
    },
  })

  const remove = useMutation({
    mutationFn: (id: string) => deleteTemplate(id),
    onSuccess: () => {
      toast.success('模板已删除')
      setDeleteTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('删除失败', errorMessage(e)),
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

  // 全局模板只有超级管理员能改
  const writable = (t: Template) => !t.is_global || isSuper
  const canUpdate = can('system:template:update')
  const canDelete = can('system:template:delete')
  const hasRowActions = canUpdate || canDelete

  const columns: Column<Template>[] = [
    { key: 'code', title: '代码', sortable: true, render: (t) => <code className="font-mono text-xs text-ink-strong">{t.code}</code> },
    { key: 'channel', title: '渠道', sortable: true, width: 90, render: (t) => <Badge color={channelColor[t.channel]}>{channelLabel[t.channel]}</Badge> },
    {
      key: 'title',
      title: '标题',
      sortable: true,
      render: (t) => (
        <div className="max-w-[22rem]">
          <div className="truncate font-medium text-ink-strong">{t.title}</div>
          <div className="truncate text-xs text-ink-faint" title={t.content}>
            {t.content}
          </div>
        </div>
      ),
    },
    {
      key: 'enabled',
      title: '启用',
      width: 80,
      render: (t) => (
        <Switch
          size="sm"
          checked={t.enabled}
          disabled={!canUpdate || !writable(t) || togglingId === t.id}
          onChange={(v) => toggle.mutate({ id: t.id, enabled: v })}
        />
      ),
    },
    { key: 'is_global', title: '作用域', width: 80, render: (t) => (t.is_global ? <Badge color="purple">全局</Badge> : <Badge color="blue">租户</Badge>) },
    { key: 'updated_at', title: '更新时间', render: (t) => <span className="text-xs text-ink-muted">{formatDateTime(t.updated_at)}</span> },
  ]

  const renderActions = (t: Template) => {
    if (!writable(t)) return <span className="text-xs text-ink-faint">仅平台可改</span>
    return (
      <div className="flex items-center justify-end gap-0.5">
        {canUpdate && <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', template: t })} />}
        {canDelete && (
          <Button
            variant="ghost"
            size="sm"
            icon={Trash2}
            className="!px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200"
            title="删除"
            aria-label="删除"
            onClick={() => setDeleteTarget(t)}
          />
        )}
      </div>
    )
  }

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="通知模板"
        description="站内信 / 微信 / 短信通知内容；租户模板覆盖同代码同渠道的全局模板"
        extra={
          <Can perm="system:template:create">
            <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
              新建模板
            </Button>
          </Can>
        }
      />

      <FilterBar
        keyword={draft.keyword}
        onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))}
        keywordPlaceholder="代码 / 标题"
        onSearch={search}
        onReset={reset}
        loading={list.isFetching}
      >
        <div className="w-full sm:w-36">
          <Select
            options={channelOptions}
            placeholder="全部渠道"
            value={draft.channel}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, channel: isNotifyChannel(v) ? v : '' }))
            }}
            aria-label="渠道"
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
              actions={hasRowActions ? { render: renderActions, width: 110 } : undefined}
              empty={<Empty title="没有匹配的模板" description="调整筛选条件，或新建模板" />}
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

      <TemplateDrawer
        state={drawer}
        createsGlobal={isSuper && !viewTenantId}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
        }}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除模板「${deleteTarget?.title ?? ''}」？`}
        description={`${deleteTarget?.code ?? ''} · ${deleteTarget ? channelLabel[deleteTarget.channel] : ''}。删除后不可恢复${deleteTarget && !deleteTarget.is_global ? '；若存在同代码同渠道的全局模板，将回退到全局模板' : ''}。`}
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
