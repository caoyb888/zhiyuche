import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { AlertTriangle, Car, ChevronRight, ClipboardCheck, Cpu, Route, WifiOff, Wrench, Zap, type LucideIcon } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { chargingKeys, getChargingSummary } from '../api/charging'
import { errorMessage } from '../api/client'
import { dashboardKeys, getDashboardOverview } from '../api/dashboard'
import type { DashboardEvent, DashboardOverview, TripEventType, VehicleLive } from '../api/types'
import { listVehicleLiveStatus, vehicleKeys } from '../api/vehicles'
import VehicleMap from '../components/map/VehicleMap'
import Empty from '../components/ui/Empty'
import ErrorState from '../components/ui/ErrorState'
import Spinner from '../components/ui/Spinner'
import { usePermission } from '../hooks/usePermission'
import { useRealtimeStore } from '../store/realtime'
import { formatDateTime, text } from '../utils/format'

interface StatCard {
  key: string
  label: string
  value: number | null
  icon: LucideIcon
  color: string
  bg: string
  href?: string
}

type EventLevel = 'red' | 'amber' | 'gray'

const EVENT_META: Record<TripEventType, { label: string; level: EventLevel }> = {
  start: { label: '行程开始', level: 'gray' },
  end: { label: '行程结束', level: 'gray' },
  deviation: { label: '路线偏离', level: 'red' },
  overspeed: { label: '超速', level: 'red' },
  low_soc: { label: '低电量', level: 'amber' },
  sign_on: { label: '灯牌亮起', level: 'gray' },
  sign_off: { label: '灯牌熄灭', level: 'gray' },
  cancel: { label: '行程取消', level: 'gray' },
}

const LEVEL_CLASS: Record<EventLevel, { bg: string; icon: string; text: string }> = {
  red: { bg: 'bg-danger-500/10 border border-danger-500/30', icon: 'text-danger-400', text: 'text-danger-200' },
  amber: { bg: 'bg-warn-500/10 border border-warn-500/30', icon: 'text-warn-400', text: 'text-warn-200' },
  gray: { bg: 'bg-surface-3 border border-line', icon: 'text-ink-faint', text: 'text-ink-muted' },
}

function num(v: unknown): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

/** 事件 payload → 一句话 */
function describeEvent(e: DashboardEvent): string {
  const p = (e.payload ?? {}) as Record<string, unknown>
  switch (e.type) {
    case 'overspeed': {
      const speed = num(p.speed)
      const limit = num(p.limit)
      return speed !== null ? `车速 ${Math.round(speed)} km/h${limit !== null ? `（限速 ${Math.round(limit)}）` : ''}` : '超速行驶'
    }
    case 'deviation': {
      const d = num(p.distance_m)
      return d !== null ? `偏离计划路线约 ${d >= 1000 ? `${(d / 1000).toFixed(1)} km` : `${Math.round(d)} m`}` : '偏离计划路线'
    }
    case 'low_soc': {
      const soc = num(p.soc)
      return soc !== null ? `电量仅剩 ${Math.round(soc)}%` : '电量过低'
    }
    default:
      return EVENT_META[e.type]?.label ?? e.type
  }
}

/** 总览接口不可用时，用实时状态列表推算车辆计数 */
function countsFromLive(list: VehicleLive[]): DashboardOverview['vehicles'] {
  const c = { total: list.length, idle: 0, in_use: 0, charging: 0, maintenance: 0, disabled: 0, offline: 0 }
  for (const v of list) {
    c[v.status] += 1
    if (v.status !== 'disabled' && !v.online) c.offline += 1
  }
  return c
}

