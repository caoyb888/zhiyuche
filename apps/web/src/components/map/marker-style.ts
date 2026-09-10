import type { MapMarker, MarkerIcon } from './types'

export const DEFAULT_MARKER_COLOR = '#1d6fd8'
export const DEFAULT_LINE_COLOR = '#1d6fd8'
/** 无任何点时的缺省中心（北京） */
export const DEFAULT_CENTER: [number, number] = [116.3975, 39.9087]

/** 各图形的半径（px） */
export const MARKER_RADIUS: Record<MarkerIcon, number> = {
  vehicle: 9,
  pile: 8,
  pin: 8,
  start: 9,
  end: 9,
  dot: 4,
}

export const MARKER_GLYPH: Partial<Record<MarkerIcon, string>> = {
  start: '起',
  end: '终',
  pile: '⚡',
}

export function markerColor(m: MapMarker): string {
  if (m.color) return m.color
  if (m.icon === 'start') return '#10b981'
  if (m.icon === 'end') return '#ef4b3c'
  return DEFAULT_MARKER_COLOR
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] ?? c)
}

/**
 * 高德 Marker 的自定义 content（HTML 字符串），与示意图上的 SVG 图形保持同一视觉：
 * 圆形色块 + 白描边 + 可选航向箭头 + 下方车牌标签。锚点为圆心。
 */
export function markerHtml(m: MapMarker): string {
  const icon = m.icon ?? 'dot'
  const r = MARKER_RADIUS[icon]
  const color = markerColor(m)
  const opacity = m.opacity ?? 1
  const glyph = MARKER_GLYPH[icon]
  const ring = m.selected ? `box-shadow:0 0 0 3px ${color}55,0 0 0 5px #fff;` : ''
  const heading = icon === 'vehicle' && typeof m.heading === 'number' && Number.isFinite(m.heading) ? m.heading : null
  const arrow =
    heading === null
      ? ''
      : `<div style="position:absolute;left:50%;top:50%;width:0;height:0;transform:translate(-50%,-50%) rotate(${heading}deg) translateY(-${r + 7}px);border-left:5px solid transparent;border-right:5px solid transparent;border-bottom:8px solid ${color};"></div>`
  const label = m.label
    ? `<div style="position:absolute;left:50%;top:${r + 3}px;transform:translateX(-50%);white-space:nowrap;font:600 11px/1.2 system-ui,sans-serif;color:#dce7f2;background:rgba(13,30,51,.92);border:1px solid #23456b;border-radius:4px;padding:1px 4px;pointer-events:none;">${escapeHtml(m.label)}</div>`
    : ''
  const glyphHtml = glyph
    ? `<span style="position:absolute;inset:0;display:flex;align-items:center;justify-content:center;font:700 ${r + 1}px/1 system-ui,sans-serif;color:#fff;">${glyph}</span>`
    : ''
  return `<div style="position:relative;width:0;height:0;opacity:${opacity};"><div style="position:absolute;left:${-r}px;top:${-r}px;width:${r * 2}px;height:${r * 2}px;border-radius:9999px;background:${color};border:2px solid #fff;box-sizing:border-box;${ring}box-shadow:${m.selected ? `0 0 0 3px ${color}55` : '0 1px 3px rgba(0,0,0,.35)'};">${glyphHtml}</div>${arrow}${label}</div>`
}
