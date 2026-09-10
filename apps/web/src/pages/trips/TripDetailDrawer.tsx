import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { Ban, ChevronRight, Square, type LucideIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { errorMessage } from '../../api/client'
import { getTrip, getTripTrack, tripKeys } from '../../api/trips'
import type { Trip, TripEvent } from '../../api/types'
import { TrackPlayer, type TrackPlayerPoint } from '../../components/map'
import { VEHICLE_STATUS_BADGE, VEHICLE_STATUS_LABEL, formatKm, formatPercent, formatSpeed } from '../../components/map/vehicleStyle'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import DescriptionList from '../../components/ui/DescriptionList'
import Drawer from '../../components/ui/Drawer'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import Spinner from '../../components/ui/Spinner'
import { usePermission } from '../../hooks/usePermission'
import { formatDateTime, formatMinutes, formatNumber } from '../../utils/format'
import { TRIP_TYPE_BADGE, TRIP_TYPE_LABEL } from '../approval/style'
import { tripBilling } from './costDetail'
import CostDetailView from './CostDetailView'
import { EVENT_LEVEL_CLASS, ROOF_SIGN_BADGE, ROOF_SIGN_LABEL, TRACK_MAX_POINTS, TRIP_SOURCE_LABEL, TRIP_STATUS_BADGE, TRIP_STATUS_LABEL, describeEvent, eventMeta, trackStep } from './style'
import { CancelTripModal, EndTripModal } from './TripActionModals'

interface TripDetailDrawerProps {
  id: string | null
  onClose: () => void
}

/** 进行中的行程：轮询间隔（WebSocket trip.event 到达时也会失效重取） */
const LIVE_INTERVAL_MS = 10_000

interface StatCell {
  label: string
  value: string
  sub?: string
  color?: string
}

function StatGrid({ t }: { t: Trip }) {
  const cells: StatCell[] = [
    { label: '里程', value: formatKm(t.distance_km, 1), color: 'text-brand-300' },
    { label: '时长', value: formatMinutes(t.duration_min) },
    { label: '平均速度', value: formatSpeed(t.avg_speed) },
    { label: '最高速度', value: formatSpeed(t.max_speed), color: typeof t.max_speed === 'number' && t.max_speed > 100 ? 'text-danger-200' : undefined },
    { label: '耗电', value: formatNumber(t.energy_kwh, 1, 'kWh'), sub: typeof t.energy_per_100km === 'number' ? `百公里 ${t.energy_per_100km.toFixed(1)} kWh` : undefined, color: 'text-warn-200' },
    { label: 'SOC 起止', value: `${formatPercent(t.start_soc)} → ${formatPercent(t.end_soc)}` },
    { label: '急加速 / 急刹', value: `${t.harsh_accel} / ${t.harsh_brake} 次`, color: t.harsh_accel + t.harsh_brake > 8 ? 'text-danger-200' : undefined },
    { label: '灯牌', value: ROOF_SIGN_LABEL[t.roof_sign_status] },
    { label: '偏离最大距离', value: t.deviation_flag ? (typeof t.deviation_max_m === 'number' ? (t.deviation_max_m >= 1000 ? `${(t.deviation_max_m / 1000).toFixed(1)} km` : `${Math.round(t.deviation_max_m)} m`) : '有偏离') : '无偏离', color: t.deviation_flag ? 'text-danger-200' : 'text-ev-200' },
  ]
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {cells.map((c) => (
        <div key={c.label} className="rounded-xl bg-surface-3 p-3">
          <div className="text-xs text-ink-faint">{c.label}</div>
          <div className={clsx('mt-0.5 text-base font-semibold', c.color ?? 'text-ink-strong')}>{c.value}</div>
          {c.sub && <div className="text-[11px] text-ink-faint">{c.sub}</div>}
        </div>
      ))}
    </div>
  )
}

