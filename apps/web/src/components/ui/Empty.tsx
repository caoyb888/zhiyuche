import clsx from 'clsx'
import { Inbox, type LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

export interface EmptyProps {
  icon?: LucideIcon
  title?: ReactNode
  description?: ReactNode
  /** 操作按钮等 */
  action?: ReactNode
  className?: string
  size?: 'sm' | 'md'
}

export default function Empty({ icon: Icon = Inbox, title = '暂无数据', description, action, className, size = 'md' }: EmptyProps) {
  return (
    <div className={clsx('flex flex-col items-center justify-center text-center', size === 'sm' ? 'py-6' : 'py-12', className)}>
      <div className={clsx('rounded-full bg-slate-100 text-slate-400 flex items-center justify-center', size === 'sm' ? 'w-10 h-10' : 'w-14 h-14')}>
        <Icon size={size === 'sm' ? 20 : 26} />
      </div>
      <div className="mt-3 text-sm font-medium text-slate-600">{title}</div>
      {description && <div className="mt-1 text-xs text-slate-400 max-w-xs">{description}</div>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}
