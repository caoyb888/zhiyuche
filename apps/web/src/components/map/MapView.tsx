import clsx from 'clsx'
import AMapView from './AMapView'
import SchematicMap from './SchematicMap'
import type { MapViewProps } from './types'
import { useMapEngine } from './useMapEngine'

export interface MapViewExtraProps {
  /** 示意图模式下单点 / 无数据时的最小视野跨度（度），缺省 0.01 ≈ 1 km */
  spanDeg?: number
}

/**
 * 统一地图组件：有高德 Key → AMapView；无 Key 或加载失败 → SchematicMap（SVG 示意图）。
 * 两种引擎渲染同一套 props（markers / polylines / center / zoom / fitBounds / onClick / overlay）。
 * overlay 内的可交互元素会自动获得 pointer-events。
 */
export default function MapView({ height = 320, className, overlay, spanDeg, ...rest }: MapViewProps & MapViewExtraProps) {
  const { engine } = useMapEngine()
  return (
    <div className={clsx('relative w-full overflow-hidden rounded-xl border border-slate-100 bg-slate-50', className)} style={{ height }}>
      {engine === 'amap' ? <AMapView {...rest} /> : <SchematicMap {...rest} spanDeg={spanDeg} />}
      {overlay && <div className="pointer-events-none absolute inset-0 z-10 [&>*]:pointer-events-auto">{overlay}</div>}
    </div>
  )
}
