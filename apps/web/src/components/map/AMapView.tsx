import { useEffect, useRef, useState } from 'react'
import { errorMessage } from '../../api/client'
import { isValidLngLat } from '../../utils/geo'
import Spinner from '../ui/Spinner'
import type { AMapMap, AMapMarker, AMapNS, AMapOverlay, AMapPolyline } from './amap-types'
import { mapEngineStore } from './engine'
import { DEFAULT_CENTER, DEFAULT_LINE_COLOR, markerHtml } from './marker-style'
import type { LngLat, MapMarker, MapViewProps } from './types'

interface MarkerEntry {
  overlay: AMapMarker
  html: string
  pos: string
  title: string
}

interface LineEntry {
  overlay: AMapPolyline
  sig: string
}

interface MapContext {
  ns: AMapNS
  map: AMapMap
  markers: Map<string, MarkerEntry>
  lines: Map<string, LineEntry>
}

function lineSignature(points: LngLat[], color: string, width: number, dashed: boolean, opacity: number): string {
  const last = points[points.length - 1]
  const first = points[0]
  return `${points.length}|${first?.join(',') ?? ''}|${last?.join(',') ?? ''}|${color}|${width}|${dashed ? 1 : 0}|${opacity}`
}

/**
 * 高德 JS API 2.0 引擎：与 SchematicMap 渲染同一套 MapView props。
 * 标记 / 折线按 id 做增量更新（实时位置更新不重建 Marker），点击回调通过 extData 读取最新数据避免闭包过期。
 */
export default function AMapView({ markers = [], polylines = [], center, zoom, fitBounds = true, fitKey, onClick }: MapViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const ctxRef = useRef<MapContext | null>(null)
  const onClickRef = useRef(onClick)
  const lastFitRef = useRef<string | null>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // 初始视野只在创建地图时使用，之后由 fit / 用户操作决定
  const [initial] = useState(() => ({ center: center && isValidLngLat(center) ? center : DEFAULT_CENTER, zoom: zoom ?? 12 }))

  useEffect(() => {
    onClickRef.current = onClick
  }, [onClick])

  // 创建 / 销毁地图
  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    let cancelled = false
    mapEngineStore
      .load()
      .then((ns) => {
        if (cancelled) return
        // 深空控制台：底图走高德官方深色样式，轨迹/标记才压得住
        const map = new ns.Map(el, { zoom: initial.zoom, center: initial.center, viewMode: '2D', resizeEnable: true, mapStyle: 'amap://styles/darkblue' })
        map.addControl(new ns.Scale())
        map.on('click', (e) => onClickRef.current?.([e.lnglat.getLng(), e.lnglat.getLat()]))
        ctxRef.current = { ns, map, markers: new Map(), lines: new Map() }
        setReady(true)
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e, '高德地图加载失败'))
      })
    return () => {
      cancelled = true
      ctxRef.current?.map.destroy()
      ctxRef.current = null
    }
  }, [initial])

  // 同步标记
  useEffect(() => {
    const ctx = ctxRef.current
    if (!ctx || !ready) return
    const seen = new Set<string>()
    for (const m of markers) {
      if (!isValidLngLat(m.lnglat)) continue
      seen.add(m.id)
      const html = markerHtml(m)
      const pos = m.lnglat.join(',')
      const title = m.title ?? ''
      const ex = ctx.markers.get(m.id)
      if (ex) {
        if (ex.pos !== pos) ex.overlay.setPosition(m.lnglat)
        if (ex.html !== html) ex.overlay.setContent(html)
        if (ex.title !== title) ex.overlay.setTitle(title)
        ex.overlay.setExtData(m)
        ex.pos = pos
        ex.html = html
        ex.title = title
      } else {
        const overlay = new ctx.ns.Marker({
          position: m.lnglat,
          content: html,
          anchor: 'center',
          offset: new ctx.ns.Pixel(0, 0),
          title,
          zIndex: 100 + (m.zIndex ?? 0),
          extData: m,
        })
        overlay.on('click', () => {
          const data = overlay.getExtData() as MapMarker | undefined
          data?.onClick?.(data)
        })
        ctx.map.add(overlay)
        ctx.markers.set(m.id, { overlay, html, pos, title })
      }
    }
    for (const [id, ex] of ctx.markers) {
      if (!seen.has(id)) {
        ctx.map.remove(ex.overlay)
        ctx.markers.delete(id)
      }
    }
  }, [markers, ready])

  // 同步折线
  useEffect(() => {
    const ctx = ctxRef.current
    if (!ctx || !ready) return
    const seen = new Set<string>()
    for (const line of polylines) {
      const pts = line.points.filter(isValidLngLat)
      if (pts.length < 2) continue
      seen.add(line.id)
      const color = line.color ?? DEFAULT_LINE_COLOR
      const width = line.width ?? 4
      const dashed = line.dashed ?? false
      const opacity = line.opacity ?? 0.9
      const sig = lineSignature(pts, color, width, dashed, opacity)
      const ex = ctx.lines.get(line.id)
      const options = { strokeColor: color, strokeWeight: width, strokeOpacity: opacity, strokeStyle: dashed ? 'dashed' : 'solid', strokeDasharray: dashed ? [8, 6] : undefined, lineJoin: 'round', lineCap: 'round', zIndex: 50 }
      if (ex) {
        if (ex.sig !== sig) {
          ex.overlay.setPath(pts)
          ex.overlay.setOptions(options)
          ex.sig = sig
        }
      } else {
        const overlay = new ctx.ns.Polyline({ path: pts, ...options })
        ctx.map.add(overlay)
        ctx.lines.set(line.id, { overlay, sig })
      }
    }
    for (const [id, ex] of ctx.lines) {
      if (!seen.has(id)) {
        ctx.map.remove(ex.overlay)
        ctx.lines.delete(id)
      }
    }
  }, [polylines, ready])

  // 视野适配：首次有数据、fitKey / center 变化时执行一次（放在同步之后，保证覆盖物已就绪）
  const hasData = markers.some((m) => isValidLngLat(m.lnglat)) || polylines.some((l) => l.points.length > 1)
  const fitSig = `${String(fitKey ?? '')}:${hasData ? 1 : 0}:${center ? center.join(',') : ''}`
  useEffect(() => {
    const ctx = ctxRef.current
    if (!ctx || !ready) return
    if (lastFitRef.current === fitSig) return
    lastFitRef.current = fitSig
    const overlays: AMapOverlay[] = []
    if (fitBounds) {
      for (const e of ctx.markers.values()) overlays.push(e.overlay)
      for (const e of ctx.lines.values()) overlays.push(e.overlay)
    }
    if (overlays.length === 0) {
      if (center && isValidLngLat(center)) ctx.map.setZoomAndCenter(zoom ?? 14, center)
      return
    }
    if (ctx.lines.size === 0 && ctx.markers.size === 1) {
      const only = markers.find((m) => isValidLngLat(m.lnglat))
      if (only) ctx.map.setZoomAndCenter(zoom ?? 15, only.lnglat)
      return
    }
    ctx.map.setFitView(overlays, false, [48, 48, 48, 48], 17)
  }, [fitSig, ready, fitBounds, center, zoom, markers])

  return (
    <>
      <div ref={containerRef} className="absolute inset-0" />
      {!ready && !error && (
        <div className="absolute inset-0 flex items-center justify-center bg-surface-2">
          <Spinner label="正在加载地图…" />
        </div>
      )}
      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-surface-2 p-4 text-center text-xs text-ink-muted">
          地图加载失败：{error}
        </div>
      )}
    </>
  )
}
