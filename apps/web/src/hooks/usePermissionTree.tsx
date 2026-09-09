import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { KeyRound } from 'lucide-react'
import { useMemo } from 'react'
import { getPermissionTree, roleKeys } from '../api/roles'
import type { PermissionNode } from '../api/types'
import { getMenuIcon } from '../components/icons'
import type { TreeNode } from '../components/ui/tree-utils'

/**
 * 权限树 → 通用 TreeNode：菜单为分组节点（带图标），动作为叶子（extra 显示权限码）。
 * 没有子节点的菜单不可勾选（角色只能持有动作码）。
 */
export function permissionsToTreeNodes(nodes: PermissionNode[]): TreeNode[] {
  return nodes.map((n) => {
    const children = n.children && n.children.length > 0 ? permissionsToTreeNodes(n.children) : undefined
    const isMenu = n.type === 'menu'
    return {
      key: n.code,
      label: n.name,
      icon: isMenu ? getMenuIcon(n.icon) : KeyRound,
      extra: isMenu ? undefined : <code className="font-mono">{n.code}</code>,
      children,
      disabled: isMenu && !children,
    }
  })
}

/** 深度优先收集全部动作码 */
export function collectActionCodes(nodes: PermissionNode[], into: string[] = []): string[] {
  for (const n of nodes) {
    if (n.type === 'action') into.push(n.code)
    if (n.children) collectActionCodes(n.children, into)
  }
  return into
}

export interface PermissionTreeResult {
  query: UseQueryResult<PermissionNode[]>
  tree: PermissionNode[]
  nodes: TreeNode[]
  /** 全部动作码（用于"全选"） */
  actionCodes: string[]
}

/** GET /system/permissions/tree（注册表是静态的，缓存较久） */
export function usePermissionTree(enabled = true): PermissionTreeResult {
  const query = useQuery({
    queryKey: roleKeys.permissionTree,
    queryFn: getPermissionTree,
    enabled,
    staleTime: 10 * 60_000,
  })
  const tree = useMemo(() => query.data ?? [], [query.data])
  const nodes = useMemo(() => permissionsToTreeNodes(tree), [tree])
  const actionCodes = useMemo(() => collectActionCodes(tree), [tree])
  return { query, tree, nodes, actionCodes }
}
