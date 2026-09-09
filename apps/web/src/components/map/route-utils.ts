import type { PlannedRoute } from '../../api/types'
import { isValidLngLat, type LngLat } from '../../utils/geo'

/** 把契约里的 PlannedRoute.points（number[][]）收窄为有效坐标 */
export function routePoints(route: PlannedRoute | null | undefined): LngLat[] {
  return (route?.points ?? []).filter((p) => Array.isArray(p) && p.length >= 2 && isValidLngLat([p[0], p[1]])).map((p): LngLat => [p[0], p[1]])
}
