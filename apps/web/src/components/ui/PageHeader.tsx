import clsx from 'clsx'
import type { ReactNode } from 'react'

export interface PageHeaderProps {
  title: ReactNode
  description?: ReactNode
  /** 右侧操作区（按钮等） */
  extra?: ReactNode
  className?: string
}

export default function PageHeader({ title, description, extra, className }: PageHeaderProps) {
  return (
    <div className={clsx('flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between', className)}>
      <div className="min-w-0">
        <h2 className="text-lg font-semibold text-ink-strong tracking-[0.02em]">{title}</h2>
        {description && <p className="mt-0.5 text-sm text-ink-faint">{description}</p>}
      </div>
      {extra && <div className="flex flex-wrap items-center gap-2 shrink-0">{extra}</div>}
    </div>
  )
}
