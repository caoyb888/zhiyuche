import clsx from 'clsx'
import { ChevronRight } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { allLeafKeys, collectLeafKeys, type TreeNode } from './tree-utils'

export interface CheckTreeProps {
  nodes: TreeNode[]
  /** 已勾选的叶子键（父节点状态由子节点推导，不在 value 中） */
  value: readonly string[]
  onChange: (leafKeys: string[]) => void
  /** 缺省展开全部（数据异步到达时同样生效） */
  defaultExpandAll?: boolean
  disabled?: boolean
  size?: 'sm' | 'md'
  className?: string
  emptyText?: ReactNode
}

type CheckState = 'checked' | 'indeterminate' | 'none'

function IndeterminateCheckbox({ state, disabled, onToggle, label }: { state: CheckState; disabled?: boolean; onToggle: () => void; label: string }) {
  const ref = useRef<HTMLInputElement>(null)
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = state === 'indeterminate'
  }, [state])
  return (
    <input
      ref={ref}
      type="checkbox"
      aria-label={label}
      checked={state === 'checked'}
      disabled={disabled}
      onChange={onToggle}
      onClick={(e) => e.stopPropagation()}
      className="h-4 w-4 shrink-0 rounded border-line-strong bg-surface-3 text-brand-600 focus:ring-brand-600/30 accent-brand-600 cursor-pointer disabled:cursor-not-allowed"
    />
  )
}

interface ItemProps {
  node: TreeNode
  depth: number
  expanded: (key: string) => boolean
  toggleExpand: (key: string) => void
  checked: ReadonlySet<string>
  toggleCheck: (node: TreeNode) => void
  disabled?: boolean
  size: 'sm' | 'md'
}

function stateOf(node: TreeNode, checked: ReadonlySet<string>): CheckState {
  const leaves = collectLeafKeys(node)
  if (leaves.length === 0) return checked.has(node.key) ? 'checked' : 'none'
  let n = 0
  for (const k of leaves) if (checked.has(k)) n += 1
  if (n === 0) return 'none'
  return n === leaves.length ? 'checked' : 'indeterminate'
}

function CheckTreeItem({ node, depth, expanded, toggleExpand, checked, toggleCheck, disabled, size }: ItemProps) {
  const hasChildren = (node.children?.length ?? 0) > 0
  const open = hasChildren && expanded(node.key)
  const itemDisabled = disabled || node.disabled
  const state = stateOf(node, checked)
  const Icon = node.icon

  return (
    <li>
      <div
        role="treeitem"
        aria-expanded={hasChildren ? open : undefined}
        aria-checked={state === 'indeterminate' ? 'mixed' : state === 'checked'}
        onClick={() => {
          if (!itemDisabled) toggleCheck(node)
        }}
        style={{ paddingLeft: depth * 16 + 6 }}
        className={clsx(
          'flex items-center gap-1.5 rounded-md pr-2 transition-colors',
          size === 'sm' ? 'py-1 text-xs' : 'py-1.5 text-sm',
          itemDisabled ? 'cursor-not-allowed text-ink-disabled' : 'cursor-pointer text-ink hover:bg-surface-3',
        )}
      >
        <button
          type="button"
          tabIndex={-1}
          aria-label={open ? '收起' : '展开'}
          onClick={(e) => {
            e.stopPropagation()
            if (hasChildren) toggleExpand(node.key)
          }}
          className={clsx(
            'flex h-5 w-5 shrink-0 items-center justify-center rounded text-ink-faint',
            hasChildren ? 'hover:bg-surface-4 hover:text-ink-strong' : 'invisible',
          )}
        >
          <ChevronRight size={14} className={clsx('transition-transform', open && 'rotate-90')} />
        </button>
        <IndeterminateCheckbox state={state} disabled={itemDisabled} onToggle={() => toggleCheck(node)} label={node.label} />
        {Icon && <Icon size={14} className="shrink-0 text-ink-faint" />}
        <span className={clsx('truncate flex-1', hasChildren && 'font-medium')}>{node.title ?? node.label}</span>
        {node.extra !== undefined && <span className="shrink-0 text-xs text-ink-faint">{node.extra}</span>}
      </div>
      {open && node.children && (
        <ul role="group">
          {node.children.map((c) => (
            <CheckTreeItem
              key={c.key}
              node={c}
              depth={depth + 1}
              expanded={expanded}
              toggleExpand={toggleExpand}
              checked={checked}
              toggleCheck={toggleCheck}
              disabled={disabled}
              size={size}
            />
          ))}
        </ul>
      )}
    </li>
  )
}

/**
 * 可勾选的树（受控）：value 只保存叶子键；
 * 勾选父节点 = 全选其下可用叶子，部分勾选显示半选态。
 * 展开状态的保存方式与 <Tree> 相同（记录"被切换过"的键）。
 */
export default function CheckTree({ nodes, value, onChange, defaultExpandAll = true, disabled, size = 'md', className, emptyText = '暂无数据' }: CheckTreeProps) {
  const [toggled, setToggled] = useState<ReadonlySet<string>>(() => new Set())
  const checked = useMemo(() => new Set(value), [value])
  // 叶子键的稳定顺序（深度优先），保证 onChange 输出顺序与树一致
  const order = useMemo(() => allLeafKeys(nodes), [nodes])

  const expanded = (key: string) => (defaultExpandAll ? !toggled.has(key) : toggled.has(key))
  const toggleExpand = (key: string) =>
    setToggled((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })

  const toggleCheck = (node: TreeNode) => {
    const leaves = collectLeafKeys(node)
    if (leaves.length === 0) return
    const next = new Set(checked)
    const allOn = leaves.every((k) => next.has(k))
    for (const k of leaves) {
      if (allOn) next.delete(k)
      else next.add(k)
    }
    // 保留树之外的键（如后端返回但当前树未包含的权限码），避免保存时静默丢失
    const known = new Set(order)
    const extra = [...next].filter((k) => !known.has(k))
    onChange([...order.filter((k) => next.has(k)), ...extra])
  }

  if (nodes.length === 0) {
    return <div className={clsx('py-4 text-center text-xs text-ink-faint', className)}>{emptyText}</div>
  }

  return (
    <ul role="tree" aria-multiselectable className={clsx('select-none', className)}>
      {nodes.map((n) => (
        <CheckTreeItem
          key={n.key}
          node={n}
          depth={0}
          expanded={expanded}
          toggleExpand={toggleExpand}
          checked={checked}
          toggleCheck={toggleCheck}
          disabled={disabled}
          size={size}
        />
      ))}
    </ul>
  )
}
