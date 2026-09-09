import { useCallback } from 'react'
import { useAuthStore } from '../store/auth'
import { hasPermission, type PermissionInput } from '../auth/permission'

export interface PermissionApi {
  /** 是否拥有权限码（数组时任一命中） */
  can: (perm: PermissionInput) => boolean
  /** 数组全部命中 */
  canAll: (perms: readonly string[]) => boolean
  isSuper: boolean
}

/** 按钮级权限控制钩子；超级管理员视为全部拥有 */
export function usePermission(): PermissionApi {
  const profile = useAuthStore((s) => s.profile)
  const can = useCallback((perm: PermissionInput) => hasPermission(profile, perm, 'any'), [profile])
  const canAll = useCallback((perms: readonly string[]) => hasPermission(profile, perms, 'all'), [profile])
  return { can, canAll, isSuper: profile?.is_super ?? false }
}
