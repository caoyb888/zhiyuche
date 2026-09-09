import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { useMemo } from 'react'
import { deptKeys, getDeptTree } from '../api/depts'
import type { DeptNode } from '../api/types'
import type { TreeNode } from '../components/ui/tree-utils'

/** 部门树 → 通用 TreeNode（extra 显示直属人数） */
export function deptsToTreeNodes(depts: DeptNode[]): TreeNode[] {
  return depts.map((d) => ({
    key: d.id,
    label: d.name,
    extra: `${d.user_count} 人`,
    children: d.children && d.children.length > 0 ? deptsToTreeNodes(d.children) : undefined,
  }))
}

/** 在部门树中按 id 查找 */
export function findDept(depts: DeptNode[], id: string): DeptNode | undefined {
  for (const d of depts) {
    if (d.id === id) return d
    if (d.children) {
      const hit = findDept(d.children, id)
      if (hit) return hit
    }
  }
  return undefined
}

/** 收集节点及其全部后代 id */
export function collectDeptIds(dept: DeptNode, into: Set<string> = new Set()): Set<string> {
  into.add(dept.id)
  dept.children?.forEach((c) => collectDeptIds(c, into))
  return into
}

export interface DeptTreeResult {
  query: UseQueryResult<DeptNode[]>
  depts: DeptNode[]
  nodes: TreeNode[]
}

/** 部门树查询（后端未就绪返回 404 时 depts/nodes 为空，query.isError 为 true） */
export function useDeptTree(enabled = true): DeptTreeResult {
  const query = useQuery({
    queryKey: deptKeys.tree,
    queryFn: getDeptTree,
    enabled,
    staleTime: 60_000,
  })
  const depts = useMemo(() => query.data ?? [], [query.data])
  const nodes = useMemo(() => deptsToTreeNodes(depts), [depts])
  return { query, depts, nodes }
}
