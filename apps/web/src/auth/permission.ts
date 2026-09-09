import type { Profile } from '../api/types'

export type PermissionInput = string | readonly string[]
export type PermissionMode = 'any' | 'all'

/**
 * 判断用户是否拥有权限码。
 * - 超级管理员（is_super）视为拥有全部权限
 * - 传数组时缺省任一命中即可（mode='any'），mode='all' 要求全部命中
 * - 空权限要求（'' 或 []）视为无需权限
 */
export function hasPermission(profile: Profile | null | undefined, perm: PermissionInput, mode: PermissionMode = 'any'): boolean {
  const codes = typeof perm === 'string' ? (perm ? [perm] : []) : perm
  if (codes.length === 0) return true
  if (!profile) return false
  if (profile.is_super) return true
  const owned = new Set(profile.permissions)
  return mode === 'all' ? codes.every((c) => owned.has(c)) : codes.some((c) => owned.has(c))
}
