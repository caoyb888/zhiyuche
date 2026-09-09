/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 后端 API 前缀，默认 /api/v1（dev 由 vite proxy 转发到 20080） */
  readonly VITE_API_BASE?: string
  /** WebSocket 地址 */
  readonly VITE_WS_URL?: string
  /** 高德地图 Web JS API Key */
  readonly VITE_AMAP_KEY?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