function EventTimeline({ events }: { events: TripEvent[] }) {
  const sorted = useMemo(() => [...events].sort((a, b) => Date.parse(a.ts) - Date.parse(b.ts)), [events])
  if (sorted.length === 0) return <Empty size="sm" title="暂无事件" description="行程开始 / 结束、超速、偏离、低电量等事件会显示在这里" />
  return (
    <ol className="space-y-0">
      {sorted.map((e, i) => {
        const meta = eventMeta(e.type)
        const cls = EVENT_LEVEL_CLASS[meta.level]
        const Icon: LucideIcon = meta.icon
        const last = i === sorted.length - 1
        return (
          <li key={e.id} className="relative flex gap-3 pb-3">
            {!last && <span className="absolute left-3 top-6 h-[calc(100%-0.5rem)] w-px bg-line-strong" aria-hidden />}
            <span className={clsx('relative z-[1] flex h-6 w-6 shrink-0 items-center justify-center rounded-full', cls.dot)}>
              <Icon size={13} />
            </span>
            <div className={clsx('min-w-0 flex-1 rounded-lg px-3 py-2', cls.bg)}>
              <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-0.5">
                <span className={clsx('text-sm font-medium', cls.text)}>{meta.label}</span>
                <span className="font-mono text-[11px] text-ink-faint">{formatDateTime(e.ts)}</span>
              </div>
              <div className="text-xs text-ink">{describeEvent(e)}</div>
            </div>
          </li>
        )
      })}
    </ol>
  )
}

