import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { dashboardKeys } from '../api/dashboard'
import { deviceKeys } from '../api/devices'
import { notificationKeys } from '../api/notifications'
import type { Notification, Page, Vehicle, VehicleLive } from '../api/types'
import { vehicleKeys } from '../api/vehicles'
import { useToast, type ToastApi } from '../components/ui/toast-context'
import { useAuthStore } from '../store/auth'
import { mergeVehicleLive, useRealtimeStore, type VehicleLivePatch } from '../store/realtime'

/** 服务端消息 `{type, ts, data}` */
interface WsEvent {
  type?: string
  ts?: string
  data?: unknown
}

interface DeviceOnlineData {
  device_id?: string
  vehicle_id?: string | null
  online?: boolean
}

const PING = JSON.stringify({ type: 'ping' })
const PING_INTERVAL_MS = 25_000
const RECONNECT_BASE_MS = 1_000
const RECONNECT_MAX_MS = 30_000

/** 审批 / 行程页面（后续任务）约定使用的 query key 前缀 */
const APPROVAL_KEY = ['approvals'] as const
const TRIP_KEY = ['trips'] as const

/**
 * WebSocket 地址：VITE_WS_URL（完整地址）优先；否则按当前页面协议/主机 + VITE_API_BASE + /ws 推导，
 * dev 模式下与 /api 一样经 vite proxy 转发。
 */
export function buildWsUrl(token: string): string {
  const explicit = (import.meta.env.VITE_WS_URL ?? '').trim()
  let base: string
  if (explicit) {
    base = explicit
  } else {
    const apiBase = (import.meta.env.VITE_API_BASE ?? '/api/v1').replace(/\/$/, '')
    if (/^https?:\/\//i.test(apiBase)) {
      base = `${apiBase.replace(/^http/i, 'ws')}/ws`
    } else {
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      base = `${proto}//${window.location.host}${apiBase.startsWith('/') ? apiBase : `/${apiBase}`}/ws`
    }
  }
  return `${base}${base.includes('?') ? '&' : '?'}access_token=${encodeURIComponent(token)}`
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null
}

/** vehicle.status：更新 realtime store，并就地更新 react-query 中的实时状态 / 详情 / 列表缓存 */
function applyVehicleStatus(queryClient: QueryClient, patch: VehicleLivePatch): void {
  const id = patch.vehicle_id
  const merged = useRealtimeStore.getState().upsertVehicle(patch)

  queryClient.setQueryData<VehicleLive[]>(vehicleKeys.status, (old) => {
    if (!old) return old
    return old.some((v) => v.vehicle_id === id) ? old.map((v) => (v.vehicle_id === id ? mergeVehicleLive(v, patch) : v)) : [...old, merged]
  })
  queryClient.setQueryData<VehicleLive>(vehicleKeys.live(id), (old) => (old ? mergeVehicleLive(old, patch) : old))
  queryClient.setQueryData<Vehicle>(vehicleKeys.detail(id), (old) => {
    if (!old) return old
    const live = mergeVehicleLive(old.live, patch)
    return { ...old, status: live.status, live }
  })
  queryClient.setQueriesData<Page<Vehicle>>({ queryKey: [...vehicleKeys.all, 'list'] }, (old) => {
    if (!old || !old.items.some((v) => v.id === id)) return old
    return {
      ...old,
      items: old.items.map((v) => {
        if (v.id !== id) return v
        const live = mergeVehicleLive(v.live, patch)
        return { ...v, status: live.status, live }
      }),
    }
  })
}

function handleEvent(raw: unknown, queryClient: QueryClient, toast: ToastApi): void {
  if (!isRecord(raw)) return
  const ev = raw as WsEvent
  const type = ev.type
  if (!type || type === 'pong') return
  const store = useRealtimeStore.getState()
  store.touch(ev.ts)
  const invalidate = (...keys: ReadonlyArray<readonly unknown[]>) => {
    for (const key of keys) void queryClient.invalidateQueries({ queryKey: key })
  }

  switch (type) {
    case 'vehicle.status': {
      if (isRecord(ev.data) && typeof ev.data.vehicle_id === 'string') {
        applyVehicleStatus(queryClient, ev.data as VehicleLivePatch)
      }
      break
    }
    case 'notification.new': {
      store.incUnread()
      if (isRecord(ev.data)) {
        const n = ev.data as Partial<Notification>
        toast.info(n.title || '新通知', n.content)
      }
      invalidate(notificationKeys.all)
      break
    }
    case 'approval.updated':
      invalidate(APPROVAL_KEY, dashboardKeys.all)
      break
    case 'trip.started':
    case 'trip.ended':
    case 'trip.event':
      invalidate(TRIP_KEY, dashboardKeys.all)
      break
    case 'device.online': {
      invalidate(deviceKeys.all, dashboardKeys.all)
      if (isRecord(ev.data)) {
        const d = ev.data as DeviceOnlineData
        if (typeof d.vehicle_id === 'string' && typeof d.online === 'boolean' && store.vehicles[d.vehicle_id]) {
          applyVehicleStatus(queryClient, { vehicle_id: d.vehicle_id, online: d.online })
        }
      }
      break
    }
    default:
      break
  }
}

