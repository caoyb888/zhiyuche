import { forwardRef, useId, type InputHTMLAttributes, type ReactNode } from 'react'
import clsx from 'clsx'

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: ReactNode
  description?: ReactNode
}

const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(function Checkbox(
  { label, description, className, id, disabled, ...rest },
  ref,
) {
  const autoId = useId()
  const inputId = id ?? autoId
  return (
    <label
      htmlFor={inputId}
      className={clsx('inline-flex items-start gap-2 text-sm select-none', disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer', className)}
    >
      <input
        ref={ref}
        id={inputId}
        type="checkbox"
        disabled={disabled}
        className="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-300 text-brand-600 focus:ring-brand-300 accent-brand-600 cursor-pointer disabled:cursor-not-allowed"
        {...rest}
      />
      {(label || description) && (
        <span className="flex flex-col">
          {label && <span className="text-slate-700">{label}</span>}
          {description && <span className="text-xs text-slate-400">{description}</span>}
        </span>
      )}
    </label>
  )
})

export default Checkbox
