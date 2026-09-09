import type { LngLat } from '../types'

export type { LngLat }

const EARTH_RADIUS_M = 6_371_000
const DEG = Math.PI / 180

/** 两点大圆距离（米），Haversine 公式 */
export function haversineMeters(a: LngLat, b: LngLat): number {
  const [lng1, lat1] = a
  const [lng2, lat2] = b
  const dLat = (lat2 - lat1) * DEG
  const dLng = (lng2 - lng1) * DEG
  const s = Math.sin(dLat / 2) ** 2 + Math.cos(lat1 * DEG) * Math.cos(lat2 * DEG) * Math.sin(dLng / 2) ** 2
  return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(s)))
}

/** 折线总长度（米） */
export function pathLengthMeters(points: readonly LngLat[]): number {
  let total = 0
  for (let i = 1; i < points.length; i++) total += haversineMeters(points[i - 1], points[i])
  return total
}

/** 米 → 可读文本（<1km 显示米，否则保留 1–2 位小数的公里） */
export function formatDistance(meters: number | null | undefined): string {
  if (meters === null || meters === undefined || !Number.isFinite(meters)) return '—'
  if (meters < 1000) return `${Math.round(meters)} m`
  const km = meters / 1000
  return `${km < 10 ? km.toFixed(2) : km.toFixed(1)} km`
}

export interface Bounds {
  minLng: number
  minLat: number
  maxLng: number
  maxLat: number
}

/** 包围盒；没有有效点时返回 null */
export function boundsOf(points: Iterable<LngLat>): Bounds | null {
  let b: Bounds | null = null
  for (const [lng, lat] of points) {
    if (!isValidLngLat([lng, lat])) continue
    if (!b) b = { minLng: lng, minLat: lat, maxLng: lng, maxLat: lat }
    else {
      if (lng < b.minLng) b.minLng = lng
      if (lng > b.maxLng) b.maxLng = lng
      if (lat < b.minLat) b.minLat = lat
      if (lat > b.maxLat) b.maxLat = lat
    }
  }
  return b
}

/** 包围盒外扩：按比例（相对宽高）与最小跨度（度） */
export function padBounds(b: Bounds, ratio = 0.1, minSpanDeg = 0.005): Bounds {
  let w = b.maxLng - b.minLng
  let h = b.maxLat - b.minLat
  let cx = (b.minLng + b.maxLng) / 2
  let cy = (b.minLat + b.maxLat) / 2
  if (w < minSpanDeg) w = minSpanDeg
  if (h < minSpanDeg) h = minSpanDeg
  w *= 1 + ratio * 2
  h *= 1 + ratio * 2
  // 纬度限制在有效范围内
  cy = Math.max(-85 + h / 2, Math.min(85 - h / 2, cy))
  cx = Math.max(-180 + w / 2, Math.min(180 - w / 2, cx))
  return { minLng: cx - w / 2, minLat: cy - h / 2, maxLng: cx + w / 2, maxLat: cy + h / 2 }
}

export function boundsCenter(b: Bounds): LngLat {
  return [(b.minLng + b.maxLng) / 2, (b.minLat + b.maxLat) / 2]
}

export function isValidLngLat(p: readonly [number, number] | null | undefined): p is LngLat {
  if (!p) return false
  const [lng, lat] = p
  return Number.isFinite(lng) && Number.isFinite(lat) && Math.abs(lng) <= 180 && Math.abs(lat) <= 90 && !(lng === 0 && lat === 0)
}

/** 从可能为 null 的 lng/lat 字段构造坐标 */
export function toLngLat(lng: number | null | undefined, lat: number | null | undefined): LngLat | null {
  if (lng === null || lng === undefined || lat === null || lat === undefined) return null
  const p: LngLat = [lng, lat]
  return isValidLngLat(p) ? p : null
}

/** 点到线段的垂直距离（用度做近似平面计算，抽稀足够） */
function perpendicularDistance(p: LngLat, a: LngLat, b: LngLat): number {
  const dx = b[0] - a[0]
  const dy = b[1] - a[1]
  if (dx === 0 && dy === 0) return Math.hypot(p[0] - a[0], p[1] - a[1])
  const t = ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / (dx * dx + dy * dy)
  const cx = a[0] + Math.max(0, Math.min(1, t)) * dx
  const cy = a[1] + Math.max(0, Math.min(1, t)) * dy
  return Math.hypot(p[0] - cx, p[1] - cy)
}

/**
 * Douglas-Peucker 折线抽稀（迭代实现，避免长轨迹递归过深）。
 * tolerance 单位为度，0.00005° ≈ 5 m。返回保留点的下标（升序）。
 */
export function simplifyIndices(points: readonly LngLat[], tolerance = 0.00005): number[] {
  const n = points.length
  if (n <= 2) return points.map((_, i) => i)
  const keep = new Uint8Array(n)
  keep[0] = 1
  keep[n - 1] = 1
  const stack: Array<[number, number]> = [[0, n - 1]]
  while (stack.length > 0) {
    const [s, e] = stack.pop() as [number, number]
    let maxD = 0
    let idx = -1
    for (let i = s + 1; i < e; i++) {
      const d = perpendicularDistance(points[i], points[s], points[e])
      if (d > maxD) {
        maxD = d
        idx = i
      }
    }
    if (idx !== -1 && maxD > tolerance) {
      keep[idx] = 1
      stack.push([s, idx], [idx, e])
    }
  }
  const out: number[] = []
  for (let i = 0; i < n; i++) if (keep[i]) out.push(i)
  return out
}

export function simplifyPath(points: readonly LngLat[], tolerance?: number): LngLat[] {
  return simplifyIndices(points, tolerance).map((i) => points[i])
}

/** 等间隔抽样到不超过 max 个点（首尾保留），用于图表 */
export function sampleEvenly<T>(items: readonly T[], max: number): T[] {
  if (max <= 0 || items.length <= max) return [...items]
  if (max === 1) return [items[0]]
  const out: T[] = []
  const step = (items.length - 1) / (max - 1)
  for (let i = 0; i < max; i++) out.push(items[Math.round(i * step)])
  return out
}

/** 在一组 [lng, lat] 中找离目标最近的点下标；空数组返回 -1 */
export function nearestIndex(points: readonly LngLat[], target: LngLat): number {
  let best = -1
  let bestD = Infinity
  for (let i = 0; i < points.length; i++) {
    const d = haversineMeters(points[i], target)
    if (d < bestD) {
      bestD = d
      best = i
    }
  }
  return best
}

/** 两点间方位角（0–360，北为 0，顺时针） */
export function bearing(a: LngLat, b: LngLat): number {
  const φ1 = a[1] * DEG
  const φ2 = b[1] * DEG
  const Δλ = (b[0] - a[0]) * DEG
  const y = Math.sin(Δλ) * Math.cos(φ2)
  const x = Math.cos(φ1) * Math.sin(φ2) - Math.sin(φ1) * Math.cos(φ2) * Math.cos(Δλ)
  return ((Math.atan2(y, x) / DEG) + 360) % 360
}

/** 经纬度 → 6 位小数文本 */
export function formatLngLat(p: LngLat | null | undefined): string {
  if (!p) return '—'
  return `${p[0].toFixed(6)}, ${p[1].toFixed(6)}`
}
