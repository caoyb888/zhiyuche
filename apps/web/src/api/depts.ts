import { compactParams, del, get, post, put } from './client'
import type { Dept, DeptCreate, DeptNode, DeptStatus, DeptUpdate } from './types'

export const deptKeys = {
  all: ['depts'] as const,
  tree: ['depts', 'tree'] as const,
  list: (params: DeptListParams) => ['depts', 'list', params] as const,
  detail: (id: string) => ['depts', 'detail', id] as const,
}

export interface DeptListParams {
  keyword?: string
  status?: DeptStatus | ''
}

/** GET /system/depts/tree —— 整棵树，含直属在职人数 */
export const getDeptTree = (): Promise<DeptNode[]> => get<DeptNode[]>('/system/depts/tree')

/** GET /system/depts —— 平铺列表（不分页） */
export const listDepts = (params: DeptListParams = {}): Promise<Dept[]> =>
  get<Dept[]>('/system/depts', { params: compactParams(params) })

export const getDept = (id: string): Promise<Dept> => get<Dept>(`/system/depts/${id}`)

export const createDept = (body: DeptCreate): Promise<Dept> =>
  post<Dept, DeptCreate>('/system/depts', body)

export const updateDept = (id: string, body: DeptUpdate): Promise<Dept> =>
  put<Dept, DeptUpdate>(`/system/depts/${id}`, body)

export const deleteDept = (id: string): Promise<unknown> => del<unknown>(`/system/depts/${id}`)
