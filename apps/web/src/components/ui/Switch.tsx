import clsx from 'clsx'
import type { ReactNode } from 'react'

export interface SwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  label?: ReactNode
  size?: 'sm' | 'md'
  className?: string
  id?: string
  name?: string
}

/** 开关（受控） */
export default function Switch({ checked, onChange, disabled, label, size = 'md', className, id, name }: SwitchProps) {
  const track = size === 'sm' ? 'h-5 w-9' : 'h-6 w-11'
  const knob = size === 'sm' ? 'h-4 w-4' : 'h-5 w-5'
  const shift = size === 'sm' ? 'translate-x-4' : 'translate-x-5'
  return (
    <label className={clsx('inline-flex items-center gap-2 select-none', disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer', className)}>
      <button
        type="button"
        role="switch"
        id={id}
        name={name}
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={clsx(
          'relative inline-flex shrink-0 items-center rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-300',
          track,
          checked ? 'bg-brand-600' : 'bg-slate-300',
          disabled && 'cursor-not-allowed',
        )}
      >
        <span
          className={clsx(
            'inline-block rounded-full bg-white shadow transition-transform translate-x-0.5',
            knob,
            checked && shift,
          )}
        />
      </button>
      {label && <span className="text-sm text-slate-700">{label}</span>}
    </label>
  )
}
