import { get } from './client'
import type { Health, Ready } from './types'

export type SystemHealth = Health

/** GET /health —— 后端存活与版本信息（公开接口） */
export const getHealth = (): Promise<Health> => get<Health>('/health', { skipAuth: true, skipAuthRefresh: true })

/** GET /ready —— 依赖就绪状态 */
export const getReady = (): Promise<Ready> => get<Ready>('/ready', { skipAuth: true, skipAuthRefresh: true })
