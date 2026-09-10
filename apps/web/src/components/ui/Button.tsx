import { forwardRef, type ButtonHTMLAttributes } from 'react'
import clsx from 'clsx'
import { Loader2, type LucideIcon } from 'lucide-react'

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost'
export type ButtonSize = 'sm' | 'md' | 'lg'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  /** 加载中：禁用并显示转圈 */
  loading?: boolean
  /** 左侧图标 */
  icon?: LucideIcon
  /** 占满一行 */
  block?: boolean
}

const variantClass: Record<ButtonVariant, string> = {
  primary: 'bg-brand-600 text-white hover:bg-brand-500 focus-visible:ring-brand-600/40 shadow-[0_8px_18px_-10px_rgba(29,111,216,.95)]',
  secondary: 'bg-surface-3 text-ink border border-line-strong hover:bg-surface-4 hover:border-brand-600/50 focus-visible:ring-brand-600/30',
  danger: 'bg-danger-500 text-white hover:bg-danger-400 focus-visible:ring-danger-500/40',
  ghost: 'text-ink-muted hover:bg-surface-3 hover:text-ink-strong focus-visible:ring-line-strong',
}

const sizeClass: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-xs',
  md: 'h-9 px-4 text-sm',
  lg: 'h-11 px-5 text-base',
}

const iconSize: Record<ButtonSize, number> = { sm: 14, md: 16, lg: 18 }

const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'primary', size = 'md', loading = false, icon: Icon, block = false, className, children, disabled, type = 'button', ...rest },
  ref,
) {
  const isDisabled = disabled || loading
  return (
    <button
      ref={ref}
      type={type}
      disabled={isDisabled}
      aria-busy={loading || undefined}
      className={clsx(
        'inline-flex items-center justify-center gap-1.5 rounded-lg font-medium whitespace-nowrap transition-colors',
        'focus:outline-none focus-visible:ring-2',
        'disabled:opacity-50 disabled:cursor-not-allowed',
        variantClass[variant],
        sizeClass[size],
        block && 'w-full',
        className,
      )}
      {...rest}
    >
      {loading ? (
        <Loader2 size={iconSize[size]} className="animate-spin shrink-0" />
      ) : Icon ? (
        <Icon size={iconSize[size]} className="shrink-0" />
      ) : null}
      {children}
    </button>
  )
})

export default Button
