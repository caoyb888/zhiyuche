import clsx from 'clsx'
import { ChevronLeft, ChevronRight } from 'lucide-react'

export interface PaginationProps {
  page: number
  pageSize: number
  total: number
  onChange: (page: number, pageSize: number) => void
  pageSizeOptions?: number[]
  className?: string
}

const DEFAULT_SIZES = [10, 20, 50, 100]

/** 生成页码序列，超过 7 页时用 '…' 折叠 */
function pageItems(current: number, pages: number): Array<number | 'gap'> {
  if (pages <= 7) return Array.from({ length: pages }, (_, i) => i + 1)
  const items: Array<number | 'gap'> = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(pages - 1, current + 1)
  if (start > 2) items.push('gap')
  for (let p = start; p <= end; p++) items.push(p)
  if (end < pages - 1) items.push('gap')
  items.push(pages)
  return items
}

/** 页码 + 每页条数 */
export default function Pagination({ page, pageSize, total, onChange, pageSizeOptions = DEFAULT_SIZES, className }: PaginationProps) {
  const pages = Math.max(1, Math.ceil(total / Math.max(1, pageSize)))
  const current = Math.min(Math.max(1, page), pages)
  const from = total === 0 ? 0 : (current - 1) * pageSize + 1
  const to = Math.min(total, current * pageSize)

  const go = (p: number) => {
    if (p < 1 || p > pages || p === current) return
    onChange(p, pageSize)
  }

  const btn = 'inline-flex h-8 min-w-[2rem] items-center justify-center rounded-md px-2 text-sm transition-colors disabled:opacity-40 disabled:cursor-not-allowed'

  return (
    <div className={clsx('flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between', className)}>
      <div className="text-xs text-slate-400">
        共 <span className="text-slate-600 font-medium">{total}</span> 条{total > 0 && <>，当前 {from}–{to}</>}
      </div>
      <div className="flex items-center gap-3">
        <select
          value={pageSize}
          onChange={(e) => onChange(1, Number(e.target.value))}
          className="h-8 rounded-md border border-slate-200 bg-white px-2 text-xs text-slate-600 focus:outline-none focus:ring-2 focus:ring-brand-100"
          aria-label="每页条数"
        >
          {pageSizeOptions.map((n) => (
            <option key={n} value={n}>
              {n} 条/页
            </option>
          ))}
        </select>
        <nav className="flex items-center gap-1" aria-label="分页">
          <button type="button" className={clsx(btn, 'text-slate-500 hover:bg-slate-100')} disabled={current <= 1} onClick={() => go(current - 1)} aria-label="上一页">
            <ChevronLeft size={16} />
          </button>
          {pageItems(current, pages).map((it, i) =>
            it === 'gap' ? (
              <span key={`gap-${i}`} className="px-1 text-slate-400">
                …
              </span>
            ) : (
              <button
                key={it}
                type="button"
                aria-current={it === current ? 'page' : undefined}
                onClick={() => go(it)}
                className={clsx(btn, it === current ? 'bg-brand-600 text-white' : 'text-slate-600 hover:bg-slate-100')}
              >
                {it}
              </button>
            ),
          )}
          <button type="button" className={clsx(btn, 'text-slate-500 hover:bg-slate-100')} disabled={current >= pages} onClick={() => go(current + 1)} aria-label="下一页">
            <ChevronRight size={16} />
          </button>
        </nav>
      </div>
    </div>
  )
}
