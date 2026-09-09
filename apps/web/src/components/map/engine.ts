import type { AMapNS } from './amap-types'
import type { MapEngineKind } from './types'

export type AMapLoadStatus = 'idle' | 'loading' | 'ready' | 'failed'

export interface MapEngineSnapshot {
  /** 当前应使用的引擎：有 Key 且未加载失败 → amap，否则 schematic */
  engine: MapEngineKind
  amapKey: string | null
  status: AMapLoadStatus
  ns: AMapNS | null
  error: string | null
}

/** 高德 JS API 需要的插件（选点搜索、逆地理、比例尺、驾车路线规划） */
const AMAP_PLUGINS = ['AMap.Scale', 'AMap.PlaceSearch', 'AMap.Geocoder', 'AMap.Driving']

function envKey(): string | null {
  const key = (import.meta.env.VITE_AMAP_KEY ?? '').trim()
  return key ? key : null
}

/**
 * 模块级单例：整个应用只加载一次高德脚本；无 Key 或加载失败时所有地图统一降级为示意图。
 * 通过 subscribe/getSnapshot 供 useSyncExternalStore 订阅。
 */
class MapEngineStore {
  private snapshot: MapEngineSnapshot
  private listeners = new Set<() => void>()
  private loading: Promise<AMapNS> | null = null

  constructor() {
    const key = envKey()
    this.snapshot = { engine: key ? 'amap' : 'schematic', amapKey: key, status: 'idle', ns: null, error: null }
  }

  getSnapshot = (): MapEngineSnapshot => this.snapshot

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  private set(patch: Partial<MapEngineSnapshot>): void {
    this.snapshot = { ...this.snapshot, ...patch }
    this.listeners.forEach((l) => l())
  }

  /** 覆盖 Key（MapProvider 显式传入时）；空字符串 = 强制示意图 */
  setKey(key: string | null): void {
    const next = key && key.trim() ? key.trim() : null
    if (next === this.snapshot.amapKey) return
    this.loading = null
    this.set({ amapKey: next, engine: next ? 'amap' : 'schematic', status: 'idle', ns: null, error: null })
  }

  /** 加载高德 JS API；失败时切换到示意图并返回 reject */
  load(): Promise<AMapNS> {
    const { amapKey, ns } = this.snapshot
    if (ns) return Promise.resolve(ns)
    if (!amapKey) return Promise.reject(new Error('未配置高德地图 Key'))
    if (this.loading) return this.loading
    this.set({ status: 'loading', error: null })
    const securityCode = (import.meta.env.VITE_AMAP_SECURITY_CODE ?? '').trim()
    if (securityCode && typeof window !== 'undefined') {
      window._AMapSecurityConfig = { securityJsCode: securityCode }
    }
    this.loading = import('@amap/amap-jsapi-loader')
      .then((mod) => mod.default.load({ key: amapKey, version: '2.0', plugins: AMAP_PLUGINS }) as Promise<AMapNS>)
      .then((loaded) => {
        this.set({ status: 'ready', ns: loaded, engine: 'amap' })
        return loaded
      })
      .catch((e: unknown) => {
        const message = e instanceof Error ? e.message : typeof e === 'string' ? e : '高德地图加载失败'
        this.loading = null
        this.set({ status: 'failed', error: message, engine: 'schematic', ns: null })
        throw e instanceof Error ? e : new Error(message)
      })
    return this.loading
  }
}

export const mapEngineStore = new MapEngineStore()
