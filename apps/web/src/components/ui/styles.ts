import clsx from 'clsx'

/** 输入类控件（Input / Select / Textarea / TreeSelect 触发器）的统一外观 */
export function controlClass(invalid?: boolean, extra?: string): string {
  return clsx(
    'w-full rounded-lg border bg-white text-sm text-slate-800 placeholder:text-slate-400 transition-colors',
    'focus:outline-none focus:ring-2',
    'disabled:bg-slate-50 disabled:text-slate-400 disabled:cursor-not-allowed',
    invalid
      ? 'border-red-400 focus:border-red-400 focus:ring-red-200'
      : 'border-slate-200 focus:border-brand-400 focus:ring-brand-100',
    extra,
  )
}