/**
 * 登录后维持一条 WebSocket 连接（在 Layout 挂载）：
 * - 地址 `${VITE_WS_URL ?? 推导}/ws?access_token=`；每 25s 发 `{"type":"ping"}`
 * - 断线指数退避重连（1s → 30s，带抖动）；access token 变化（刷新 / 换账号）时用新 token 重连
 * - 退出登录（token 清空）或 Layout 卸载时断开
 * - 事件分发：vehicle.status 就地更新缓存；notification.new 未读 +1 并 Toast；其余失效对应 query
 */
export function useWebSocket(): void {
  const queryClient = useQueryClient()
  const toast = useToast()
  const token = useAuthStore((s) => s.accessToken)

  useEffect(() => {
    const store = useRealtimeStore.getState()
    if (!token || typeof WebSocket === 'undefined') {
      store.reset()
      return
    }

    let ws: WebSocket | null = null
    let disposed = false
    let attempt = 0
    let everOpened = false
    let reconnectTimer: number | undefined
    let pingTimer: number | undefined

    const clearTimers = () => {
      if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
      if (pingTimer !== undefined) window.clearInterval(pingTimer)
      reconnectTimer = undefined
      pingTimer = undefined
    }

    const scheduleReconnect = () => {
      if (disposed) return
      attempt += 1
      const backoff = Math.min(RECONNECT_MAX_MS, RECONNECT_BASE_MS * 2 ** (attempt - 1))
      const delay = Math.round(backoff * (0.8 + Math.random() * 0.4))
      useRealtimeStore.getState().setStatus('reconnecting', attempt)
      reconnectTimer = window.setTimeout(connect, delay)
    }

    const connect = () => {
      if (disposed) return
      // 重连时总是使用最新的 access token
      const current = useAuthStore.getState().accessToken
      if (!current) return
      useRealtimeStore.getState().setStatus(attempt === 0 ? 'connecting' : 'reconnecting', attempt)
      let sock: WebSocket
      try {
        sock = new WebSocket(buildWsUrl(current))
      } catch {
        scheduleReconnect()
        return
      }
      ws = sock
      sock.onopen = () => {
        if (sock !== ws) return
        attempt = 0
        useRealtimeStore.getState().setStatus('open', 0)
        pingTimer = window.setInterval(() => {
          if (sock.readyState === WebSocket.OPEN) sock.send(PING)
        }, PING_INTERVAL_MS)
        if (everOpened) {
          // 断线期间可能漏掉事件：重新同步实时状态与未读数
          void queryClient.invalidateQueries({ queryKey: vehicleKeys.status })
          void queryClient.invalidateQueries({ queryKey: notificationKeys.all })
        }
        everOpened = true
      }
      sock.onmessage = (e: MessageEvent<unknown>) => {
        if (sock !== ws || typeof e.data !== 'string') return
        let parsed: unknown
        try {
          parsed = JSON.parse(e.data)
        } catch {
          return
        }
        handleEvent(parsed, queryClient, toast)
      }
      sock.onclose = () => {
        if (sock !== ws) return
        if (pingTimer !== undefined) window.clearInterval(pingTimer)
        pingTimer = undefined
        ws = null
        scheduleReconnect()
      }
      sock.onerror = () => {
        // 浏览器随后必定触发 onclose，在那里统一重连
      }
    }

    connect()

    return () => {
      disposed = true
      clearTimers()
      const sock = ws
      ws = null
      if (sock) {
        sock.onclose = null
        sock.onmessage = null
        sock.close()
      }
      useRealtimeStore.getState().setStatus('closed')
    }
  }, [token, queryClient, toast])
}
