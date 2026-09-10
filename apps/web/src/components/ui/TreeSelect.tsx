import clsx from 'clsx'
import { ChevronDown, Search, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import Spinner from './Spinner'
import Tree from './Tree'
import { controlClass } from './styles'
import { filterTree, findTreeNode, type TreeNode } from './tree-utils'

export interface TreeSelectProps {
  nodes: TreeNode[]
  value: string | null | undefined
  onChange: (key: string | null) => void
  placeholder?: string
  disabled?: boolean
  /** 不可选的节点键（仍显示，用于排除自身子树等） */
  disabledKeys?: ReadonlySet<string>
  /** 允许清空，缺省 true */
  allowClear?: boolean
  invalid?: boolean
  loading?: boolean
  emptyText?: string
  /** 显示搜索框，缺省 true */
  searchable?: boolean
  id?: string
  className?: string
}

/** 下拉里选树节点（用于选部门） */
export default function TreeSelect({
  nodes,
  value,
  onChange,
  placeholder = '请选择',
  disabled,
  disabledKeys,
  allowClear = true,
  invalid,
  loading,
  emptyText = '暂无数据',
  searchable = true,
  id,
  className,
}: TreeSelectProps) {
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const rootRef = useRef<HTMLDivElement>(null)

  const selected = useMemo(() => (value ? findTreeNode(nodes, value) : undefined), [nodes, value])
  const visible = useMemo(() => filterTree(nodes, keyword), [nodes, keyword])

  const close = useCallback(() => {
    setOpen(false)
    setKeyword('')
  }, [])

  // 点击外部 / ESC 关闭
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) close()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        close()
      }
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey, true)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey, true)
    }
  }, [open, close])

  return (
    <div ref={rootRef} className={clsx('relative w-full', className)}>
      <button
        type="button"
        id={id}
        disabled={disabled}
        aria-haspopup="tree"
        aria-expanded={open}
        aria-invalid={invalid || undefined}
        onClick={() => (open ? close() : setOpen(true))}
        className={controlClass(invalid, 'h-9 pl-3 pr-14 text-left flex items-center')}
      >
        <span className={clsx('truncate', !selected && 'text-ink-disabled')}>
          {selected ? selected.label : value && !loading ? '（未知节点）' : placeholder}
        </span>
      </button>
      <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-0.5">
        {allowClear && value && !disabled && (
          <button
            type="button"
            aria-label="清空"
            onMouseDown={(e) => e.preventDefault()}
            onClick={(e) => {
              e.stopPropagation()
              onChange(null)
            }}
            className="rounded p-0.5 text-ink-faint hover:text-ink-strong"
          >
            <X size={14} />
          </button>
        )}
        <ChevronDown size={16} className={clsx('text-ink-faint pointer-events-none transition-transform', open && 'rotate-180')} />
      </div>

      {open && (
        <div className="absolute z-40 mt-1 w-full min-w-[16rem] rounded-card border border-line bg-surface-2 shadow-float">
          {searchable && (
            <div className="relative border-b border-line-soft p-2">
              <Search size={14} className="absolute left-4 top-1/2 -translate-y-1/2 text-ink-faint" />
              <input
                autoFocus
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder="搜索"
                className="h-8 w-full rounded-md border border-line-strong bg-surface-3 text-ink placeholder:text-ink-disabled pl-7 pr-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-600/25"
              />
            </div>
          )}
          <div className="max-h-64 overflow-y-auto p-1.5">
            {loading ? (
              <div className="flex justify-center py-4">
                <Spinner size="sm" label="加载中" />
              </div>
            ) : visible.length === 0 ? (
              <div className="py-4 text-center text-xs text-ink-faint">{keyword ? '无匹配结果' : emptyText}</div>
            ) : (
              <Tree
                nodes={visible}
                selectedKey={value ?? null}
                disabledKeys={disabledKeys}
                size="sm"
                onSelect={(key) => {
                  onChange(key)
                  close()
                }}
              />
            )}
          </div>
        </div>
      )}
    </div>
  )
}
