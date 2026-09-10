import { forwardRef, type SelectHTMLAttributes } from 'react'
import clsx from 'clsx'
import { ChevronDown } from 'lucide-react'
import { controlClass } from './styles'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'children'> {
  options: SelectOption[]
  /** 显示为第一个空值选项（value=''），可用于"全部"/"请选择" */
  placeholder?: string
  invalid?: boolean
}

/** 原生 select 封装：统一外观 + 下拉箭头 */
const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { options, placeholder, invalid, className, ...rest },
  ref,
) {
  return (
    <div className={clsx('relative w-full', className)}>
      <select
        ref={ref}
        aria-invalid={invalid || undefined}
        className={controlClass(invalid, 'h-9 pl-3 pr-8 appearance-none cursor-pointer')}
        {...rest}
      >
        {placeholder !== undefined && <option value="">{placeholder}</option>}
        {options.map((o) => (
          <option key={o.value} value={o.value} disabled={o.disabled}>
            {o.label}
          </option>
        ))}
      </select>
      <ChevronDown size={16} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-ink-faint pointer-events-none" />
    </div>
  )
})

export default Select
