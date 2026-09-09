import { useEffect, type ReactNode } from 'react'
import { mapEngineStore } from './engine'
import { MapEngineContext } from './map-context'
import { useEngineSnapshot } from './useMapEngine'

export interface MapProviderProps {
  /** 覆盖 VITE_AMAP_KEY；传空字符串强制示意图 */
  amapKey?: string
  children: ReactNode
}

/**
 * 地图引擎上下文：读 VITE_AMAP_KEY 决定使用高德还是自绘示意图。
 * 可选——未包裹时 useMapEngine 直接使用模块级单例，行为一致。
 */
export default function MapProvider({ amapKey, children }: MapProviderProps) {
  useEffect(() => {
    if (amapKey !== undefined) mapEngineStore.setKey(amapKey)
  }, [amapKey])
  const api = useEngineSnapshot()
  return <MapEngineContext.Provider value={api}>{children}</MapEngineContext.Provider>
}
