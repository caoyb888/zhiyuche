import { compactParams, del, get, post, put } from './client'
import type { Page, PageQuery, PermissionNode, Role, RoleBrief, RoleCreate, RoleUpdate } from './types'

/** GET /system/roles 的筛选条件（不含分页） */
export interface RoleFilter {
  keyword?: string
}

export type RoleListParams = RoleFilter & PageQuery

export const roleKeys = {
  all: ['roles'] as const,
  options: ['roles', 'options'] as const,
  list: (params: RoleListParams) => ['roles', 'list', params] as const,
  detail: (id: string) => ['roles', 'detail', id] as const,
  permissionTree: ['permissions', 'tree'] as const,
}

/** GET /system/roles —— 分页；平台租户额外可见平台级角色 */
export const listRoles = (params: RoleListParams): Promise<Page<Role>> =>
  get<Page<Role>>('/system/roles', { params: compactParams(params) })

/** GET /system/roles/options —— 本租户全部角色简表（下拉用） */
export const listRoleOptions = (): Promise<RoleBrief[]> => get<RoleBrief[]>('/system/roles/options')

/** GET /system/roles/{id} —— 含权限码 */
export const getRole = (id: string): Promise<Role> => get<Role>(`/system/roles/${id}`)

export const createRole = (body: RoleCreate): Promise<Role> => post<Role, RoleCreate>('/system/roles', body)

/** 内置角色与自定义角色一样只能改名称 / 描述 */
export const updateRole = (id: string, body: RoleUpdate): Promise<Role> => put<Role, RoleUpdate>(`/system/roles/${id}`, body)

/** 内置角色或仍有用户持有时后端返回 409 */
export const deleteRole = (id: string): Promise<unknown> => del<unknown>(`/system/roles/${id}`)

export interface SetRolePermissionsBody {
  permissions: string[]
}

/** PUT /system/roles/{id}/permissions —— 整体替换动作码集合 */
export const setRolePermissions = (id: string, permissions: string[]): Promise<Role> =>
  put<Role, SetRolePermissionsBody>(`/system/roles/${id}/permissions`, { permissions })

/** GET /system/permissions/tree —— 菜单为父节点、动作为叶子；非平台租户不含平台专属权限 */
export const getPermissionTree = (): Promise<PermissionNode[]> => get<PermissionNode[]>('/system/permissions/tree')
