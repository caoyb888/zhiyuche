import clsx from 'clsx'
import dayjs from 'dayjs'
import { Pause, Play, SkipBack } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import type { PlannedRoute } from '../../api/types'
import { bearing, formatDistance, isValidLngLat, pathLengthMeters, simplifyIndices, type LngLat } from '../../utils/geo'
import MapView from './MapView'
import type { MapMarker, MapPolyline } from './types'
import { formatPercent, formatSpeed } from './vehicleStyle'

export interface TrackPlayerPoint {
  ts: string
  lng: number
  lat: number
  speed?: number | null
  soc?: number | null
  heading?: number | null
}

export type PlaybackRate = 1 | 4 | 16

export interface TrackPlayerProps {
  points: TrackPlayerPoint[]
  /** 计划路线（灰色虚线） */
  plannedRoute?: PlannedRoute | null
  height?: number | string
  className?: string
  startLabel?: string
  endLabel?: string
  autoPlay?: boolean
}

const RATES: PlaybackRate[] = [1, 4, 16]
// 深色底图：已走轨迹用电光青（同时是「实时」语义色），未走与计划线退到暗蓝灰
const TRACK_COLOR = '#3ccfe0'
const TRACK_REMAINING_COLOR = '#1f4b78'
const PLANNED_COLOR = '#7089a8'
/** 抽稀容差（度），约 3 m */
const SIMPLIFY_TOLERANCE = 0.00003

interface Prepared {
  points: TrackPlayerPoint[]
  coords: LngLat[]
  times: number[]
  /** 抽稀后用于绘制的下标 */
  drawn: number[]
}

function prepare(raw: TrackPlayerPoint[]): Prepared {
  const points = raw
    .filter((p) => isValidLngLat([p.lng, p.lat]) && Number.isFinite(Date.parse(p.ts)))
    .sort((a, b) => Date.parse(a.ts) - Date.parse(b.ts))
  const coords = points.map((p): LngLat => [p.lng, p.lat])
  const times = points.map((p) => Date.parse(p.ts))
  return { points, coords, times, drawn: simplifyIndices(coords, SIMPLIFY_TOLERANCE) }
}

/** 最后一个 times[i] <= t 的 i（二分） */
function indexAt(times: number[], t: number): number {
  let lo = 0
  let hi = times.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (times[mid] <= t) lo = mid
    else hi = mid - 1
  }
  return lo
}

function formatDuration(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  return h > 0 ? `${h}h ${m}m` : m > 0 ? `${m}m ${sec}s` : `${sec}s`
}

/**
 * 轨迹回放：轨迹折线（蓝，已走过部分深色）+ 计划路线（灰虚线）+ 起终点标记 + 移动标记，
 * 播放 / 暂停 / 倍速（1/4/16，1x 为真实时间）/ 时间轴滑块，显示当前速度、SOC、时间。
 */
