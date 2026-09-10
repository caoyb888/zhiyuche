import clsx from 'clsx'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { useOverlay } from './useOverlay'

export type DrawerWidth = 'sm' | 'md' | 'lg'

export interface DrawerProps {
  open: boolean
  onClose: () => void
  title?: ReactNode
  description?: ReactNode
  children: ReactNode
  footer?: ReactNode
  width?: DrawerWidth
  closeOnBackdrop?: boolean
}

const widthClass: Record<DrawerWidth, string> = {
  sm: 'sm:max-w-md',
  md: 'sm:max-w-xl',
  lg: 'sm:max-w-3xl',
}

/** 右侧抽屉（Portal 到 body），移动端占满宽度 */
export default function Drawer({ open, onClose, title, description, children, footer, width = 'md', closeOnBackdrop = true }: DrawerProps) {
  useOverlay(open, onClose)
  if (!open) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex justify-end" role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-[#03080F]/70 backdrop-blur-[2px]" onClick={closeOnBackdrop ? onClose : undefined} />
      <div className={clsx('relative h-full w-full bg-surface-2 border-l border-line shadow-float flex flex-col drawer-in', widthClass[width])}>
        <div
          className="flex items-start justify-between px-5 py-4 border-b border-line bg-surface-1"
          style={{ backgroundImage: 'radial-gradient(rgba(60,207,224,.07) 1px, transparent 1px)', backgroundSize: '22px 22px' }}
        >
          <div className="min-w-0">
            <h3 className="text-base font-semibold text-ink-strong tracking-[0.02em]">{title}</h3>
            {description && <p className="mt-0.5 text-xs text-ink-faint">{description}</p>}
          </div>
          <button type="button" onClick={onClose} className="p-1 rounded-md text-ink-faint hover:text-ink-strong hover:bg-white/[0.06]" aria-label="关闭">
            <X size={18} />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
        {footer && <div className="px-5 py-3 border-t border-line-soft flex items-center justify-end gap-2 bg-surface-2">{footer}</div>}
      </div>
    </div>,
    document.body,
  )
}
