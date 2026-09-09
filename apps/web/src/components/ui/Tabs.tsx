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
    <div role="tablist" className={clsx('flex gap-1 border-b border-slate-100 overflow-x-auto', className)}>
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
              active ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500 hover:text-slate-800',
              it.disabled && 'opacity-50 cursor-not-allowed',
            )}
          >
            {it.label}
            {it.count !== undefined && (
              <span className={clsx('rounded-full px-1.5 text-[10px] leading-4', active ? 'bg-brand-50 text-brand-700' : 'bg-slate-100 text-slate-500')}>
                {it.count}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
