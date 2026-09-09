import axios, { type AxiosRequestConfig, type AxiosResponse } from 'axios'

/** 后端统一响应信封 */
export interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
  request_id?: string
}

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

function isEnvelope(value: unknown): value is ApiEnvelope<unknown> {
  if (typeof value !== 'object' || value === null) return false
  const v = value as Record<string, unknown>
  return typeof v.code === 'number' && typeof v.message === 'string' && 'data' in v
}

export const client = axios.create({
  baseURL: import.meta.env.VITE_API_BASE ?? '/api/v1',
  timeout: 10_000,
  headers: { Accept: 'application/json' },
})

// 解开信封：成功时把 response.data 替换为 data 字段；code != 0 抛 ApiError
client.interceptors.response.use(
  (response: AxiosResponse<unknown>) => {
    const body = response.data
    if (isEnvelope(body)) {
      if (body.code !== 0) {
        throw new ApiError(body.message || `请求失败 (code ${body.code})`, body.code, body.request_id, response.status)
      }
      return { ...response, data: body.data }
    }
    return response
  },
  (error: unknown) => {
    if (axios.isAxiosError(error)) {
      const body: unknown = error.response?.data
      if (isEnvelope(body)) {
        throw new ApiError(body.message || error.message, body.code, body.request_id, error.response?.status)
      }
      throw new ApiError(error.message, -1, undefined, error.response?.status)
    }
    throw error
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

export default client
