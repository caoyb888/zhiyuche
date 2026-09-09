import { get } from './client'
import type { PermissionNode, RoleBrief } from './types'

export const roleKeys = {
  all: ['roles'] as const,
  options: ['roles', 'options'] as const,
  permissionTree: ['permissions', 'tree'] as const,
}

/** GET /system/roles/options —— 本租户全部角色简表（下拉用） */
export const listRoleOptions = (): Promise<RoleBrief[]> => get<RoleBrief[]>('/system/roles/options')

/** GET /system/permissions/tree */
export const getPermissionTree = (): Promise<PermissionNode[]> => get<PermissionNode[]>('/system/permissions/tree')
