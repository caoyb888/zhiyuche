import clsx from 'clsx'
import { useMemo } from 'react'
import type { PlannedRoute } from '../../api/types'
import { formatDistance, haversineMeters, isValidLngLat, type LngLat } from '../../utils/geo'
import MapView from './MapView'
import { routePoints } from './route-utils'
import type { MapMarker, MapPolyline } from './types'

export interface RoutePreviewProps {
  origin?: LngLat | null
  originLabel?: string
  destination?: LngLat | null
  destinationLabel?: string
  /** 规划路线；无 points 时在起终点之间画直线（虚线） */
  route?: PlannedRoute | null
  /** 附加标记（途经点等） */
  extraMarkers?: MapMarker[]
  height?: number | string
  className?: string
  /** 不显示底部距离 / 时长摘要 */
  hideSummary?: boolean
}

const ROUTE_COLOR = '#3ccfe0'
const STRAIGHT_COLOR = '#7089a8'

function formatMinutes(min: number | null | undefined): string {
  if (min === null || min === undefined || !Number.isFinite(min)) return '—'
  const m = Math.round(min)
  return m >= 60 ? `${Math.floor(m / 60)} 小时 ${m % 60} 分` : `${m} 分钟`
}

/** 起点、目的地与计划路线的静态展示 */
export default function RoutePreview({ origin, originLabel = '起点', destination, destinationLabel = '目的地', route, extraMarkers, height = 260, className, hideSummary }: RoutePreviewProps) {
  const o = origin && isValidLngLat(origin) ? origin : null
  const d = destination && isValidLngLat(destination) ? destination : null
  const pts = useMemo(() => routePoints(route), [route])

  const polylines = useMemo<MapPolyline[]>(() => {
    if (pts.length >= 2) return [{ id: 'route', points: pts, color: ROUTE_COLOR, width: 4 }]
    if (o && d) return [{ id: 'straight', points: [o, d], color: STRAIGHT_COLOR, width: 3, dashed: true }]
    return []
  }, [pts, o, d])

  const markers = useMemo<MapMarker[]>(() => {
    const out: MapMarker[] = []
    const start = o ?? pts[0]
    const end = d ?? (pts.length > 1 ? pts[pts.length - 1] : undefined)
    if (start) out.push({ id: 'origin', lnglat: start, icon: 'start', label: originLabel, zIndex: 3 })
    if (end) out.push({ id: 'destination', lnglat: end, icon: 'end', label: destinationLabel, zIndex: 3 })
    if (extraMarkers) out.push(...extraMarkers)
    return out
  }, [o, d, pts, originLabel, destinationLabel, extraMarkers])

  const straight = o && d ? haversineMeters(o, d) : null
  const distanceKm = route?.distance_km ?? null
  const hasAny = markers.length > 0 || polylines.length > 0

  return (
    <div className={clsx('space-y-2', className)}>
      <MapView height={height} markers={markers} polylines={polylines} fitKey={`${pts.length}:${o?.join(',') ?? ''}:${d?.join(',') ?? ''}`} hint={!hasAny ? '暂无路线信息' : undefined} />
      {!hideSummary && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ink-muted">
          <span>
            距离 <span className="font-medium text-ink-strong">{distanceKm !== null ? `${distanceKm.toFixed(1)} km` : straight !== null ? `${formatDistance(straight)}（直线）` : '—'}</span>
          </span>
          <span>
            预计时长 <span className="font-medium text-ink-strong">{formatMinutes(route?.duration_min)}</span>
          </span>
          {route?.summary && <span className="truncate">路线：{route.summary}</span>}
        </div>
      )}
    </div>
  )
}
