import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { CheckCheck, ExternalLink } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { errorMessage } from '../../api/client'
import { listNotifications, markAllNotificationsRead, markNotificationsRead, notificationHref, notificationKeys, type NotificationListParams } from '../../api/notifications'
import type { Notification } from '../../api/types'
import { NOTIFICATION_TYPE_OPTIONS, notificationTypeMeta } from '../../components/notificationMeta'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import PageHeader from '../../components/ui/PageHeader'
import Pagination from '../../components/ui/Pagination'
import Select from '../../components/ui/Select'
import Spinner from '../../components/ui/Spinner'
import Tabs from '../../components/ui/Tabs'
import { useToast } from '../../components/ui/toast-context'
import { useRealtimeStore } from '../../store/realtime'
import { formatDateTime } from '../../utils/format'

type Scope = 'all' | 'unread'

const DEFAULT_PAGE_SIZE = 20

/** /notifications —— 我的全部通知：分页、未读筛选、按类型筛选、标记已读、跳转关联对象 */
export default function NotificationsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const toast = useToast()
  const unread = useRealtimeStore((s) => s.unread)
  const setUnread = useRealtimeStore((s) => s.setUnread)

  const [scope, setScope] = useState<Scope>('all')
  const [type, setType] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)

  const params: NotificationListParams = { page, pageSize, unread: scope === 'unread' ? true : undefined, type: type || undefined }
  const list = useQuery({
    queryKey: notificationKeys.list(params),
    queryFn: () => listNotifications(params),
    placeholderData: keepPreviousData,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: notificationKeys.all })

  const markRead = useMutation({
    mutationFn: (ids: string[]) => markNotificationsRead(ids),
    onSuccess: (_, ids) => {
      setUnread(Math.max(0, (unread ?? ids.length) - ids.length))
      void invalidate()
    },
    onError: (e) => toast.error('标记已读失败', errorMessage(e)),
  })

  const readAll = useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      setUnread(0)
      void invalidate()
      toast.success('已全部标记为已读')
    },
    onError: (e) => toast.error('操作失败', errorMessage(e)),
  })

  const open = (n: Notification) => {
    if (!n.read_at) markRead.mutate([n.id])
    const href = notificationHref(n)
    if (href) navigate(href)
  }

  const items = list.data?.items ?? []
  const pageUnreadIds = items.filter((n) => !n.read_at).map((n) => n.id)

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="通知中心"
        description="站内通知按时间倒序；点击可跳转到关联的审批 / 行程 / 车辆"
        extra={
          <>
            <Button variant="secondary" icon={CheckCheck} disabled={pageUnreadIds.length === 0} loading={markRead.isPending} onClick={() => markRead.mutate(pageUnreadIds)}>
              本页已读
            </Button>
            <Button icon={CheckCheck} loading={readAll.isPending} disabled={(unread ?? 0) === 0 && pageUnreadIds.length === 0} onClick={() => readAll.mutate()}>
              全部已读
            </Button>
          </>
        }
      />

      <div className="card p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Tabs<Scope>
            size="sm"
            items={[
              { key: 'all', label: '全部' },
              { key: 'unread', label: '未读', count: unread ?? undefined },
            ]}
            value={scope}
            onChange={(k) => {
              setScope(k)
              setPage(1)
            }}
          />
          <div className="w-full sm:w-44">
            <Select
              options={NOTIFICATION_TYPE_OPTIONS}
              placeholder="全部类型"
              value={type}
              onChange={(e) => {
                setType(e.target.value)
                setPage(1)
              }}
              aria-label="通知类型"
            />
          </div>
        </div>

        <div className="mt-3">
          {list.isPending ? (
            <div className="flex justify-center py-10">
              <Spinner label="加载中" />
            </div>
          ) : list.isError ? (
            <ErrorState message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
          ) : items.length === 0 ? (
            <Empty title={scope === 'unread' ? '没有未读通知' : '暂无通知'} />
          ) : (
            <ul className={clsx('divide-y divide-slate-100', list.isFetching && 'opacity-70')}>
              {items.map((n) => {
                const meta = notificationTypeMeta(n.type)
                const isUnread = !n.read_at
                const href = notificationHref(n)
                return (
                  <li key={n.id}>
                    <div className={clsx('flex items-start gap-3 rounded-lg px-2 py-3 transition-colors', isUnread ? 'bg-brand-50/40' : 'hover:bg-slate-50')}>
                      <span className={clsx('mt-2 h-2 w-2 shrink-0 rounded-full', isUnread ? 'bg-brand-600' : 'bg-slate-200')} aria-label={isUnread ? '未读' : '已读'} />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <button type="button" onClick={() => open(n)} className={clsx('text-left text-sm hover:underline', isUnread ? 'font-medium text-slate-800' : 'text-slate-700')}>
                            {n.title}
                          </button>
                          <Badge color={meta.color}>{meta.label}</Badge>
                          {href && (
                            <span className="inline-flex items-center gap-0.5 text-[11px] text-slate-400">
                              <ExternalLink size={11} />
                              可跳转
                            </span>
                          )}
                        </div>
                        {n.content && <p className="mt-1 whitespace-pre-line text-sm text-slate-600">{n.content}</p>}
                        <div className="mt-1 flex items-center gap-3 text-[11px] text-slate-400">
                          <span>{formatDateTime(n.created_at)}</span>
                          {n.read_at && <span>已读于 {formatDateTime(n.read_at)}</span>}
                        </div>
                      </div>
                      <div className="flex shrink-0 items-center gap-1">
                        {isUnread && (
                          <Button variant="ghost" size="sm" onClick={() => markRead.mutate([n.id])} disabled={markRead.isPending}>
                            标为已读
                          </Button>
                        )}
                        {href && (
                          <Button variant="secondary" size="sm" onClick={() => open(n)}>
                            查看
                          </Button>
                        )}
                      </div>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </div>

        {!list.isError && (
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
        )}
      </div>
    </div>
  )
}
