import clsx from 'clsx'
import { RotateCcw, Search } from 'lucide-react'
import type { FormEvent, ReactNode } from 'react'
import Button from './Button'
import Input from './Input'

export interface FilterBarProps {
  /** 关键词（受控） */
  keyword?: string
  onKeywordChange?: (value: string) => void
  keywordPlaceholder?: string
  /** 点击"查询"或在关键词框回车 */
  onSearch: () => void
  onReset: () => void
  /** 其余筛选控件 */
  children?: ReactNode
  /** 右侧附加操作 */
  extra?: ReactNode
  loading?: boolean
  className?: string
}

/** 列表页筛选条：搜索框 + 若干筛选 + 查询/重置 */
export default function FilterBar({
  keyword,
  onKeywordChange,
  keywordPlaceholder = '搜索',
  onSearch,
  onReset,
  children,
  extra,
  loading,
  className,
}: FilterBarProps) {
  const submit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    onSearch()
  }
  return (
    <form onSubmit={submit} className={clsx('card p-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between', className)}>
      <div className="flex flex-wrap items-center gap-2 flex-1 min-w-0">
        {onKeywordChange && (
          <div className="w-full sm:w-56">
            <Input
              icon={Search}
              value={keyword ?? ''}
              onChange={(e) => onKeywordChange(e.target.value)}
              placeholder={keywordPlaceholder}
              aria-label="关键词"
            />
          </div>
        )}
        {children}
        <div className="flex items-center gap-2">
          <Button type="submit" size="md" loading={loading} icon={Search}>
            查询
          </Button>
          <Button type="button" variant="secondary" icon={RotateCcw} onClick={onReset}>
            重置
          </Button>
        </div>
      </div>
      {extra && <div className="flex flex-wrap items-center gap-2 shrink-0">{extra}</div>}
    </form>
  )
}
