import { useContext, useMemo, useSyncExternalStore } from 'react'
import { mapEngineStore } from './engine'
import { MapEngineContext, type MapEngineApi } from './map-context'

export type { MapEngineApi } from './map-context'

/** 订阅模块级引擎单例（MapProvider 与无 Provider 的回退共用） */
export function useEngineSnapshot(): MapEngineApi {
  const snap = useSyncExternalStore(mapEngineStore.subscribe, mapEngineStore.getSnapshot, mapEngineStore.getSnapshot)
  return useMemo(() => ({ ...snap, loadAMap: () => mapEngineStore.load() }), [snap])
}

/**
 * 当前地图引擎（amap / schematic）及高德加载状态。
 * 有 <MapProvider> 时取上下文；没有时直接使用单例，行为一致。
 */
export function useMapEngine(): MapEngineApi {
  const ctx = useContext(MapEngineContext)
  const fallback = useEngineSnapshot()
  return ctx ?? fallback
}
