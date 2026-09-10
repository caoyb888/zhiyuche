import clsx from 'clsx'
import { AlertTriangle, CheckCircle2, Info, X, XCircle, type LucideIcon } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { ToastContext, type ToastApi, type ToastOptions, type ToastType } from './toast-context'

interface ToastItem extends Required<Pick<ToastOptions, 'type' | 'title'>> {
  id: number
  description?: string
}

const iconByType: Record<ToastType, LucideIcon> = {
  success: CheckCircle2,
  error: XCircle,
  info: Info,
  warning: AlertTriangle,
}

const styleByType: Record<ToastType, string> = {
  success: 'border-ev-500/40 text-ev-200',
  error: 'border-danger-500/40 text-danger-200',
  info: 'border-brand-600/45 text-brand-300',
  warning: 'border-warn-500/40 text-warn-200',
}

const MAX_VISIBLE = 5

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const seq = useRef(0)
  const timers = useRef(new Map<number, number>())

  const dismiss = useCallback((id: number) => {
    const t = timers.current.get(id)
    if (t !== undefined) {
      window.clearTimeout(t)
      timers.current.delete(id)
    }
    setItems((list) => list.filter((it) => it.id !== id))
  }, [])

  const show = useCallback(
    (opts: ToastOptions) => {
      const type = opts.type ?? 'info'
      const id = ++seq.current
      const duration = opts.duration ?? (type === 'error' ? 5_000 : 3_500)
      setItems((list) => [...list.slice(-(MAX_VISIBLE - 1)), { id, type, title: opts.title, description: opts.description }])
      timers.current.set(id, window.setTimeout(() => dismiss(id), duration))
    },
    [dismiss],
  )

  // 卸载时清理所有定时器
  useEffect(() => {
    const map = timers.current
    return () => {
      map.forEach((t) => window.clearTimeout(t))
      map.clear()
    }
  }, [])

  const api = useMemo<ToastApi>(
    () => ({
      show,
      success: (title, description) => show({ type: 'success', title, description }),
      error: (title, description) => show({ type: 'error', title, description }),
      info: (title, description) => show({ type: 'info', title, description }),
      warning: (title, description) => show({ type: 'warning', title, description }),
    }),
    [show],
  )

  return (
    <ToastContext.Provider value={api}>
      {children}
      {createPortal(
        <div className="pointer-events-none fixed top-4 right-4 z-[60] flex w-[calc(100vw-2rem)] max-w-sm flex-col gap-2" aria-live="polite">
          {items.map((it) => {
            const Icon = iconByType[it.type]
            return (
              <div
                key={it.id}
                role={it.type === 'error' ? 'alert' : 'status'}
                className={clsx('pointer-events-auto flex items-start gap-2.5 rounded-card border bg-surface-2 px-4 py-3 shadow-float slide-up', styleByType[it.type])}
              >
                <Icon size={18} className="mt-0.5 shrink-0" />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-ink-strong break-words">{it.title}</div>
                  {it.description && <div className="mt-0.5 text-xs text-ink-muted break-words">{it.description}</div>}
                </div>
                <button type="button" onClick={() => dismiss(it.id)} className="shrink-0 rounded p-0.5 text-ink-faint hover:text-ink-strong" aria-label="关闭提示">
                  <X size={14} />
                </button>
              </div>
            )
          })}
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  )
}
