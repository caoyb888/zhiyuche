import clsx from 'clsx'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'
import type { ReactNode } from 'react'
import Empty from './Empty'
import Spinner from './Spinner'
import { EMPTY } from '../../utils/format'

export type Align = 'left' | 'center' | 'right'

export interface Column<T> {
  /** 唯一键；sortable 时同时作为后端排序字段名 */
  key: string
  title: ReactNode
  /** 自定义单元格；缺省显示 row[dataIndex ?? key] */
  render?: (row: T, index: number) => ReactNode
  dataIndex?: keyof T
  width?: number | string
  align?: Align
  /** 点击表头排序，输出 `key` / `-key` */
  sortable?: boolean
  className?: string
}

export interface RowActions<T> {
  title?: ReactNode
  width?: number | string
  align?: Align
  render: (row: T, index: number) => ReactNode
}

export interface TableProps<T> {
  columns: Column<T>[]
  data: T[] | undefined
  /** 行唯一键：字段名或取值函数 */
  rowKey: keyof T | ((row: T) => string)
  loading?: boolean
  /** 空状态内容 */
  empty?: ReactNode
  /** 当前排序，`field` 或 `-field` */
  sort?: string
  onSortChange?: (sort: string | undefined) => void
  /** 可选的行操作列 */
  actions?: RowActions<T>
  skeletonRows?: number
  onRowClick?: (row: T) => void
  className?: string
}

const alignClass: Record<Align, string> = { left: 'text-left', center: 'text-center', right: 'text-right' }

function cellText(value: unknown): ReactNode {
  if (value === null || value === undefined || value === '') return EMPTY
  if (typeof value === 'string' || typeof value === 'number') return value
  if (typeof value === 'boolean') return value ? '是' : '否'
  return String(value)
}

/** 通用数据表：列配置、loading 骨架、空状态、行操作列、表头排序 */
export default function Table<T>({
  columns,
  data,
  rowKey,
  loading = false,
  empty,
  sort,
  onSortChange,
  actions,
  skeletonRows = 5,
  onRowClick,
  className,
}: TableProps<T>) {
  const rows = data ?? []
  const colCount = columns.length + (actions ? 1 : 0)
  const getKey = (row: T, index: number): string => {
    if (typeof rowKey === 'function') return rowKey(row)
    const v = row[rowKey]
    return typeof v === 'string' || typeof v === 'number' ? String(v) : String(index)
  }

  const sortField = sort?.startsWith('-') ? sort.slice(1) : sort
  const sortDesc = sort?.startsWith('-') ?? false

  const toggleSort = (key: string) => {
    if (!onSortChange) return
    if (sortField !== key) onSortChange(key)
    else if (!sortDesc) onSortChange(`-${key}`)
    else onSortChange(undefined)
  }

  const showSkeleton = loading && rows.length === 0
  const showEmpty = !loading && rows.length === 0

  return (
    <div className={clsx('relative overflow-x-auto', className)}>
      {loading && rows.length > 0 && (
        <div className="absolute right-2 top-1 z-10">
          <Spinner size="sm" />
        </div>
      )}
      <table className="data-table">
        <thead>
          <tr>
            {columns.map((col) => {
              const active = col.sortable && sortField === col.key
              return (
                <th
                  key={col.key}
                  style={{ width: col.width }}
                  className={clsx('pr-3 whitespace-nowrap', alignClass[col.align ?? 'left'], col.className)}
                  aria-sort={active ? (sortDesc ? 'descending' : 'ascending') : undefined}
                >
                  {col.sortable && onSortChange ? (
                    <button
                      type="button"
                      onClick={() => toggleSort(col.key)}
                      className={clsx('inline-flex items-center gap-1 uppercase tracking-wide hover:text-slate-700', active && 'text-brand-700')}
                    >
                      {col.title}
                      {active ? (sortDesc ? <ArrowDown size={12} /> : <ArrowUp size={12} />) : <ChevronsUpDown size={12} className="text-slate-300" />}
                    </button>
                  ) : (
                    col.title
                  )}
                </th>
              )
            })}
            {actions && (
              <th style={{ width: actions.width }} className={clsx('whitespace-nowrap', alignClass[actions.align ?? 'right'])}>
                {actions.title ?? '操作'}
              </th>
            )}
          </tr>
        </thead>
        <tbody className={clsx(loading && rows.length > 0 && 'opacity-60')}>
          {showSkeleton &&
            Array.from({ length: skeletonRows }).map((_, i) => (
              <tr key={`sk-${i}`}>
                {Array.from({ length: colCount }).map((_, j) => (
                  <td key={j} className="pr-3">
                    <div className="h-3.5 rounded bg-slate-100 animate-pulse" style={{ width: `${55 + ((i * 7 + j * 13) % 40)}%` }} />
                  </td>
                ))}
              </tr>
            ))}
          {showEmpty && (
            <tr>
              <td colSpan={colCount} className="!py-0">
                {empty ?? <Empty />}
              </td>
            </tr>
          )}
          {rows.map((row, index) => (
            <tr
              key={getKey(row, index)}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
              className={clsx(onRowClick && 'cursor-pointer')}
            >
              {columns.map((col) => (
                <td key={col.key} className={clsx('pr-3 align-middle', alignClass[col.align ?? 'left'], col.className)}>
                  {col.render ? col.render(row, index) : cellText(row[(col.dataIndex ?? col.key) as keyof T])}
                </td>
              ))}
              {actions && (
                <td className={clsx('align-middle whitespace-nowrap', alignClass[actions.align ?? 'right'])} onClick={(e) => e.stopPropagation()}>
                  {actions.render(row, index)}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
