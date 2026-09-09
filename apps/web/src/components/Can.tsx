import type { ReactNode } from 'react'
import { usePermission } from '../hooks/usePermission'
import type { PermissionInput } from '../auth/permission'

export interface CanProps {
  /** 所需权限码；数组时任一命中即可，mode='all' 要求全部 */
  perm: PermissionInput
  mode?: 'any' | 'all'
  /** 无权限时渲染的内容，缺省不渲染 */
  fallback?: ReactNode
  children: ReactNode
}

/** 按权限码控制子元素显示：`<Can perm="system:user:create"><Button/></Can>` */
export default function Can({ perm, mode = 'any', fallback = null, children }: CanProps) {
  const { can, canAll } = usePermission()
  const allowed = mode === 'all' ? canAll(typeof perm === 'string' ? [perm] : perm) : can(perm)
  return <>{allowed ? children : fallback}</>
}
