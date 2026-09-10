import clsx from 'clsx'
import type { ReactNode } from 'react'

export interface TabItem<K extends string = string> {
  key: K
  label: ReactNode
  /** 右侧计数 */
  count?: number
  disabled?: boolean
}

export interface TabsProps<K extends string = string> {
  items: TabItem<K>[]
  value: K
  onChange: (key: K) => void
  className?: string
  size?: 'sm' | 'md'
}

/** 下划线风格页签（受控） */
export default function Tabs<K extends string = string>({ items, value, onChange, className, size = 'md' }: TabsProps<K>) {
  return (
    <div role="tablist" className={clsx('flex gap-1 border-b border-line overflow-x-auto', className)}>
      {items.map((it) => {
        const active = it.key === value
        return (
          <button
            key={it.key}
            type="button"
            role="tab"
            aria-selected={active}
            disabled={it.disabled}
            onClick={() => onChange(it.key)}
            className={clsx(
              'relative -mb-px inline-flex items-center gap-1.5 whitespace-nowrap border-b-2 font-medium transition-colors',
              size === 'sm' ? 'px-3 py-2 text-xs' : 'px-4 py-2.5 text-sm',
              active ? 'border-brand-500 text-brand-300' : 'border-transparent text-ink-muted hover:text-ink-strong',
              it.disabled && 'opacity-50 cursor-not-allowed',
            )}
          >
            {it.label}
            {it.count !== undefined && (
              <span className={clsx('font-mono rounded px-1.5 text-[10px] leading-4', active ? 'bg-brand-600/20 text-brand-200' : 'bg-white/[0.06] text-ink-muted')}>
                {it.count}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
