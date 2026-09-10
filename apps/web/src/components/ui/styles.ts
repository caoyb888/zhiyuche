import clsx from 'clsx'

/** 输入类控件（Input / Select / Textarea / TreeSelect 触发器）的统一外观 */
export function controlClass(invalid?: boolean, extra?: string): string {
  return clsx(
    'w-full rounded-lg border bg-surface-3 text-sm text-ink placeholder:text-ink-disabled transition-colors',
    'focus:outline-none focus:ring-2',
    'disabled:bg-surface-2 disabled:text-ink-disabled disabled:cursor-not-allowed',
    invalid
      ? 'border-danger-400 focus:border-danger-400 focus:ring-danger-500/25'
      : 'border-line-strong focus:border-brand-500 focus:ring-brand-600/25',
    extra,
  )
}
