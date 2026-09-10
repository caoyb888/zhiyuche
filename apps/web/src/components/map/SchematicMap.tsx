import clsx from 'clsx'
import { Maximize2, Minus, Plus } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent, type PointerEvent as ReactPointerEvent } from 'react'
import { boundsCenter, boundsOf, isValidLngLat, padBounds, type Bounds } from '../../utils/geo'
import { DEFAULT_CENTER, DEFAULT_LINE_COLOR, MARKER_GLYPH, MARKER_RADIUS, markerColor } from './marker-style'
import type { LngLat, MapMarker, MapPolyline, MapViewProps } from './types'

interface Size {
  w: number
  h: number
}

/** 用户缩放 / 平移（相对于自适应后的基准投影） */
interface View {
  k: number
  tx: number
  ty: number
}

const IDENTITY: View = { k: 1, tx: 0, ty: 0 }
/** 轴标签留白 */
const PAD = 34
const MIN_K = 0.25
const MAX_K = 64
const GRID_STEPS = [0.0002, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 20]
const SCALE_METERS = [5, 10, 20, 50, 100, 200, 500, 1_000, 2_000, 5_000, 10_000, 20_000, 50_000, 100_000, 200_000, 500_000]
const M_PER_DEG_LAT = 111_320
/** 单点 / 无数据时的缺省视野跨度（度，约 1 km） */
const DEFAULT_SPAN_DEG = 0.01

interface Projection {
  project: (p: LngLat) => [number, number]
  unproject: (x: number, y: number) => LngLat
  pxPerDegLat: number
}

function makeProjection(b: Bounds, size: Size, view: View): Projection {
  const [cx, cy] = boundsCenter(b)
  const cosLat = Math.max(0.2, Math.cos((cy * Math.PI) / 180))
  const innerW = Math.max(1, size.w - PAD * 2)
  const innerH = Math.max(1, size.h - PAD * 2)
  const spanLng = Math.max(1e-6, (b.maxLng - b.minLng) * cosLat)
  const spanLat = Math.max(1e-6, b.maxLat - b.minLat)
  const s = Math.min(innerW / spanLng, innerH / spanLat) * view.k
  const ox = size.w / 2 + view.tx
  const oy = size.h / 2 + view.ty
  return {
    project: ([lng, lat]) => [(lng - cx) * cosLat * s + ox, -(lat - cy) * s + oy],
    unproject: (x, y) => [(x - ox) / (cosLat * s) + cx, -(y - oy) / s + cy],
    pxPerDegLat: s,
  }
}

function pickStep(rangeDeg: number, maxLines = 8): number {
  return GRID_STEPS.find((st) => rangeDeg / st <= maxLines) ?? GRID_STEPS[GRID_STEPS.length - 1]
}

function stepDigits(step: number): number {
  return Math.max(0, Math.ceil(-Math.log10(step)))
}

function formatMeters(m: number): string {
  return m >= 1000 ? `${m / 1000} km` : `${m} m`
}

/** 计算基准视野：全部标记 + 折线（fitBounds）或 center 附近 */
function computeBounds(markers: MapMarker[], polylines: MapPolyline[], center: LngLat | undefined, fitBounds: boolean, spanDeg: number): Bounds | null {
  const pts: LngLat[] = []
  if (fitBounds) {
    for (const m of markers) pts.push(m.lnglat)
    for (const l of polylines) for (const p of l.points) pts.push(p)
  }
  if (pts.length === 0 && center && isValidLngLat(center)) pts.push(center)
  const raw = boundsOf(pts)
  return raw ? padBounds(raw, 0.12, spanDeg) : null
}

interface CanvasProps {
  bounds: Bounds
  markers: MapMarker[]
  polylines: MapPolyline[]
  onClick?: (lnglat: LngLat) => void
  empty: boolean
}

