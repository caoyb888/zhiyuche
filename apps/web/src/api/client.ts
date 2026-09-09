import axios, { type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { getViewTenantId, useAuthStore } from '../store/auth'
import type { TokenPair } from './types'

declare module 'axios' {
  export interface AxiosRequestConfig {
    /** 不附加 Authorization 头（登录、刷新令牌等公开接口） */
    skipAuth?: boolean
    /** 收到 401 时不尝试刷新令牌（登录、刷新、退出本身） */
    skipAuthRefresh?: boolean
    /** 不附加 X-Tenant-ID（租户管理等平台级接口，或需要按本租户操作的请求） */
    skipTenantHeader?: boolean
    /** 内部标记：该请求已经因 401 刷新并重放过一次，避免死循环 */
    _retried?: boolean
  }
}

/** 超级管理员切换查看租户时附加的请求头（与后端 auth.HeaderTenant 一致） */
export const TENANT_HEADER = 'X-Tenant-ID'

/** 后端统一响应信封；错误时 data 缺省 */
export interface ApiEnvelope<T> {
  code: number
  message: string
  data?: T
  request_id?: string
}

/** 业务错误码（与 pkg/httpx 一致） */
export const ErrorCode = {
  BadRequest: 40000,
  Unauthorized: 40100,
  Forbidden: 40300,
  NotFound: 40400,
  Conflict: 40900,
  Internal: 50000,
  /** 网络层错误（无响应 / 非信封响应） */
  Network: -1,
} as const

/** 业务错误（信封 code != 0）或 HTTP 层错误 */
export class ApiError extends Error {
  readonly code: number
  readonly requestId: string | undefined
  readonly httpStatus: number | undefined

  constructor(message: string, code: number, requestId?: string, httpStatus?: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.requestId = requestId
    this.httpStatus = httpStatus
  }
}

export const isApiError = (e: unknown): e is ApiError => e instanceof ApiError

/** 从任意异常里取可展示的信息 */
export function errorMessage(e: unknown, fallback = '请求失败，请稍后重试'): string {
  if (e instanceof Error && e.message) return e.message
  return fallback
}

function isEnvelope(value: unknown): value is ApiEnvelope<unknown> {
  if (typeof value !== 'object' || value === null) return false
  const v = value as Record<string, unknown>
  return typeof v.code === 'number' && typeof v.message === 'string'
}

/** responseType 为 blob 时错误体也是 Blob，需要先读出 JSON */
async function readErrorBody(data: unknown): Promise<unknown> {
  if (typeof Blob !== 'undefined' && data instanceof Blob) {
    if (!data.type.includes('json')) return undefined
    try {
      return JSON.parse(await data.text()) as unknown
    } catch {
      return undefined
    }
  }
  return data
}

export const client = axios.create({
  baseURL: import.meta.env.VITE_API_BASE ?? '/api/v1',
  timeout: 10_000,
  headers: { Accept: 'application/json' },
})

// ── 请求拦截：附加 Bearer 与（超级管理员）X-Tenant-ID ────
client.interceptors.request.use((config) => {
  if (!config.skipAuth) {
    const token = useAuthStore.getState().accessToken
    if (token) config.headers.set('Authorization', `Bearer ${token}`)
    // /auth/* 是关于调用者本人的接口，不随查看租户切换
    const isAuthApi = (config.url ?? '').startsWith('/auth')
    if (!config.skipTenantHeader && !isAuthApi) {
      const tenantId = getViewTenantId()
      if (tenantId) config.headers.set(TENANT_HEADER, tenantId)
    }
  }
  return config
})

// ── 令牌刷新：并发 401 只刷新一次，其余等待同一个 Promise ──
let refreshing: Promise<string | null> | null = null

async function doRefresh(): Promise<string | null> {
  const { refreshToken, setTokens, clear } = useAuthStore.getState()
  if (!refreshToken) {
    clear()
    return null
  }
  try {
    const pair = await request<TokenPair>({
      method: 'POST',
      url: '/auth/refresh',
      data: { refresh_token: refreshToken },
      skipAuth: true,
      skipAuthRefresh: true,
    })
    setTokens(pair)
    return pair.access_token
  } catch {
    // refresh token 失效：清空状态，路由守卫会跳转到 /login
    clear()
    return null
  }
}

function refreshAccessToken(): Promise<string | null> {
  if (!refreshing) {
    refreshing = doRefresh().finally(() => {
      refreshing = null
    })
  }
  return refreshing
}

// ── 响应拦截：解信封 / 401 刷新重放 / 统一 ApiError ───────
client.interceptors.response.use(
  (response: AxiosResponse<unknown>) => {
    const body = response.data
    if (isEnvelope(body)) {
      if (body.code !== 0) {
        throw new ApiError(body.message || `请求失败 (code ${body.code})`, body.code, body.request_id, response.status)
      }
      return { ...response, data: body.data }
    }
    // 非信封（如文件下载）原样返回
    return response
  },
  async (error: unknown) => {
    if (!axios.isAxiosError(error)) throw error

    const config = error.config
    const status = error.response?.status

    if (status === 401 && config && !config.skipAuthRefresh && !config._retried) {
      const token = await refreshAccessToken()
      if (token) {
        config._retried = true
        config.headers.set('Authorization', `Bearer ${token}`)
        return client.request(config)
      }
    }

    const body = await readErrorBody(error.response?.data)
    if (isEnvelope(body)) {
      throw new ApiError(body.message || error.message, body.code, body.request_id, status)
    }
    const msg = error.code === 'ECONNABORTED'
      ? '请求超时，请稍后重试'
      : status
        ? `服务异常 (HTTP ${status})`
        : '网络连接失败，请检查后端服务'
    throw new ApiError(msg, ErrorCode.Network, undefined, status)
  },
)

/** 发起请求并直接返回解开信封后的 data */
export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const res = await client.request<T>(config)
  return res.data
}

