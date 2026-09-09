import { get } from './client'

export interface SystemHealth {
  status: string
  version: string
  uptime_seconds: number
}

/** GET /health —— 后端存活与版本信息 */
export const getHealth = (): Promise<SystemHealth> => get<SystemHealth>('/health')