function SchematicCanvas({ bounds, markers, polylines, onClick, empty }: CanvasProps) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState<Size>({ w: 0, h: 0 })
  const [view, setView] = useState<View>(IDENTITY)
  const [hovered, setHovered] = useState<string | null>(null)
  const drag = useRef<{ x: number; y: number; tx: number; ty: number; moved: boolean } | null>(null)

  // 容器尺寸
  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      const r = entries[0]?.contentRect
      if (r) setSize({ w: Math.round(r.width), h: Math.round(r.height) })
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  // 滚轮缩放（需要非 passive 监听才能阻止页面滚动）
  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      const rect = el.getBoundingClientRect()
      const x = e.clientX - rect.left
      const y = e.clientY - rect.top
      const f = e.deltaY < 0 ? 1.25 : 0.8
      setView((v) => {
        const k = Math.min(MAX_K, Math.max(MIN_K, v.k * f))
        const real = k / v.k
        const w = rect.width
        const h = rect.height
        return { k, tx: (1 - real) * (x - w / 2) + real * v.tx, ty: (1 - real) * (y - h / 2) + real * v.ty }
      })
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [])

  const zoomBy = useCallback((f: number) => {
    setView((v) => {
      const k = Math.min(MAX_K, Math.max(MIN_K, v.k * f))
      const real = k / v.k
      return { k, tx: real * v.tx, ty: real * v.ty }
    })
  }, [])

  const proj = useMemo(() => makeProjection(bounds, size, view), [bounds, size, view])

  const localPoint = (e: { clientX: number; clientY: number }): [number, number] => {
    const rect = wrapRef.current?.getBoundingClientRect()
    return rect ? [e.clientX - rect.left, e.clientY - rect.top] : [0, 0]
  }

  const onPointerDown = (e: ReactPointerEvent<SVGSVGElement>) => {
    if (e.button !== 0) return
    const [x, y] = localPoint(e)
    drag.current = { x, y, tx: view.tx, ty: view.ty, moved: false }
    e.currentTarget.setPointerCapture(e.pointerId)
  }
  const onPointerMove = (e: ReactPointerEvent<SVGSVGElement>) => {
    const d = drag.current
    if (!d) return
    const [x, y] = localPoint(e)
    const dx = x - d.x
    const dy = y - d.y
    if (!d.moved && Math.hypot(dx, dy) < 4) return
    d.moved = true
    setView((v) => ({ ...v, tx: d.tx + dx, ty: d.ty + dy }))
  }
  const onPointerUp = (e: ReactPointerEvent<SVGSVGElement>) => {
    if (drag.current && e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId)
    // click 事件紧随其后触发，由 onBackgroundClick 判断是否拖动过
    window.setTimeout(() => {
      drag.current = null
    }, 0)
  }
  const onBackgroundClick = (e: ReactMouseEvent<SVGSVGElement>) => {
    if (drag.current?.moved) return
    if (!onClick) return
    const [x, y] = localPoint(e)
    const p = proj.unproject(x, y)
    if (isValidLngLat(p)) onClick(p)
  }

  const ready = size.w > 0 && size.h > 0

  // ── 网格 / 刻度 ──
  const grid = useMemo(() => {
    if (!ready) return null
    const [lngL, latB] = proj.unproject(0, size.h)
    const [lngR, latT] = proj.unproject(size.w, 0)
    const stepLng = pickStep(Math.abs(lngR - lngL))
    const stepLat = pickStep(Math.abs(latT - latB))
    const vlines: Array<{ x: number; label: string }> = []
    const hlines: Array<{ y: number; label: string }> = []
    const dLng = stepDigits(stepLng)
    const dLat = stepDigits(stepLat)
    for (let lng = Math.ceil(lngL / stepLng) * stepLng; lng <= lngR; lng += stepLng) {
      const [x] = proj.project([lng, latB])
      vlines.push({ x, label: lng.toFixed(dLng) })
      if (vlines.length > 40) break
    }
    for (let lat = Math.ceil(latB / stepLat) * stepLat; lat <= latT; lat += stepLat) {
      const [, y] = proj.project([lngL, lat])
      hlines.push({ y, label: lat.toFixed(dLat) })
      if (hlines.length > 40) break
    }
    return { vlines, hlines }
  }, [proj, ready, size.h, size.w])

  // ── 比例尺 ──
  const scale = useMemo(() => {
    if (!ready) return null
    const mPerPx = M_PER_DEG_LAT / proj.pxPerDegLat
    let best = SCALE_METERS[0]
    for (const m of SCALE_METERS) if (m / mPerPx <= 140) best = m
    return { px: best / mPerPx, label: formatMeters(best) }
  }, [proj, ready])

  const sortedMarkers = useMemo(
    () => [...markers].sort((a, b) => (a.zIndex ?? 0) - (b.zIndex ?? 0) || Number(Boolean(a.selected)) - Number(Boolean(b.selected))),
    [markers],
  )

  return (
    <div
      ref={wrapRef}
      className="absolute inset-0 bg-surface-2 select-none touch-none"
      style={{ backgroundImage: 'radial-gradient(rgba(60,207,224,.05) 1px, transparent 1px)', backgroundSize: '22px 22px' }}
    >
      {ready && (
        <svg
          width={size.w}
          height={size.h}
          className={clsx('block', onClick ? 'cursor-crosshair' : 'cursor-grab')}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          onClick={onBackgroundClick}
          role="img"
          aria-label="示意图"
        >
          {/* 网格 */}
          {grid?.vlines.map((l) => (
            <g key={`v${l.label}`}>
              <line x1={l.x} x2={l.x} y1={0} y2={size.h} stroke="#12253c" strokeWidth={1} />
              <text x={l.x} y={size.h - 6} textAnchor="middle" fontSize={10} fill="#5f7896" className="font-mono">
                {l.label}
              </text>
            </g>
          ))}
          {grid?.hlines.map((l) => (
            <g key={`h${l.label}`}>
              <line x1={0} x2={size.w} y1={l.y} y2={l.y} stroke="#12253c" strokeWidth={1} />
              <text x={4} y={l.y - 3} fontSize={10} fill="#5f7896" className="font-mono">
                {l.label}
              </text>
            </g>
          ))}

          {/* 折线 */}
          {polylines.map((line) => {
            const pts = line.points.filter(isValidLngLat)
            if (pts.length < 2) return null
            return (
              <polyline
                key={line.id}
                points={pts.map((p) => proj.project(p).join(',')).join(' ')}
                fill="none"
                stroke={line.color ?? DEFAULT_LINE_COLOR}
                strokeWidth={line.width ?? 3}
                strokeOpacity={line.opacity ?? 0.9}
                strokeDasharray={line.dashed ? '7 5' : undefined}
                strokeLinejoin="round"
                strokeLinecap="round"
              />
            )
          })}

          {/* 标记 */}
          {sortedMarkers.map((m) => {
            if (!isValidLngLat(m.lnglat)) return null
            const [x, y] = proj.project(m.lnglat)
            const icon = m.icon ?? 'dot'
            const r = MARKER_RADIUS[icon]
            const color = markerColor(m)
            const glyph = MARKER_GLYPH[icon]
            const heading = icon === 'vehicle' && typeof m.heading === 'number' && Number.isFinite(m.heading) ? m.heading : null
            const isHover = hovered === m.id
            return (
              <g
                key={m.id}
                transform={`translate(${x} ${y})`}
                opacity={m.opacity ?? 1}
                style={{ cursor: m.onClick ? 'pointer' : 'default' }}
                onClick={(e) => {
                  e.stopPropagation()
                  m.onClick?.(m)
                }}
                onPointerEnter={() => setHovered(m.id)}
                onPointerLeave={() => setHovered((h) => (h === m.id ? null : h))}
              >
                {m.title && <title>{m.title}</title>}
                {m.selected && <circle r={r + 5} fill="none" stroke={color} strokeOpacity={0.55} strokeWidth={3} />}
                {isHover && <circle r={r + 4} fill={color} fillOpacity={0.18} />}
                {heading !== null && <path d={`M0 ${-(r + 9)} L-5 ${-(r + 1)} L5 ${-(r + 1)} Z`} fill={color} transform={`rotate(${heading})`} />}
                {icon === 'pin' ? (
                  <>
                    <path d="M0 0 C-7 -8 -9 -12 -9 -16 A9 9 0 1 1 9 -16 C9 -12 7 -8 0 0 Z" fill={color} stroke="#fff" strokeWidth={2} />
                    <circle cy={-16} r={3} fill="#fff" />
                  </>
                ) : (
                  <circle r={r} fill={color} stroke="#fff" strokeWidth={2} />
                )}
                {glyph && (
                  <text textAnchor="middle" dominantBaseline="central" fontSize={r + 1} fontWeight={700} fill="#fff" style={{ pointerEvents: 'none' }}>
                    {glyph}
                  </text>
                )}
                {m.label && (
                  <text
                    y={icon === 'pin' ? 12 : r + 13}
                    textAnchor="middle"
                    fontSize={11}
                    fontWeight={600}
                    fill="#e8f0f9"
                    stroke="#071223"
                    strokeWidth={3}
                    paintOrder="stroke"
                    style={{ pointerEvents: 'none' }}
                  >
                    {m.label}
                  </text>
                )}
              </g>
            )
          })}

          {/* 比例尺 */}
          {scale && (
            <g transform={`translate(${size.w - 16 - scale.px} ${size.h - 22})`}>
              <rect x={-6} y={-16} width={scale.px + 12} height={24} rx={4} fill="#fff" fillOpacity={0.85} />
              <line x1={0} x2={scale.px} y1={2} y2={2} stroke="#7089a8" strokeWidth={2} />
              <line x1={0} x2={0} y1={-3} y2={6} stroke="#7089a8" strokeWidth={2} />
              <line x1={scale.px} x2={scale.px} y1={-3} y2={6} stroke="#7089a8" strokeWidth={2} />
              <text x={scale.px / 2} y={-4} textAnchor="middle" fontSize={10} fill="#7089a8" className="font-mono">
                {scale.label}
              </text>
            </g>
          )}

          {empty && (
            <text x={size.w / 2} y={size.h / 2} textAnchor="middle" dominantBaseline="central" fontSize={13} fill="#5f7896">
              暂无位置数据
            </text>
          )}
        </svg>
      )}

      {/* 缩放控件 */}
      <div className="absolute right-2 top-2 flex flex-col overflow-hidden rounded-lg border border-line bg-surface-2/90 backdrop-blur">
        <button type="button" aria-label="放大" title="放大" onClick={() => zoomBy(1.5)} className="flex h-7 w-7 items-center justify-center text-ink-muted hover:bg-surface-3">
          <Plus size={14} />
        </button>
        <button type="button" aria-label="缩小" title="缩小" onClick={() => zoomBy(1 / 1.5)} className="flex h-7 w-7 items-center justify-center border-t border-line-soft text-ink-muted hover:bg-surface-3">
          <Minus size={14} />
        </button>
        <button type="button" aria-label="重置视野" title="重置视野" onClick={() => setView(IDENTITY)} className="flex h-7 w-7 items-center justify-center border-t border-line-soft text-ink-muted hover:bg-surface-3">
          <Maximize2 size={13} />
        </button>
      </div>
    </div>
  )
}

