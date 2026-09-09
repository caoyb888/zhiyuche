import { compactParams, del, download, get, post, put, type DownloadedFile } from './client'
import type { ImportResult, Page, PageQuery, User, UserCreate, UserStatus, UserUpdate } from './types'

/** GET /system/users 的筛选条件（不含分页） */
export interface UserFilter {
  keyword?: string
  /** 含子部门 */
  dept_id?: string
  status?: UserStatus | ''
  role_id?: string
}

export type UserListParams = UserFilter & PageQuery

export const userKeys = {
  all: ['users'] as const,
  list: (params: UserListParams) => ['users', 'list', params] as const,
  detail: (id: string) => ['users', 'detail', id] as const,
}

export const listUsers = (params: UserListParams): Promise<Page<User>> =>
  get<Page<User>>('/system/users', { params: compactParams(params) })

export const getUser = (id: string): Promise<User> => get<User>(`/system/users/${id}`)

export const createUser = (body: UserCreate): Promise<User> =>
  post<User, UserCreate>('/system/users', body)

export const updateUser = (id: string, body: UserUpdate): Promise<User> =>
  put<User, UserUpdate>(`/system/users/${id}`, body)

export const deleteUser = (id: string): Promise<unknown> => del<unknown>(`/system/users/${id}`)

export interface ResetPasswordResult {
  /** 明文密码，仅返回一次 */
  password: string
}

/** 不传 password 则使用系统缺省密码 */
export const resetUserPassword = (id: string, password?: string): Promise<ResetPasswordResult> =>
  post<ResetPasswordResult>(`/system/users/${id}/reset-password`, password ? { password } : {})

export const assignUserRoles = (id: string, role_ids: string[]): Promise<User> =>
  put<User>(`/system/users/${id}/roles`, { role_ids })

/** POST /system/users/import —— multipart 上传 xlsx */
export const importUsers = (file: File): Promise<ImportResult> => {
  const form = new FormData()
  form.append('file', file)
  return post<ImportResult, FormData>('/system/users/import', form, { timeout: 120_000 })
}

export const downloadUserImportTemplate = (): Promise<DownloadedFile> =>
  download('/system/users/import-template', '用户导入模板.xlsx')

export const exportUsers = (filter: UserFilter): Promise<DownloadedFile> =>
  download('/system/users/export', '用户列表.xlsx', { params: compactParams(filter) })
