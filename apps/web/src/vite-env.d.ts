/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 后端 API 前缀，默认 /api/v1（dev 由 vite proxy 转发到 20080） */
  readonly VITE_API_BASE?: string
  /** WebSocket 完整地址；留空按页面地址 + VITE_API_BASE + /ws 推导 */
  readonly VITE_WS_URL?: string
  /** 高德地图 Web JS API Key；留空则地图降级为自绘示意图 */
  readonly VITE_AMAP_KEY?: string
  /** 高德安全密钥（可选，PlaceSearch 等服务需要） */
  readonly VITE_AMAP_SECURITY_CODE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
