import { compactParams, del, get, post, put } from './client'
import type { DictItem, DictItemCreate, DictItemUpdate, DictType, DictTypeCreate, DictTypeUpdate, Page, PageQuery } from './types'

export interface DictTypeFilter {
  keyword?: string
}

export type DictTypeListParams = DictTypeFilter & PageQuery

export const dictKeys = {
  all: ['dicts'] as const,
  list: (params: DictTypeListParams) => ['dicts', 'list', params] as const,
  detail: (id: string) => ['dicts', 'detail', id] as const,
  byCode: (code: string) => ['dicts', 'code', code] as const,
}

/** GET /system/dict-types —— 全局 + 本租户，分页；列表项 items 为空数组 */
export const listDictTypes = (params: DictTypeListParams): Promise<Page<DictType>> =>
  get<Page<DictType>>('/system/dict-types', { params: compactParams(params) })

/** GET /system/dict-types/{id} —— 含条目（按 sort 升序） */
export const getDictType = (id: string): Promise<DictType> => get<DictType>(`/system/dict-types/${id}`)

/** 超级管理员未切换查看租户时创建全局字典 */
export const createDictType = (body: DictTypeCreate): Promise<DictType> =>
  post<DictType, DictTypeCreate>('/system/dict-types', body)

/** 只能改名称 / 描述；全局字典仅超级管理员可改（否则 403） */
export const updateDictType = (id: string, body: DictTypeUpdate): Promise<DictType> =>
  put<DictType, DictTypeUpdate>(`/system/dict-types/${id}`, body)

/** 系统字典返回 409 */
export const deleteDictType = (id: string): Promise<unknown> => del<unknown>(`/system/dict-types/${id}`)

export const createDictItem = (typeId: string, body: DictItemCreate): Promise<DictItem> =>
  post<DictItem, DictItemCreate>(`/system/dict-types/${typeId}/items`, body)

export const updateDictItem = (id: string, body: DictItemUpdate): Promise<DictItem> =>
  put<DictItem, DictItemUpdate>(`/system/dict-items/${id}`, body)

export const deleteDictItem = (id: string): Promise<unknown> => del<unknown>(`/system/dict-items/${id}`)

/** GET /system/dicts/{code} —— 任意登录用户可读的启用条目（租户覆盖优先） */
export const getDictByCode = (code: string): Promise<DictItem[]> => get<DictItem[]>(`/system/dicts/${encodeURIComponent(code)}`)
