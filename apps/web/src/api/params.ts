import { compactParams, del, get, put } from './client'
import type { Param, ParamUpdate } from './types'

export interface ParamFilter {
  keyword?: string
}

export const paramKeys = {
  all: ['params'] as const,
  list: (params: ParamFilter) => ['params', 'list', params] as const,
}

/** GET /system/params —— 不分页；租户覆盖优先，否则全局缺省 */
export const listParams = (params: ParamFilter = {}): Promise<Param[]> =>
  get<Param[]>('/system/params', { params: compactParams(params) })

/**
 * PUT /system/params/{key}
 * 普通租户写租户覆盖；超级管理员未切换查看租户时写全局缺省。值按 value_type 由后端校验。
 */
export const setParam = (key: string, body: ParamUpdate): Promise<Param> =>
  put<Param, ParamUpdate>(`/system/params/${encodeURIComponent(key)}`, body)

/** DELETE /system/params/{key} —— 删除租户覆盖值，恢复全局缺省（全局作用域下 400） */
export const resetParam = (key: string): Promise<unknown> => del<unknown>(`/system/params/${encodeURIComponent(key)}`)
