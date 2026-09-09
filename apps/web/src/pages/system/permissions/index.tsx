import { KeyRound, ListTree } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { errorMessage } from '../../../api/client'
import type { PermissionNode } from '../../../api/types'
import { getMenuIcon } from '../../../components/icons'
import Badge from '../../../components/ui/Badge'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import FilterBar from '../../../components/ui/FilterBar'
import PageHeader from '../../../components/ui/PageHeader'
import Spinner from '../../../components/ui/Spinner'
import Tree from '../../../components/ui/Tree'
import type { TreeNode } from '../../../components/ui/tree-utils'
import { usePermissionTree } from '../../../hooks/usePermissionTree'

/** 关键词高亮（大小写不敏感） */
function highlight(source: string, kw: string): ReactNode {
  if (!kw) return source
  const lower = source.toLowerCase()
  const parts: ReactNode[] = []
  let i = 0
  let n = 0
  while (i < source.length) {
    const hit = lower.indexOf(kw, i)
    if (hit < 0) break
    if (hit > i) parts.push(source.slice(i, hit))
    parts.push(
      <mark key={n++} className="rounded bg-amber-100 px-0.5 text-amber-900">
        {source.slice(hit, hit + kw.length)}
      </mark>,
    )
    i = hit + kw.length
  }
  if (i < source.length) parts.push(source.slice(i))
  return <>{parts}</>
}

function matches(n: PermissionNode, kw: string): boolean {
  return n.name.toLowerCase().includes(kw) || n.code.toLowerCase().includes(kw) || (n.path ?? '').toLowerCase().includes(kw)
}

/** 名称 / 权限码 / 路由 任一命中即保留；命中的分组整枝保留，否则只保留有命中后代的祖先链 */
function filterPermissions(nodes: PermissionNode[], kw: string): PermissionNode[] {
  if (!kw) return nodes
  return nodes.flatMap((n) => {
    if (matches(n, kw)) return [n]
    const children = n.children ? filterPermissions(n.children, kw) : []
    return children.length > 0 ? [{ ...n, children }] : []
  })
}

function toNodes(nodes: PermissionNode[], kw: string): TreeNode[] {
  return nodes.map((n) => {
    const isMenu = n.type === 'menu'
    return {
      key: n.code,
      label: n.name,
      title: (
        <span className="inline-flex items-center gap-2">
          <span className={isMenu ? 'font-medium' : undefined}>{highlight(n.name, kw)}</span>
          {isMenu ? <Badge color="blue">菜单</Badge> : <Badge color="gray">动作</Badge>}
        </span>
      ),
      icon: isMenu ? getMenuIcon(n.icon) : KeyRound,
      extra: (
        <span className="inline-flex items-center gap-3 font-mono">
          {isMenu && n.path && <span className="text-slate-400">{highlight(n.path, kw)}</span>}
          <span className={isMenu ? 'text-slate-400' : 'text-slate-600'}>{highlight(n.code, kw)}</span>
        </span>
      ),
      children: n.children && n.children.length > 0 ? toNodes(n.children, kw) : undefined,
    }
  })
}

function countNodes(nodes: PermissionNode[]): { menus: number; actions: number } {
  let menus = 0
  let actions = 0
  const walk = (list: PermissionNode[]) => {
    for (const n of list) {
      if (n.type === 'menu') menus += 1
      else actions += 1
      if (n.children) walk(n.children)
    }
  }
  walk(nodes)
  return { menus, actions }
}

/** 菜单与权限：只读展示权限注册表（代码定义，不可在此编辑） */
export default function PermissionsPage() {
  const { query, tree } = usePermissionTree()
  const [draft, setDraft] = useState('')
  const [keyword, setKeyword] = useState('')

  const kw = keyword.trim().toLowerCase()
  const visible = useMemo(() => filterPermissions(tree, kw), [tree, kw])
  const nodes = useMemo(() => toNodes(visible, kw), [visible, kw])
  const total = useMemo(() => countNodes(tree), [tree])
  const shown = useMemo(() => countNodes(visible), [visible])

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="菜单与权限"
        description="权限点由代码注册表定义：菜单节点带路由与图标，动作节点为可分配给角色的权限码"
        extra={
          query.isSuccess && (
            <div className="flex items-center gap-2 text-xs text-slate-500">
              <Badge color="blue">{total.menus} 个菜单</Badge>
              <Badge color="gray">{total.actions} 个动作</Badge>
            </div>
          )
        }
      />

      <FilterBar
        keyword={draft}
        onKeywordChange={(v) => {
          setDraft(v)
          setKeyword(v)
        }}
        keywordPlaceholder="名称 / 权限码 / 路由"
        onSearch={() => setKeyword(draft)}
        onReset={() => {
          setDraft('')
          setKeyword('')
        }}
      />

      <div className="card p-4">
        {query.isPending ? (
          <div className="flex justify-center py-12">
            <Spinner label="加载权限树" />
          </div>
        ) : query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => void query.refetch()} />
        ) : tree.length === 0 ? (
          <Empty icon={ListTree} title="暂无权限点" />
        ) : nodes.length === 0 ? (
          <Empty title="没有匹配的权限点" description="换个关键词试试" />
        ) : (
          <>
            {kw && (
              <div className="mb-2 px-1 text-xs text-slate-400">
                匹配 {shown.menus} 个菜单、{shown.actions} 个动作
              </div>
            )}
            <Tree key={kw} nodes={nodes} />
          </>
        )}
      </div>
    </div>
  )
}
