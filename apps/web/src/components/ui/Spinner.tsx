import clsx from 'clsx'
import { Loader2 } from 'lucide-react'

export interface SpinnerProps {
  size?: 'sm' | 'md' | 'lg'
  className?: string
  /** 文字说明，显示在图标右侧 */
  label?: string
}

const sizes = { sm: 14, md: 20, lg: 32 } as const

export default function Spinner({ size = 'md', className, label }: SpinnerProps) {
  return (
    <span role="status" aria-live="polite" className={clsx('inline-flex items-center gap-2 text-slate-400', className)}>
      <Loader2 size={sizes[size]} className="animate-spin" />
      {label && <span className="text-sm">{label}</span>}
    </span>
  )
}
