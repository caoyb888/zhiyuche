/**
 * 高德 JS API 2.0 的最小类型声明：只覆盖本项目用到的成员，避免引入整套官方类型包。
 * 官方文档：https://lbs.amap.com/api/javascript-api-v2/documentation
 */

export interface AMapLngLat {
  lng: number
  lat: number
  getLng(): number
  getLat(): number
}

export interface AMapMapEvent {
  lnglat: AMapLngLat
}

export interface AMapOverlay {
  setMap(map: AMapMap | null): void
  on(event: string, handler: (e: AMapMapEvent) => void): void
  off(event: string, handler: (e: AMapMapEvent) => void): void
}

export interface AMapMarker extends AMapOverlay {
  setPosition(position: [number, number]): void
  setContent(content: string | HTMLElement): void
  setTitle(title: string): void
  setzIndex(z: number): void
  setExtData(data: unknown): void
  getExtData(): unknown
}

export interface AMapPolyline extends AMapOverlay {
  setPath(path: Array<[number, number]>): void
  setOptions(options: Record<string, unknown>): void
}

export interface AMapMap {
  add(overlays: AMapOverlay | AMapOverlay[]): void
  remove(overlays: AMapOverlay | AMapOverlay[]): void
  addControl(control: unknown): void
  setFitView(overlays?: AMapOverlay[] | null, immediately?: boolean, avoid?: [number, number, number, number], maxZoom?: number): void
  setZoomAndCenter(zoom: number, center: [number, number], immediately?: boolean): void
  setCenter(center: [number, number]): void
  setZoom(zoom: number): void
  getZoom(): number
  on(event: string, handler: (e: AMapMapEvent) => void): void
  off(event: string, handler: (e: AMapMapEvent) => void): void
  destroy(): void
}

export interface AMapPoi {
  id?: string
  name: string
  /** 部分结果为空数组 */
  address?: string | string[]
  location?: AMapLngLat | null
  pname?: string
  cityname?: string
  adname?: string
}

export interface AMapPlaceSearchResult {
  info?: string
  poiList?: { pois?: AMapPoi[]; count?: number }
}

export type AMapServiceStatus = 'complete' | 'error' | 'no_data'

export interface AMapPlaceSearch {
  search(keyword: string, callback: (status: AMapServiceStatus, result: AMapPlaceSearchResult | string) => void): void
}

export interface AMapGeocoderResult {
  regeocode?: { formattedAddress?: string }
}

export interface AMapGeocoder {
  getAddress(lnglat: [number, number], callback: (status: AMapServiceStatus, result: AMapGeocoderResult | string) => void): void
}

/** `AMapLoader.load()` 返回的命名空间 */
export interface AMapNS {
  Map: new (container: HTMLElement | string, options?: Record<string, unknown>) => AMapMap
  Marker: new (options: Record<string, unknown>) => AMapMarker
  Polyline: new (options: Record<string, unknown>) => AMapPolyline
  Scale: new (options?: Record<string, unknown>) => unknown
  Pixel: new (x: number, y: number) => unknown
  PlaceSearch: new (options?: Record<string, unknown>) => AMapPlaceSearch
  Geocoder: new (options?: Record<string, unknown>) => AMapGeocoder
}

/** 高德安全密钥需在脚本加载前写入 window._AMapSecurityConfig */
declare global {
  interface Window {
    _AMapSecurityConfig?: { securityJsCode?: string; serviceHost?: string }
  }
}