/** 充电统计卡：充电中事务数 / 今日充电 kWh（GET /charging/summary，需 charging:view），点击进入充电管理 */
function ChargingCard() {
  const { can } = usePermission()
  const enabled = can('charging:view')
  const summary = useQuery({ queryKey: chargingKeys.summary(), queryFn: () => getChargingSummary(), enabled, staleTime: 30_000, refetchInterval: 60_000 })
  const s = summary.data
  const body = (
    <>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-ink">充电</h2>
        {enabled ? <ChevronRight size={16} className="text-ink-disabled" /> : <Zap size={15} className="text-ink-disabled" />}
      </div>
      {!enabled ? (
        <div className="py-4 text-center text-xs text-ink-faint">无充电管理权限</div>
      ) : s ? (
        <div className="mt-3 grid grid-cols-2 gap-x-3 gap-y-3">
          <div>
            <div className={clsx('font-mono text-xl font-semibold', s.ongoing > 0 ? 'text-warn-200' : 'text-ink-strong')}>{s.ongoing}</div>
            <div className="text-xs text-ink-faint">充电中</div>
          </div>
          <div>
            <div className="font-mono text-xl font-semibold text-ink-strong">{(s.today?.kwh ?? 0).toFixed(1)}</div>
            <div className="text-xs text-ink-faint">今日充电 kWh（{s.today?.sessions ?? 0} 次）</div>
          </div>
          <div>
            <div className="font-mono text-xl font-semibold text-ink-strong">
              {s.piles?.online ?? 0}
              <span className="text-sm font-normal text-ink-faint"> / {s.piles?.total ?? 0}</span>
            </div>
            <div className="text-xs text-ink-faint">桩在线{(s.piles?.faulted ?? 0) > 0 && <span className="ml-1 text-danger-200">故障 {s.piles?.faulted}</span>}</div>
          </div>
          <div>
            <div className={clsx('font-mono text-xl font-semibold', s.pending_review > 0 ? 'text-danger-200' : 'text-ink-strong')}>{s.pending_review}</div>
            <div className="text-xs text-ink-faint">待复核</div>
          </div>
        </div>
      ) : summary.isPending ? (
        <div className="flex justify-center py-6">
          <Spinner size="sm" />
        </div>
      ) : (
        <div className="py-4 text-center text-xs text-ink-faint">{summary.isError ? '充电汇总暂不可用' : '暂无数据'}</div>
      )}
    </>
  )
  if (!enabled) return <div className="card p-4">{body}</div>
  return (
    <Link to="/charging" className="card block p-4 transition-colors hover:border-brand-600/60">
      {body}
    </Link>
  )
}

function StatTile({ s }: { s: StatCard }) {
  const body = (
    <>
      <div className={clsx('flex h-10 w-10 shrink-0 items-center justify-center rounded-xl', s.bg)}>
        <s.icon size={18} className={s.color} />
      </div>
      <div className="min-w-0">
        <div className={clsx('font-mono text-2xl font-semibold tracking-tight', s.color)}>{s.value === null ? '—' : s.value}</div>
        <div className="mt-0.5 text-xs text-ink-faint">{s.label}</div>
      </div>
      {s.href && <ChevronRight size={16} className="ml-auto text-ink-disabled" />}
    </>
  )
  if (s.href) {
    return (
      <Link to={s.href} className="stat-card flex items-center gap-3 transition-colors hover:border-brand-600/60 hover:bg-surface-3">
        {body}
      </Link>
    )
  }
  return <div className="stat-card flex items-center gap-3">{body}</div>
}

