import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react'
import clsx from 'clsx'
import type { LucideIcon } from 'lucide-react'
import { controlClass } from './styles'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  /** 校验失败态（红框） */
  invalid?: boolean
  /** 左侧图标 */
  icon?: LucideIcon
  /** 右侧附加内容（如清除按钮） */
  suffix?: ReactNode
}

const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { invalid, icon: Icon, suffix, className, ...rest },
  ref,
) {
  const input = (
    <input
      ref={ref}
      aria-invalid={invalid || undefined}
      className={controlClass(invalid, clsx('h-9 px-3', Icon && 'pl-9', suffix && 'pr-9', className))}
      {...rest}
    />
  )
  if (!Icon && !suffix) return input
  return (
    <div className="relative w-full">
      {Icon && (
        <Icon size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-faint pointer-events-none" />
      )}
      {input}
      {suffix && <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center">{suffix}</div>}
    </div>
  )
})

export default Input
