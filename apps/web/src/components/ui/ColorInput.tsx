import clsx from 'clsx'
import { X } from 'lucide-react'
import Input from './Input'

export interface ColorInputProps {
  /** 颜色文本（如 #10b981）；允许为空 */
  value: string
  onChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
  invalid?: boolean
  id?: string
  className?: string
}

const HEX6 = /^#[0-9a-fA-F]{6}$/

/** 取色器 + 文本输入：文本可填任意 CSS 颜色，取色器只在 #rrggbb 时同步 */
export default function ColorInput({ value, onChange, placeholder = '#10b981', disabled, invalid, id, className }: ColorInputProps) {
  const swatch = HEX6.test(value) ? value : '#000000'
  return (
    <div className={clsx('flex items-center gap-2', className)}>
      <label
        className={clsx(
          'relative h-9 w-9 shrink-0 overflow-hidden rounded-lg border border-slate-200',
          disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer',
        )}
        title="取色"
        style={{ backgroundColor: value || undefined }}
      >
        {!value && <span className="absolute inset-0 bg-[linear-gradient(135deg,#f1f5f9_25%,transparent_25%,transparent_50%,#f1f5f9_50%,#f1f5f9_75%,transparent_75%)] bg-[length:8px_8px]" />}
        <input
          type="color"
          aria-label="取色"
          disabled={disabled}
          value={swatch}
          onChange={(e) => onChange(e.target.value)}
          className="absolute inset-0 h-full w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
        />
      </label>
      <Input
        id={id}
        invalid={invalid}
        disabled={disabled}
        value={value}
        onChange={(e) => onChange(e.target.value.trim())}
        placeholder={placeholder}
        spellCheck={false}
        autoComplete="off"
        className="font-mono"
        suffix={
          value && !disabled ? (
            <button type="button" aria-label="清空颜色" onClick={() => onChange('')} className="rounded p-0.5 text-slate-400 hover:text-slate-600">
              <X size={14} />
            </button>
          ) : undefined
        }
      />
    </div>
  )
}
