import clsx from 'clsx'
import type { ReactNode } from 'react'
import { EMPTY } from '../../utils/format'

export interface DescriptionItem {
  label: ReactNode
  value: ReactNode
  /** 占用列数（如备注跨整行） */
  span?: 1 | 2 | 3
}

export interface DescriptionListProps {
  items: DescriptionItem[]
  columns?: 1 | 2 | 3
  className?: string
}

const colClass: Record<NonNullable<DescriptionListProps['columns']>, string> = {
  1: 'grid-cols-1',
  2: 'sm:grid-cols-2',
  3: 'sm:grid-cols-3',
}

const spanClass: Record<NonNullable<DescriptionItem['span']>, string> = {
  1: '',
  2: 'sm:col-span-2',
  3: 'sm:col-span-3',
}

/** 详情键值对展示 */
export default function DescriptionList({ items, columns = 2, className }: DescriptionListProps) {
  return (
    <dl className={clsx('grid grid-cols-1 gap-x-6 gap-y-4', colClass[columns], className)}>
      {items.map((it, i) => (
        <div key={i} className={clsx('min-w-0', it.span && spanClass[it.span])}>
          <dt className="text-xs text-slate-400 mb-1">{it.label}</dt>
          <dd className="text-sm text-slate-800 break-words">
            {it.value === null || it.value === undefined || it.value === '' ? EMPTY : it.value}
          </dd>
        </div>
      ))}
    </dl>
  )
}
