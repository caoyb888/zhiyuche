import type { ReactNode } from 'react'
import type { LngLat } from '../../types'

export type { LngLat }

/** 标记图形：车辆（圆 + 航向箭头）、充电桩、图钉、起 / 终点、小圆点 */
export type MarkerIcon = 'vehicle' | 'pile' | 'pin' | 'start' | 'end' | 'dot'

export interface MapMarker {
  id: string
  lnglat: LngLat
  /** 标记旁的文字（如车牌） */
  label?: string
  /** 填充色（CSS 颜色） */
  color?: string
  icon?: MarkerIcon
  /** 航向 0–360（vehicle 图形显示箭头） */
  heading?: number | null
  /** 0–1；离线车辆等半透明 */
  opacity?: number
  /** hover 提示 */
  title?: string
  /** 选中态（外圈高亮） */
  selected?: boolean
  zIndex?: number
  onClick?: (marker: MapMarker) => void
}

export interface MapPolyline {
  id: string
  points: LngLat[]
  color?: string
  width?: number
  dashed?: boolean
  opacity?: number
}

export interface MapViewProps {
  markers?: MapMarker[]
  polylines?: MapPolyline[]
  /** 无数据或不自动适配时的中心 */
  center?: LngLat
  /** 有 Key 时为高德缩放级别（3–20）；示意图按 fitBounds / center 自适应，忽略此值 */
  zoom?: number
  /**
   * 自动把视野适配到全部标记与折线（缺省 true）。
   * 仅在 fitKey 变化（或首次有数据）时执行，避免实时更新时视野被反复拉回。
   */
  fitBounds?: boolean
  /** 变化时重新执行 fitBounds；缺省为"首次有数据时" */
  fitKey?: string | number
  /** 点击地图空白处（非标记） */
  onClick?: (lnglat: LngLat) => void
  height?: number | string
  className?: string
  /** 叠加在地图容器上的内容（信息卡等，绝对定位，自行摆放） */
  overlay?: ReactNode
  /** 示意图模式下右上角的补充说明 */
  hint?: ReactNode
}

export type MapEngineKind = 'amap' | 'schematic'