export default function TrackPlayer({ points, plannedRoute, height = 380, className, startLabel = '起点', endLabel = '终点', autoPlay = false }: TrackPlayerProps) {
  const data = useMemo(() => prepare(points), [points])
  const n = data.points.length
  const [idx, setIdx] = useState(0)
  const [playing, setPlaying] = useState(autoPlay && n > 1)
  const [rate, setRate] = useState<PlaybackRate>(4)
  const virtualRef = useRef<number | null>(null)
  const cur = Math.min(idx, Math.max(0, n - 1))

  // 播放循环：虚拟时间按倍速推进，按 ts 找当前点
  useEffect(() => {
    if (!playing || n < 2) return
    let raf = 0
    let last = performance.now()
    if (virtualRef.current === null) virtualRef.current = data.times[0]
    const tick = (now: number) => {
      const v = (virtualRef.current ?? data.times[0]) + (now - last) * rate
      last = now
      virtualRef.current = v
      const end = data.times[n - 1]
      if (v >= end) {
        virtualRef.current = end
        setIdx(n - 1)
        setPlaying(false)
        return
      }
      setIdx((prev) => {
        const next = indexAt(data.times, v)
        return next === prev ? prev : next
      })
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [playing, rate, data, n])

  const seek = (i: number) => {
    const clamped = Math.max(0, Math.min(n - 1, i))
    setIdx(clamped)
    virtualRef.current = data.times[clamped] ?? null
  }

  const togglePlay = () => {
    if (n < 2) return
    if (!playing && cur >= n - 1) seek(0)
    setPlaying((p) => !p)
  }

  const polylines = useMemo<MapPolyline[]>(() => {
    const lines: MapPolyline[] = []
    const planned = (plannedRoute?.points ?? []).filter((p): p is LngLat => Array.isArray(p) && p.length >= 2 && isValidLngLat([p[0], p[1]]))
    if (planned.length >= 2) lines.push({ id: 'planned', points: planned, color: PLANNED_COLOR, dashed: true, width: 3, opacity: 0.9 })
    if (n >= 2) {
      const drawnIdx = data.drawn
      const passed: LngLat[] = []
      const remaining: LngLat[] = []
      for (const i of drawnIdx) {
        if (i <= cur) passed.push(data.coords[i])
        else remaining.push(data.coords[i])
      }
      passed.push(data.coords[cur])
      remaining.unshift(data.coords[cur])
      if (remaining.length >= 2) lines.push({ id: 'remaining', points: remaining, color: TRACK_REMAINING_COLOR, width: 4 })
      if (passed.length >= 2) lines.push({ id: 'passed', points: passed, color: TRACK_COLOR, width: 4 })
    }
    return lines
  }, [plannedRoute, data, n, cur])

  const markers = useMemo<MapMarker[]>(() => {
    if (n === 0) return []
    const p = data.points[cur]
    const heading = typeof p.heading === 'number' && Number.isFinite(p.heading) ? p.heading : cur > 0 ? bearing(data.coords[cur - 1], data.coords[cur]) : null
    const out: MapMarker[] = [{ id: 'start', lnglat: data.coords[0], icon: 'start', label: startLabel, zIndex: 3 }]
    if (n > 1) out.push({ id: 'end', lnglat: data.coords[n - 1], icon: 'end', label: endLabel, zIndex: 3 })
    out.push({ id: 'current', lnglat: data.coords[cur], icon: 'vehicle', color: TRACK_COLOR, heading, zIndex: 10, title: dayjs(p.ts).format('HH:mm:ss') })
    return out
  }, [data, n, cur, startLabel, endLabel])

  const current = n > 0 ? data.points[cur] : null
  const totalMs = n > 1 ? data.times[n - 1] - data.times[0] : 0
  const elapsedMs = n > 1 ? data.times[cur] - data.times[0] : 0
  const distance = useMemo(() => pathLengthMeters(data.coords), [data])

  return (
    <div className={clsx('space-y-3', className)}>
      <MapView
        height={height}
        polylines={polylines}
        markers={markers}
        fitKey={`${n}:${plannedRoute?.points?.length ?? 0}`}
        hint={n === 0 ? '暂无轨迹点' : undefined}
        overlay={
          current && (
            <div className="absolute bottom-2 left-2 flex items-center gap-3 rounded-lg bg-surface-2/90 px-3 py-1.5 text-xs shadow-sm">
              <span className="font-mono text-ink">{dayjs(current.ts).format('MM-DD HH:mm:ss')}</span>
              <span className="text-ink-faint">|</span>
              <span className="text-ink">
                速度 <span className="font-medium text-ink-strong">{formatSpeed(current.speed)}</span>
              </span>
              <span className="text-ink">
                SOC <span className="font-medium text-ink-strong">{formatPercent(current.soc)}</span>
              </span>
            </div>
          )
        }
      />

      <div className="flex flex-col gap-2 rounded-xl border border-line bg-surface-2 p-3 sm:flex-row sm:items-center">
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            onClick={() => seek(0)}
            disabled={n < 2}
            aria-label="回到起点"
            title="回到起点"
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-line-strong text-ink hover:bg-surface-3 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <SkipBack size={14} />
          </button>
          <button
            type="button"
            onClick={togglePlay}
            disabled={n < 2}
            aria-label={playing ? '暂停' : '播放'}
            title={playing ? '暂停' : '播放'}
            className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-600 text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {playing ? <Pause size={14} /> : <Play size={14} />}
          </button>
          <div className="ml-1 flex overflow-hidden rounded-lg border border-line-strong text-xs">
            {RATES.map((r) => (
              <button
                key={r}
                type="button"
                onClick={() => setRate(r)}
                aria-pressed={rate === r}
                className={clsx('px-2 py-1.5 transition-colors', rate === r ? 'bg-brand-600 text-white' : 'text-ink hover:bg-surface-3')}
              >
                {r}x
              </button>
            ))}
          </div>
        </div>
        <div className="flex flex-1 items-center gap-3">
          <input
            type="range"
            min={0}
            max={Math.max(0, n - 1)}
            value={cur}
            disabled={n < 2}
            onChange={(e) => seek(Number(e.target.value))}
            aria-label="时间轴"
            className="w-full accent-brand-600"
          />
          <span className="shrink-0 font-mono text-[11px] text-ink-muted">
            {formatDuration(elapsedMs)} / {formatDuration(totalMs)}
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-3 text-[11px] text-ink-muted">
          <span>
            {n} 点 · {formatDistance(distance)}
          </span>
        </div>
      </div>
    </div>
  )
}
