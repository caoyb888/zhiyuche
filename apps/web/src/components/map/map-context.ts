import { createContext } from 'react'
import type { AMapNS } from './amap-types'
import type { MapEngineSnapshot } from './engine'

export interface MapEngineApi extends MapEngineSnapshot {
  /** 加载高德 JS API（有 Key 时）；失败会自动降级为示意图 */
  loadAMap: () => Promise<AMapNS>
}

export const MapEngineContext = createContext<MapEngineApi | null>(null)