export default function Dashboard() {
  const overview = useQuery({ queryKey: dashboardKeys.overview, queryFn: getDashboardOverview, staleTime: 30_000, refetchInterval: 60_000 })
  // 实时状态：WebSocket vehicle.status 就地更新此缓存；轮询作为断线兜底
  const status = useQuery({ queryKey: vehicleKeys.status, queryFn: listVehicleLiveStatus, staleTime: 30_000, refetchInterval: 60_000 })
  const setVehicles = useRealtimeStore((s) => s.setVehicles)
  const wsStatus = useRealtimeStore((s) => s.status)

  useEffect(() => {
    if (status.data) setVehicles(status.data)
  }, [status.data, setVehicles])

  const vehicles = useMemo(() => status.data ?? [], [status.data])
  const ov = overview.data
  const counts = ov?.vehicles ?? (status.data ? countsFromLive(vehicles) : null)

  const stats: StatCard[] = [
    { key: 'in_use', label: '在途车辆', value: counts?.in_use ?? null, icon: Car, color: 'text-brand-300', bg: 'bg-brand-600/15' },
    { key: 'idle', label: '空闲车辆', value: counts?.idle ?? null, icon: Car, color: 'text-ev-200', bg: 'bg-ev-500/15' },
    { key: 'charging', label: '充电中', value: counts?.charging ?? null, icon: Zap, color: 'text-warn-200', bg: 'bg-warn-500/15' },
    { key: 'maintenance', label: '维保中', value: counts?.maintenance ?? null, icon: Wrench, color: 'text-danger-200', bg: 'bg-danger-500/15' },
    { key: 'offline', label: '离线', value: counts?.offline ?? null, icon: WifiOff, color: 'text-ink-muted', bg: 'bg-white/[0.06]' },
    { key: 'todo', label: '待我审批', value: ov?.approvals_todo ?? null, icon: ClipboardCheck, color: 'text-violet-200', bg: 'bg-violet-500/15', href: '/approval?scope=todo' },
  ]

  const today = ov?.today
  const devices = ov?.devices
  const deviceTotal = devices?.total ?? 0
  const deviceOnline = devices?.online ?? 0
  const onlineRate = deviceTotal > 0 ? Math.round((deviceOnline / deviceTotal) * 100) : null
  const events = ov?.recent_events ?? []
  const bothFailed = overview.isError && status.isError

  return (
    <div className="space-y-5 slide-up">
      {bothFailed && (
        <div className="card p-4">
          <ErrorState
            message={errorMessage(overview.error)}
            onRetry={() => {
              void overview.refetch()
              void status.refetch()
            }}
          />
        </div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        {stats.map((s) => (
          <StatTile key={s.key} s={s} />
        ))}
      </div>

      <div className="grid gap-5 lg:grid-cols-3">
        {/* Map */}
        <div className="card p-4 lg:col-span-2">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-ink">车辆实时位置</h2>
            <div className="flex items-center gap-3 text-xs text-ink-faint">
              <span className={clsx('inline-flex items-center gap-1', wsStatus === 'open' && 'text-tech-200')}>
                <span className={clsx('inline-block h-1.5 w-1.5 rounded-full', wsStatus === 'open' ? 'bg-tech-400 pulse-dot' : wsStatus === 'reconnecting' ? 'bg-warn-400' : 'bg-ink-disabled')} />
                {wsStatus === 'open' ? '实时推送' : wsStatus === 'reconnecting' ? '重连中' : '轮询刷新'}
              </span>
              {status.data && <span>{vehicles.length} 辆</span>}
            </div>
          </div>
          {status.isError ? (
            <ErrorState message={errorMessage(status.error)} onRetry={() => void status.refetch()} />
          ) : status.isPending ? (
            <div className="flex h-[420px] items-center justify-center rounded-xl bg-surface-3">
              <Spinner label="加载车辆状态…" />
            </div>
          ) : (
            <VehicleMap
              vehicles={vehicles}
              height={420}
              fitKey={vehicles.length}
              renderActions={(v) => (
                <Link to={`/assets/vehicles?id=${encodeURIComponent(v.vehicle_id)}`} className="inline-flex items-center gap-1 text-xs text-brand-300 hover:underline">
                  查看车辆详情
                  <ChevronRight size={12} />
                </Link>
              )}
            />
          )}
        </div>

        {/* Right column */}
        <div className="space-y-5">
          <Link to="/approval?scope=todo" className="card block p-4 transition-colors hover:border-brand-600/60">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-ink">待办审批</h2>
              <ChevronRight size={16} className="text-ink-disabled" />
            </div>
            <div className="mt-2 flex items-end gap-2">
              <span className="font-mono text-3xl font-semibold tracking-tight text-violet-200">{ov ? ov.approvals_todo : overview.isPending ? '…' : '—'}</span>
              <span className="mb-1 text-xs text-ink-faint">条待我处理</span>
            </div>
            {typeof ov?.approvals_pending === 'number' && <div className="mt-1 text-xs text-ink-faint">租户内待审批共 {ov.approvals_pending} 条</div>}
            {overview.isError && <div className="mt-2 text-xs text-warn-200">总览数据暂不可用</div>}
          </Link>

          <div className="card p-4">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-ink">今日行程</h2>
              <Route size={15} className="text-ink-disabled" />
            </div>
            {today ? (
              <div className="mt-3 grid grid-cols-2 gap-x-3 gap-y-3">
                <div>
                  <div className="font-mono text-xl font-semibold text-ink-strong">{today.trips}</div>
                  <div className="text-xs text-ink-faint">行程数（进行中 {today.ongoing}）</div>
                </div>
                <div>
                  <div className="font-mono text-xl font-semibold text-ink-strong">{today.distance_km.toFixed(1)}</div>
                  <div className="text-xs text-ink-faint">总里程 km</div>
                </div>
                <div>
                  <div className="font-mono text-xl font-semibold text-ink-strong">{today.energy_kwh.toFixed(1)}</div>
                  <div className="text-xs text-ink-faint">耗电 kWh</div>
                </div>
                <div>
                  <div className="font-mono text-xl font-semibold text-ink-strong">
                    {today.official_trips ?? '—'}
                    {typeof today.deviation_trips === 'number' && today.deviation_trips > 0 && <span className="ml-1 text-sm font-medium text-danger-200">/ 偏离 {today.deviation_trips}</span>}
                  </div>
                  <div className="text-xs text-ink-faint">公务行程</div>
                </div>
              </div>
            ) : overview.isPending ? (
              <div className="flex justify-center py-6">
                <Spinner size="sm" />
              </div>
            ) : (
              <div className="py-4 text-center text-xs text-ink-faint">暂无数据</div>
            )}
          </div>

          <ChargingCard />

          <div className="card p-4">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-ink">设备在线率</h2>
              <Cpu size={15} className="text-ink-disabled" />
            </div>
            {devices ? (
              <>
                <div className="mt-2 flex items-end gap-2">
                  <span className={clsx('font-mono text-3xl font-semibold tracking-tight', onlineRate === null ? 'text-ink-faint' : onlineRate >= 90 ? 'text-ev-200' : onlineRate >= 70 ? 'text-warn-200' : 'text-danger-200')}>
                    {onlineRate === null ? '—' : `${onlineRate}%`}
                  </span>
                  <span className="mb-1 text-xs text-ink-faint">
                    {deviceOnline} / {deviceTotal} 台在线
                  </span>
                </div>
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-surface-4">
                  <div className={clsx('h-full rounded-full transition-all', onlineRate === null ? 'bg-line-strong' : onlineRate >= 90 ? 'bg-ev-500' : onlineRate >= 70 ? 'bg-warn-500' : 'bg-danger-500')} style={{ width: `${onlineRate ?? 0}%` }} />
                </div>
              </>
            ) : overview.isPending ? (
              <div className="flex justify-center py-6">
                <Spinner size="sm" />
              </div>
            ) : (
              <div className="py-4 text-center text-xs text-ink-faint">暂无数据</div>
            )}
          </div>
        </div>
      </div>

      {/* Recent events */}
      <div className="card p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-ink">最近异常事件</h2>
          <Link to="/trips" className="text-xs text-brand-300 hover:underline">
            查看行程
          </Link>
        </div>
        {overview.isPending ? (
          <div className="flex justify-center py-6">
            <Spinner size="sm" label="加载中" />
          </div>
        ) : overview.isError ? (
          <ErrorState size="sm" message={errorMessage(overview.error)} onRetry={() => void overview.refetch()} />
        ) : events.length === 0 ? (
          <Empty size="sm" title="暂无异常事件" description="超速、路线偏离、低电量等事件会显示在这里" />
        ) : (
          <div className="space-y-2">
            {events.map((e) => {
              const meta = EVENT_META[e.type] ?? { label: e.type, level: 'gray' as EventLevel }
              const cls = LEVEL_CLASS[meta.level]
              return (
                <div key={e.id} className={clsx('flex items-start gap-3 rounded-xl p-3', cls.bg)}>
                  <AlertTriangle size={15} className={clsx('mt-0.5 shrink-0', cls.icon)} />
                  <div className="min-w-0 flex-1">
                    <p className={clsx('text-sm', cls.text)}>
                      <span className="font-medium">{text(e.plate_no)}</span>
                      {e.driver_name && <span className="text-ink-muted">（{e.driver_name}）</span>} {meta.label}：{describeEvent(e)}
                    </p>
                  </div>
                  <Link to={`/trips?id=${encodeURIComponent(e.trip_id)}`} className="shrink-0 text-xs text-ink-faint hover:text-brand-300">
                    {formatDateTime(e.ts)}
                  </Link>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
