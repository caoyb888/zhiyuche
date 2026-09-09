import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { Bell, CheckCheck } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { errorMessage } from '../api/client'
import { getUnreadCount, listNotifications, markAllNotificationsRead, markNotificationsRead, notificationHref, notificationKeys } from '../api/notifications'
import type { Notification } from '../api/types'
import { useRealtimeStore } from '../store/realtime'
import { formatDateTime } from '../utils/format'
import { notificationTypeMeta } from './notificationMeta'
import Badge from './ui/Badge'
import Empty from './ui/Empty'
import ErrorState from './ui/ErrorState'
import Spinner from './ui/Spinner'
import { useToast } from './ui/toast-context'

const RECENT = { page: 1, pageSize: 10 } as const

/** 顶栏铃铛：未读角标 + 最近 10 条下拉；WebSocket notification.new 事件实时 +1 */
export default function NotificationBell() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const storeUnread = useRealtimeStore((s) => s.unread)
  const setUnread = useRealtimeStore((s) => s.setUnread)
  const wsStatus = useRealtimeStore((s) => s.status)

  const count = useQuery({
    queryKey: notificationKeys.unread,
    queryFn: getUnreadCount,
    staleTime: 60_000,
    // WebSocket 断开时的兜底轮询
    refetchInterval: wsStatus === 'open' ? false : 60_000,
  })

  useEffect(() => {
    if (count.data) setUnread(count.data.unread)
  }, [count.data, setUnread])

  const unread = storeUnread ?? count.data?.unread ?? 0

  const recent = useQuery({
    queryKey: notificationKeys.list(RECENT),
    queryFn: () => listNotifications(RECENT),
    enabled: open,
    staleTime: 15_000,
  })

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const invalidate = () => queryClient.invalidateQueries({ queryKey: notificationKeys.all })

  const markRead = useMutation({
    mutationFn: (ids: string[]) => markNotificationsRead(ids),
    onSuccess: (_, ids) => {
      setUnread(Math.max(0, unread - ids.length))
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

  const onItemClick = (n: Notification) => {
    if (!n.read_at) markRead.mutate([n.id])
    setOpen(false)
    const href = notificationHref(n)
    if (href) navigate(href)
  }

  const items = recent.data?.items ?? []

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="relative flex h-8 w-8 items-center justify-center rounded-full text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700"
        aria-label={unread > 0 ? `通知，${unread} 条未读` : '通知'}
        aria-haspopup="menu"
        aria-expanded={open}
        title="通知"
      >
        <Bell size={18} />
        {unread > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-semibold leading-none text-white">
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>

      {open && (
        <div role="menu" className="absolute right-0 z-40 mt-1.5 w-[22rem] max-w-[calc(100vw-2rem)] rounded-xl border border-slate-100 bg-white py-1.5 shadow-lg slide-up">
          <div className="flex items-center justify-between border-b border-slate-100 px-3 py-2">
            <div className="flex items-center gap-2">
              <span className="text-sm font-semibold text-slate-800">通知</span>
              <span
                className={clsx('inline-flex items-center gap-1 text-[10px]', wsStatus === 'open' ? 'text-emerald-600' : 'text-slate-400')}
                title={wsStatus === 'open' ? '实时推送已连接' : wsStatus === 'reconnecting' ? '实时推送重连中' : '实时推送未连接'}
              >
                <span className={clsx('inline-block h-1.5 w-1.5 rounded-full', wsStatus === 'open' ? 'bg-emerald-500 pulse-dot' : wsStatus === 'reconnecting' ? 'bg-amber-400' : 'bg-slate-300')} />
                {wsStatus === 'open' ? '实时' : wsStatus === 'reconnecting' ? '重连中' : '离线'}
              </span>
            </div>
            <button
              type="button"
              onClick={() => readAll.mutate()}
              disabled={readAll.isPending || unread === 0}
              className="inline-flex items-center gap-1 text-xs text-brand-700 hover:underline disabled:cursor-not-allowed disabled:text-slate-300 disabled:no-underline"
            >
              <CheckCheck size={13} />
              全部已读
            </button>
          </div>

          <div className="max-h-96 overflow-y-auto">
            {recent.isPending ? (
              <div className="flex justify-center py-6">
                <Spinner size="sm" label="加载中" />
              </div>
            ) : recent.isError ? (
              <ErrorState size="sm" message={errorMessage(recent.error)} onRetry={() => void recent.refetch()} />
            ) : items.length === 0 ? (
              <Empty size="sm" title="暂无通知" />
            ) : (
              <ul>
                {items.map((n) => {
                  const meta = notificationTypeMeta(n.type)
                  const isUnread = !n.read_at
                  return (
                    <li key={n.id}>
                      <button
                        type="button"
                        role="menuitem"
                        onClick={() => onItemClick(n)}
                        className={clsx('flex w-full items-start gap-2.5 px-3 py-2.5 text-left transition-colors hover:bg-slate-50', isUnread && 'bg-brand-50/40')}
                      >
                        <span className={clsx('mt-1.5 h-2 w-2 shrink-0 rounded-full', isUnread ? 'bg-brand-600' : 'bg-transparent')} />
                        <span className="min-w-0 flex-1">
                          <span className="flex items-center gap-1.5">
                            <span className={clsx('truncate text-sm', isUnread ? 'font-medium text-slate-800' : 'text-slate-600')}>{n.title}</span>
                            <Badge color={meta.color} className="shrink-0 !px-1.5 !text-[10px]">
                              {meta.label}
                            </Badge>
                          </span>
                          {n.content && <span className="mt-0.5 line-clamp-2 block text-xs text-slate-500">{n.content}</span>}
                          <span className="mt-1 block text-[11px] text-slate-400">{formatDateTime(n.created_at)}</span>
                        </span>
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>

          <div className="border-t border-slate-100 px-3 py-2 text-center">
            <Link to="/notifications" onClick={() => setOpen(false)} className="text-xs text-brand-700 hover:underline">
              查看全部通知
            </Link>
          </div>
        </div>
      )}
    </div>
  )
}
