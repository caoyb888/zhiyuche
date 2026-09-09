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
  primary: 'bg-brand-600 text-white hover:bg-brand-700 focus-visible:ring-brand-300',
  secondary: 'bg-white text-slate-700 border border-slate-200 hover:bg-slate-50 hover:border-slate-300 focus-visible:ring-brand-200',
  danger: 'bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-300',
  ghost: 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 focus-visible:ring-slate-200',
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