export const get = <T>(url: string, config?: AxiosRequestConfig): Promise<T> =>
  request<T>({ ...config, method: 'GET', url })

export const post = <T, B = unknown>(url: string, body?: B, config?: AxiosRequestConfig): Promise<T> =>
  request<T>({ ...config, method: 'POST', url, data: body })

export const put = <T, B = unknown>(url: string, body?: B, config?: AxiosRequestConfig): Promise<T> =>
  request<T>({ ...config, method: 'PUT', url, data: body })

export const del = <T>(url: string, config?: AxiosRequestConfig): Promise<T> =>
  request<T>({ ...config, method: 'DELETE', url })

/** 下载文件（响应为二进制）；文件名优先取 Content-Disposition */
export interface DownloadedFile {
  blob: Blob
  filename: string
}

export async function download(url: string, fallbackName: string, config?: AxiosRequestConfig): Promise<DownloadedFile> {
  const res = await client.request<Blob>({ ...config, method: 'GET', url, responseType: 'blob', timeout: 60_000 })
  const disposition = res.headers['content-disposition']
  return {
    blob: res.data,
    filename: filenameFromDisposition(typeof disposition === 'string' ? disposition : undefined) ?? fallbackName,
  }
}

function filenameFromDisposition(header: string | undefined): string | undefined {
  if (!header) return undefined
  const utf8 = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (utf8?.[1]) {
    try {
      return decodeURIComponent(utf8[1])
    } catch {
      return undefined
    }
  }
  const plain = /filename="?([^";]+)"?/i.exec(header)
  return plain?.[1]
}

/** 去掉 undefined / null / 空字符串的查询参数 */
export function compactParams<T extends object>(params: T): Partial<T> {
  const out: Partial<T> = {}
  for (const [k, v] of Object.entries(params) as [keyof T, T[keyof T]][]) {
    if (v === undefined || v === null || v === '') continue
    out[k] = v
  }
  return out
}

export default client