function DetailBody({ id, onClose }: { id: string; onClose: () => void }) {
  const { can } = usePermission()
  const [action, setAction] = useState<'end' | 'cancel' | null>(null)

  const detail = useQuery({
    queryKey: tripKeys.detail(id),
    queryFn: () => getTrip(id),
    refetchInterval: (q) => (q.state.data?.status === 'ongoing' ? LIVE_INTERVAL_MS : false),
  })
  const trip = detail.data
  const ongoing = trip?.status === 'ongoing'
  // 点数 > 2000 时按 step 抽稀；进行中的行程点数在增长，按当前点数计算即可
  const step = trackStep(trip?.point_count)
  const track = useQuery({
    queryKey: tripKeys.track(id, step),
    queryFn: () => getTripTrack(id, step),
    enabled: Boolean(trip),
    refetchInterval: ongoing ? LIVE_INTERVAL_MS : false,
    staleTime: ongoing ? 0 : 5 * 60_000,
  })
  const points = useMemo<TrackPlayerPoint[]>(() => (track.data?.points ?? []).map((p) => ({ ts: p.ts, lng: p.lng, lat: p.lat, speed: p.speed, soc: p.soc, heading: p.heading })), [track.data])

  if (detail.isPending) {
    return (
      <div className="flex justify-center py-10">
        <Spinner label="加载中" />
      </div>
    )
  }
  if (detail.isError || !trip) return <ErrorState message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />

  const t = trip
  const canManage = can('trip:manage')
  const hasActions = canManage && ongoing
  const events = t.events ?? []

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-base font-semibold text-ink-strong">{t.trip_no}</span>
        <Badge color={TRIP_STATUS_BADGE[t.status]}>
          {ongoing && <span className="inline-block h-1.5 w-1.5 rounded-full bg-tech-400 pulse-dot" />}
          {TRIP_STATUS_LABEL[t.status]}
        </Badge>
        <Badge color={TRIP_TYPE_BADGE[t.trip_type]}>{TRIP_TYPE_LABEL[t.trip_type]}</Badge>
        <Badge color={ROOF_SIGN_BADGE[t.roof_sign_status]}>{ROOF_SIGN_LABEL[t.roof_sign_status]}</Badge>
        {t.deviation_flag && <Badge color="red">偏离路线</Badge>}
        <span className="text-xs text-ink-faint">{TRIP_SOURCE_LABEL[t.source]}</span>
      </div>

      <section>
        <div className="mb-2 flex items-center justify-between">
          <h4 className="text-sm font-semibold text-ink">轨迹回放</h4>
          <span className="text-xs text-ink-faint">
            {track.data ? (
              <>
                {t.point_count} 点{step > 1 ? `，每 ${step} 点取 1（上限 ${TRACK_MAX_POINTS}）` : ''}
                {ongoing && ' · 每 10 秒刷新'}
              </>
            ) : null}
          </span>
        </div>
        {track.isPending ? (
          <div className="flex h-[380px] items-center justify-center rounded-xl bg-surface-3">
            <Spinner label="加载轨迹…" />
          </div>
        ) : track.isError ? (
          <ErrorState message={errorMessage(track.error)} onRetry={() => void track.refetch()} />
        ) : (
          <TrackPlayer points={points} plannedRoute={track.data?.planned_route ?? null} height={380} startLabel="出发" endLabel={ongoing ? '当前' : '到达'} />
        )}
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">行程数据</h4>
        <StatGrid t={t} />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">车辆 / 驾驶员 / 申请</h4>
        <DescriptionList
          columns={2}
          items={[
            {
              label: '车辆',
              value: (
                <span className="flex flex-wrap items-center gap-1.5">
                  <Link to={`/assets/vehicles?id=${encodeURIComponent(t.vehicle.id)}`} className="font-medium text-brand-300 hover:underline">
                    {t.vehicle.plate_no}
                  </Link>
                  {t.vehicle.model && <span className="text-xs text-ink-faint">{t.vehicle.model}</span>}
                  <Badge color={VEHICLE_STATUS_BADGE[t.vehicle.status]}>{VEHICLE_STATUS_LABEL[t.vehicle.status]}</Badge>
                </span>
              ),
            },
            {
              label: '驾驶员',
              value: t.driver ? (
                <span>
                  {t.driver.name}
                  {t.driver.dept_name && <span className="ml-1 text-xs text-ink-faint">{t.driver.dept_name}</span>}
                  {t.card_uid && <span className="ml-1 font-mono text-xs text-ink-faint">卡 {t.card_uid}</span>}
                </span>
              ) : (
                <span className="text-ink-faint">未识别</span>
              ),
            },
            { label: '出发时间', value: formatDateTime(t.start_at) },
            { label: '结束时间', value: t.end_at ? formatDateTime(t.end_at) : <span className="text-tech-200">进行中</span> },
            { label: '里程表', value: `${formatKm(t.start_odometer, 1)} → ${formatKm(t.end_odometer, 1)}` },
            { label: '事由', value: t.purpose ?? t.approval?.purpose_detail ?? null },
            {
              label: '关联申请',
              value: t.approval?.id ? (
                <Link to={`/approval?id=${encodeURIComponent(t.approval.id)}`} className="inline-flex items-center gap-1 text-brand-300 hover:underline">
                  <span className="font-mono">{t.approval.apply_no}</span>
                  {t.approval.destination && <span className="text-xs text-ink-muted">· {t.approval.destination}</span>}
                  {typeof t.approval.planned_km === 'number' && <span className="text-xs text-ink-faint">· 预计 {t.approval.planned_km.toFixed(1)} km</span>}
                  <ChevronRight size={13} />
                </Link>
              ) : (
                <span className="text-ink-faint">无（{TRIP_TYPE_LABEL[t.trip_type]}）</span>
              ),
            },
            { label: '备注', value: t.remark, span: 2 },
          ]}
        />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">事件（{events.length}）</h4>
        <EventTimeline events={events} />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">费用</h4>
        <CostDetailView cost={t.cost} detail={t.cost_detail} billing={tripBilling(t)} />
      </section>

      {hasActions && (
        <div className="sticky bottom-0 -mx-5 -mb-4 flex flex-wrap items-center justify-end gap-2 border-t border-line bg-surface-2 px-5 py-3">
          <Button variant="secondary" icon={Ban} className="text-danger-200" onClick={() => setAction('cancel')}>
            作废
          </Button>
          <Button icon={Square} onClick={() => setAction('end')}>
            结束行程
          </Button>
        </div>
      )}

      <EndTripModal trip={action === 'end' ? t : null} onClose={() => setAction(null)} />
      <CancelTripModal trip={action === 'cancel' ? t : null} onClose={() => setAction(null)} onSuccess={() => onClose()} />
    </div>
  )
}

/** 行程详情抽屉：轨迹回放 + 行程数据 + 车辆 / 驾驶员 / 申请 + 事件时间轴 + 费用 + 操作 */
export default function TripDetailDrawer({ id, onClose }: TripDetailDrawerProps) {
  return (
    <Drawer open={id !== null} onClose={onClose} title="行程详情" width="lg">
      {id && <DetailBody key={id} id={id} onClose={onClose} />}
    </Drawer>
  )
}
