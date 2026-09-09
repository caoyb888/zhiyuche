import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

export interface TreeNode {
  key: string
  /** 纯文本标题（用于显示与搜索） */
  label: string
  /** 可选的富文本标题（如关键词高亮），有则代替 label 显示 */
  title?: ReactNode
  children?: TreeNode[]
  /** 右侧附加内容（如人数） */
  extra?: ReactNode
  icon?: LucideIcon
  disabled?: boolean
}

/** 遍历树查找节点 */
export function findTreeNode(nodes: TreeNode[], key: string): TreeNode | undefined {
  for (const n of nodes) {
    if (n.key === key) return n
    if (n.children) {
      const hit = findTreeNode(n.children, key)
      if (hit) return hit
    }
  }
  return undefined
}

/** 收集某节点及其全部后代的键 */
export function collectTreeKeys(node: TreeNode, into: Set<string> = new Set()): Set<string> {
  into.add(node.key)
  node.children?.forEach((c) => collectTreeKeys(c, into))
  return into
}

/** 按关键字过滤：命中节点整枝保留，未命中但有命中后代的节点保留祖先链 */
export function filterTree(nodes: TreeNode[], keyword: string): TreeNode[] {
  const kw = keyword.trim().toLowerCase()
  if (!kw) return nodes
  const walk = (list: TreeNode[]): TreeNode[] =>
    list.flatMap((n) => {
      if (n.label.toLowerCase().includes(kw)) return [n]
      const children = n.children ? walk(n.children) : []
      return children.length > 0 ? [{ ...n, children }] : []
    })
  return walk(nodes)
}

/** 收集节点下全部可选叶子键（禁用叶子不参与"勾父全选子"） */
export function collectLeafKeys(node: TreeNode, into: string[] = []): string[] {
  const children = node.children ?? []
  if (children.length === 0) {
    if (!node.disabled) into.push(node.key)
    return into
  }
  for (const c of children) collectLeafKeys(c, into)
  return into
}

/** 整棵树的可选叶子键（深度优先） */
export function allLeafKeys(nodes: TreeNode[]): string[] {
  const out: string[] = []
  for (const n of nodes) collectLeafKeys(n, out)
  return out
}

/** 深度优先展平 */
export function flattenTree(nodes: TreeNode[], into: TreeNode[] = []): TreeNode[] {
  for (const n of nodes) {
    into.push(n)
    if (n.children) flattenTree(n.children, into)
  }
  return into
}
