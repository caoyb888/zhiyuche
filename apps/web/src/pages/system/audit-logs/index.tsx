import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { auditLogKeys, listAuditLogs, type AuditLogFilter, type AuditLogListParams } from '../../../api/auditLogs'
import { errorMessage } from '../../../api/client'
import type { AuditLog } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import DateTimeInput from '../../../components/ui/DateTimeInput'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import FilterBar from '../../../components/ui/FilterBar'
import Input from '../../../components/ui/Input'
import PageHeader from '../../../components/ui/PageHeader'
import Pagination from '../../../components/ui/Pagination'
import Select from '../../../components/ui/Select'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { formatDateTime, localToRFC3339, text } from '../../../utils/format'
import AuditLogDrawer from './AuditLogDrawer'
import { auditModuleLabel, auditModuleOptions, methodColor, statusColor } from './schemas'

interface Filters {
  keyword: string
  username: string
  module: string
  action: string
  target_id: string
  /** datetime-local 值（本地时间） */
  from: string
  to: string
}

const EMPTY_FILTERS: Filters = { keyword: '', username: '', module: '', action: '', target_id: '', from: '', to: '' }
const DEFAULT_PAGE_SIZE = 20

function toFilter(f: Filters): AuditLogFilter {
  return {
    keyword: f.keyword.trim() || undefined,
    username: f.username.trim() || undefined,
    module: f.module || undefined,
    action: f.action.trim() || undefined,
    target_id: f.target_id.trim() || undefined,
    from: localToRFC3339(f.from),
    to: localToRFC3339(f.to),
  }
}

export default function AuditLogsPage() {
  const toast = useToast()
  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)
  const [current, setCurrent] = useState<AuditLog | null>(null)

  const params: AuditLogListParams = { ...toFilter(applied), page, pageSize, sort }
  const list = useQuery({
    queryKey: auditLogKeys.list(params),
    queryFn: () => listAuditLogs(params),
    placeholderData: keepPreviousData,
  })

  const search = () => {
    if (draft.from && draft.to && draft.from > draft.to) {
      toast.warning('时间范围无效', '结束时间不能早于开始时间')
      return
    }
    setApplied(draft)
    setPage(1)
  }
  const reset = () => {
    setDraft(EMPTY_FILTERS)
    setApplied(EMPTY_FILTERS)
    setPage(1)
    setSort(undefined)
  }

  const columns: Column<AuditLog>[] = [
    { key: 'created_at', title: '时间', sortable: true, width: 150, render: (l) => <span className="text-xs text-ink whitespace-nowrap">{formatDateTime(l.created_at)}</span> },
    { key: 'username', title: '用户', render: (l) => <span className="font-medium text-ink-strong">{text(l.username)}</span> },
    { key: 'module', title: '模块', sortable: true, render: (l) => <span className="text-ink">{auditModuleLabel(l.module)}</span> },
    { key: 'action', title: '动作', sortable: true, render: (l) => <code className="font-mono text-xs text-ink">{l.action}</code> },
    {
      key: 'summary',
      title: '摘要',
      render: (l) => (
        <span className="block max-w-[20rem] truncate text-ink" title={l.summary ?? undefined}>
          {text(l.summary)}
        </span>
      ),
    },
    {
      key: 'path',
      title: '请求',
      render: (l) => (
        <span className="inline-flex items-center gap-1.5">
          <Badge color={methodColor[l.method] ?? 'gray'}>{l.method}</Badge>
          <code className="block max-w-[16rem] truncate font-mono text-xs text-ink-muted" title={l.path}>
            {l.path}
          </code>
        </span>
      ),
    },
    { key: 'status', title: '状态码', width: 80, render: (l) => <Badge color={statusColor(l.status)}>{l.status}</Badge> },
    { key: 'ip', title: 'IP', render: (l) => <span className="font-mono text-xs text-ink-muted">{text(l.ip)}</span> },
  ]

  return (
    <div className="space-y-4 slide-up">
      <PageHeader title="审计日志" description="所有非 GET 请求自动记录；点击行查看变更前后数据" />

      <FilterBar
        keyword={draft.keyword}
        onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))}
        keywordPlaceholder="摘要 / 路径"
        onSearch={search}
        onReset={reset}
        loading={list.isFetching}
      >
        <div className="w-full sm:w-32">
          <Input value={draft.username} onChange={(e) => setDraft((d) => ({ ...d, username: e.target.value }))} placeholder="用户名" aria-label="用户名" />
        </div>
        <div className="w-full sm:w-44">
          <Select options={auditModuleOptions} placeholder="全部模块" value={draft.module} onChange={(e) => setDraft((d) => ({ ...d, module: e.target.value }))} aria-label="模块" />
        </div>
        <div className="w-full sm:w-28">
          <Input value={draft.action} onChange={(e) => setDraft((d) => ({ ...d, action: e.target.value }))} placeholder="动作" aria-label="动作" />
        </div>
        <div className="w-full sm:w-44">
          <Input value={draft.target_id} onChange={(e) => setDraft((d) => ({ ...d, target_id: e.target.value }))} placeholder="目标 ID" aria-label="目标 ID" className="font-mono text-xs" />
        </div>
        <div className="flex w-full items-center gap-1.5 sm:w-auto">
          <DateTimeInput value={draft.from} onChange={(v) => setDraft((d) => ({ ...d, from: v }))} aria-label="开始时间" className="text-xs" />
          <span className="text-xs text-ink-faint">至</span>
          <DateTimeInput value={draft.to} onChange={(v) => setDraft((d) => ({ ...d, to: v }))} aria-label="结束时间" className="text-xs" />
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
              rowKey={(l) => String(l.id)}
              loading={list.isFetching}
              sort={sort}
              onSortChange={(s) => {
                setSort(s)
                setPage(1)
              }}
              onRowClick={setCurrent}
              empty={<Empty title="没有匹配的日志" description="调整筛选条件或时间范围" />}
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

      <AuditLogDrawer log={current} onClose={() => setCurrent(null)} />
    </div>
  )
}