export interface SchematicMapProps extends MapViewProps {
  /** 单点 / 无数据时的最小视野跨度（度），缺省 0.01 ≈ 1 km */
  spanDeg?: number
  /** 不显示"示意图"角标（引擎已在外层说明时） */
  hideBadge?: boolean
}

/**
 * 自绘 SVG 示意图（无高德 Key 的降级实现）：
 * 把经纬度按包围盒等比缩放到容器，画浅色网格、经纬度刻度、比例尺，支持标记 / 折线 / 点击 / hover / 滚轮缩放 / 拖拽平移。
 * 视野在 fitKey 变化或首次出现数据时重新适配（内部以 key 重建画布，重置缩放）。
 */
export default function SchematicMap({ markers = [], polylines = [], center, fitBounds = true, fitKey, onClick, spanDeg = DEFAULT_SPAN_DEG, hideBadge, hint }: SchematicMapProps) {
  const hasData = markers.length > 0 || polylines.some((l) => l.points.length > 0)
  const bounds = useMemo(() => computeBounds(markers, polylines, center, fitBounds, spanDeg), [markers, polylines, center, fitBounds, spanDeg])
  // 首次出现数据、fitKey 变化时重建画布，使基准视野重新适配；实时更新（同一 key）不会拉回视野
  const mountKey = `${String(fitKey ?? '')}:${hasData ? 1 : 0}:${center ? center.join(',') : ''}`

  return (
    <>
      <FrozenCanvas key={mountKey} initialBounds={bounds} markers={markers} polylines={polylines} onClick={onClick} empty={!hasData} />
      {!hideBadge && (
        <div className="pointer-events-none absolute left-2 top-2 flex flex-col items-start gap-1">
          <span className="rounded-md border border-line bg-surface-1/85 px-2 py-0.5 text-[11px] font-medium text-ink backdrop-blur">示意图（未配置地图 Key）</span>
          {hint && <span className="rounded-md border border-line bg-surface-2/85 px-2 py-0.5 text-[11px] text-ink-muted backdrop-blur">{hint}</span>}
        </div>
      )}
    </>
  )
}

/** 基准视野在挂载时冻结，之后标记移动不会改变投影 */
function FrozenCanvas({ initialBounds, ...rest }: Omit<CanvasProps, 'bounds'> & { initialBounds: Bounds | null }) {
  const [bounds] = useState<Bounds>(() => initialBounds ?? padBounds({ minLng: DEFAULT_CENTER[0], maxLng: DEFAULT_CENTER[0], minLat: DEFAULT_CENTER[1], maxLat: DEFAULT_CENTER[1] }, 0.12, 0.05))
  return <SchematicCanvas bounds={bounds} {...rest} />
}
