import { useQuery } from '@tanstack/react-query'
import { auditLogKeys, getAuditLog } from '../../../api/auditLogs'
import { errorMessage } from '../../../api/client'
import type { AuditLog } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import DescriptionList from '../../../components/ui/DescriptionList'
import Drawer from '../../../components/ui/Drawer'
import ErrorState from '../../../components/ui/ErrorState'
import JsonView from '../../../components/ui/JsonView'
import Spinner from '../../../components/ui/Spinner'
import { formatDateTime, text } from '../../../utils/format'
import { auditModuleLabel, methodColor, statusColor } from './schemas'

interface Props {
  /** 列表行（不含 before/after）；详情按 id 另行拉取 */
  log: AuditLog | null
  onClose: () => void
}

function hasBody(v: AuditLog['before']): boolean {
  return v !== null && v !== undefined
}

function Detail({ log }: { log: AuditLog }) {
  const detail = useQuery({ queryKey: auditLogKeys.detail(log.id), queryFn: () => getAuditLog(log.id), staleTime: 5 * 60_000 })
  const full = detail.data ?? log
  const before = full.before
  const after = full.after
  const both = hasBody(before) && hasBody(after)

  return (
    <div className="space-y-5">
      <DescriptionList
        columns={2}
        items={[
          { label: '时间', value: formatDateTime(full.created_at) },
          { label: '操作人', value: full.username ? <span>{full.username}</span> : '—' },
          { label: '模块', value: <span>{auditModuleLabel(full.module)}</span> },
          { label: '动作', value: <code className="font-mono text-xs">{full.action}</code> },
          { label: '摘要', value: text(full.summary), span: 2 },
          {
            label: '请求',
            value: (
              <span className="inline-flex items-center gap-1.5 font-mono text-xs">
                <Badge color={methodColor[full.method] ?? 'gray'}>{full.method}</Badge>
                <span className="break-all">{full.path}</span>
              </span>
            ),
            span: 2,
          },
          { label: '状态码', value: <Badge color={statusColor(full.status)}>{full.status}</Badge> },
          { label: 'IP', value: <span className="font-mono text-xs">{text(full.ip)}</span> },
          { label: '目标', value: full.target_id ? <span className="font-mono text-xs break-all">{full.target_type ? `${full.target_type} · ` : ''}{full.target_id}</span> : '—', span: 2 },
          { label: '请求 ID', value: <span className="font-mono text-xs break-all">{text(full.request_id)}</span>, span: 2 },
          { label: 'User-Agent', value: <span className="text-xs text-ink-muted break-all">{text(full.user_agent)}</span>, span: 2 },
        ]}
      />

      <div>
        <div className="mb-2 text-sm font-medium text-ink">变更数据</div>
        {detail.isPending ? (
          <div className="flex justify-center py-6">
            <Spinner size="sm" label="加载详情" />
          </div>
        ) : detail.isError ? (
          <ErrorState size="sm" message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />
        ) : !hasBody(before) && !hasBody(after) ? (
          <div className="rounded-lg bg-surface-3 px-3 py-6 text-center text-xs text-ink-faint">该操作未记录变更前后数据</div>
        ) : (
          <div className={both ? 'grid grid-cols-1 gap-3 md:grid-cols-2' : 'grid grid-cols-1 gap-3'}>
            {hasBody(before) && (
              <div className="min-w-0">
                <div className="mb-1 text-xs text-ink-faint">变更前 (before)</div>
                <JsonView value={before} maxHeight="50vh" />
              </div>
            )}
            {hasBody(after) && (
              <div className="min-w-0">
                <div className="mb-1 text-xs text-ink-faint">变更后 (after)</div>
                <JsonView value={after} maxHeight="50vh" />
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

/** 审计日志详情抽屉：基本信息 + before/after JSON */
export default function AuditLogDrawer({ log, onClose }: Props) {
  return (
    <Drawer open={log !== null} onClose={onClose} title={log ? `审计日志 #${log.id}` : ''} description={log?.summary ?? undefined} width="lg">
      {log && <Detail key={log.id} log={log} />}
    </Drawer>
  )
}
