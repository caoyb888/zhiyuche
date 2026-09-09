import clsx from 'clsx'
import Checkbox from './Checkbox'

export interface CheckboxGroupOption {
  value: string
  label: string
  description?: string
  disabled?: boolean
}

export interface CheckboxGroupProps {
  options: CheckboxGroupOption[]
  value: string[]
  onChange: (value: string[]) => void
  disabled?: boolean
  /** 网格列数（移动端始终单列） */
  columns?: 1 | 2 | 3
  invalid?: boolean
  className?: string
  emptyText?: string
}

const columnClass: Record<NonNullable<CheckboxGroupProps['columns']>, string> = {
  1: 'grid-cols-1',
  2: 'sm:grid-cols-2',
  3: 'sm:grid-cols-3',
}

/** 多选框组（受控），用于角色多选等场景 */
export default function CheckboxGroup({
  options,
  value,
  onChange,
  disabled,
  columns = 2,
  invalid,
  className,
  emptyText = '暂无可选项',
}: CheckboxGroupProps) {
  const selected = new Set(value)
  const toggle = (v: string, checked: boolean) => {
    const next = new Set(selected)
    if (checked) next.add(v)
    else next.delete(v)
    onChange(options.filter((o) => next.has(o.value)).map((o) => o.value))
  }
  if (options.length === 0) {
    return <div className="text-sm text-slate-400 py-1">{emptyText}</div>
  }
  return (
    <div
      className={clsx(
        'grid grid-cols-1 gap-x-4 gap-y-2 rounded-lg border p-3',
        invalid ? 'border-red-300' : 'border-slate-200',
        columnClass[columns],
        className,
      )}
    >
      {options.map((o) => (
        <Checkbox
          key={o.value}
          label={o.label}
          description={o.description}
          checked={selected.has(o.value)}
          disabled={disabled || o.disabled}
          onChange={(e) => toggle(o.value, e.target.checked)}
        />
      ))}
    </div>
  )
}
