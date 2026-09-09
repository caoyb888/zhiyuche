import clsx from 'clsx'
import { ChevronRight } from 'lucide-react'
import { useState } from 'react'
import type { TreeNode } from './tree-utils'

export type { TreeNode } from './tree-utils'

export interface TreeProps {
  nodes: TreeNode[]
  selectedKey?: string | null
  onSelect?: (key: string, node: TreeNode) => void
  /** 缺省展开全部（数据异步到达时同样生效） */
  defaultExpandAll?: boolean
  /** 额外禁用的节点键（如移动部门时排除自身子树） */
  disabledKeys?: ReadonlySet<string>
  className?: string
  size?: 'sm' | 'md'
}

interface ItemProps {
  node: TreeNode
  depth: number
  expanded: (key: string) => boolean
  toggle: (key: string) => void
  selectedKey: string | null | undefined
  onSelect?: (key: string, node: TreeNode) => void
  disabledKeys?: ReadonlySet<string>
  size: 'sm' | 'md'
}

function TreeItem({ node, depth, expanded, toggle, selectedKey, onSelect, disabledKeys, size }: ItemProps) {
  const hasChildren = (node.children?.length ?? 0) > 0
  const open = hasChildren && expanded(node.key)
  const disabled = node.disabled || disabledKeys?.has(node.key)
  const selected = selectedKey === node.key
  const Icon = node.icon

  return (
    <li>
      <div
        role="treeitem"
        aria-selected={selected}
        aria-expanded={hasChildren ? open : undefined}
        onClick={() => {
          if (disabled) return
          onSelect?.(node.key, node)
        }}
        style={{ paddingLeft: depth * 16 + 6 }}
        className={clsx(
          'group flex items-center gap-1 rounded-md pr-2 transition-colors',
          size === 'sm' ? 'py-1 text-xs' : 'py-1.5 text-sm',
          disabled ? 'cursor-not-allowed text-slate-300' : 'cursor-pointer',
          !disabled && (selected ? 'bg-brand-50 text-brand-700 font-medium' : 'text-slate-700 hover:bg-slate-50'),
        )}
      >
        <button
          type="button"
          tabIndex={-1}
          aria-label={open ? '收起' : '展开'}
          onClick={(e) => {
            e.stopPropagation()
            if (hasChildren) toggle(node.key)
          }}
          className={clsx(
            'flex h-5 w-5 shrink-0 items-center justify-center rounded text-slate-400',
            hasChildren ? 'hover:bg-slate-200/70 hover:text-slate-600' : 'invisible',
          )}
        >
          <ChevronRight size={14} className={clsx('transition-transform', open && 'rotate-90')} />
        </button>
        {Icon && <Icon size={14} className="shrink-0 text-slate-400" />}
        <span className="truncate flex-1">{node.label}</span>
        {node.extra !== undefined && <span className="shrink-0 text-xs text-slate-400">{node.extra}</span>}
      </div>
      {open && node.children && (
        <ul role="group">
          {node.children.map((c) => (
            <TreeItem
              key={c.key}
              node={c}
              depth={depth + 1}
              expanded={expanded}
              toggle={toggle}
              selectedKey={selectedKey}
              onSelect={onSelect}
              disabledKeys={disabledKeys}
              size={size}
            />
          ))}
        </ul>
      )}
    </li>
  )
}

/**
 * 可展开、可选中的树。
 * 展开状态以"被切换过的键"保存：defaultExpandAll 时该集合表示已折叠，否则表示已展开，
 * 因此数据异步加载后无需同步 state 也能得到正确的缺省展开。
 */
export default function Tree({ nodes, selectedKey, onSelect, defaultExpandAll = true, disabledKeys, className, size = 'md' }: TreeProps) {
  const [toggled, setToggled] = useState<ReadonlySet<string>>(() => new Set())

  const expanded = (key: string) => (defaultExpandAll ? !toggled.has(key) : toggled.has(key))
  const toggle = (key: string) =>
    setToggled((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })

  return (
    <ul role="tree" className={clsx('select-none', className)}>
      {nodes.map((n) => (
        <TreeItem
          key={n.key}
          node={n}
          depth={0}
          expanded={expanded}
          toggle={toggle}
          selectedKey={selectedKey}
          onSelect={onSelect}
          disabledKeys={disabledKeys}
          size={size}
        />
      ))}
    </ul>
  )
}
