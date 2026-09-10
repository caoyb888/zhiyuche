import clsx from 'clsx'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { useOverlay } from './useOverlay'

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl'

export interface ModalProps {
  open: boolean
  onClose: () => void
  title?: ReactNode
  children: ReactNode
  /** 底部操作区；缺省不渲染 */
  footer?: ReactNode
  size?: ModalSize
  /** 点击遮罩关闭，缺省 true */
  closeOnBackdrop?: boolean
  /** 隐藏右上角关闭按钮 */
  hideClose?: boolean
}

const sizeClass: Record<ModalSize, string> = {
  sm: 'max-w-sm',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
}

/** 居中弹窗（Portal 到 body） */
export default function Modal({ open, onClose, title, children, footer, size = 'md', closeOnBackdrop = true, hideClose = false }: ModalProps) {
  useOverlay(open, onClose)
  if (!open) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4" role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-[#03080F]/70 backdrop-blur-[2px]" onClick={closeOnBackdrop ? onClose : undefined} />
      <div
        className={clsx(
          'relative w-full bg-surface-2 border border-line shadow-float flex flex-col max-h-[92vh] slide-up',
          'rounded-t-[14px] sm:rounded-[14px]',
          sizeClass[size],
        )}
      >
        {(title || !hideClose) && (
          <div className="flex items-center justify-between px-5 py-4 border-b border-line-soft">
            <h3 className="text-base font-semibold text-ink-strong">{title}</h3>
            {!hideClose && (
              <button type="button" onClick={onClose} className="p-1 rounded-md text-ink-faint hover:text-ink-strong hover:bg-surface-3" aria-label="关闭">
                <X size={18} />
              </button>
            )}
          </div>
        )}
        <div className="px-5 py-4 overflow-y-auto">{children}</div>
        {footer && <div className="px-5 py-3 border-t border-line-soft flex items-center justify-end gap-2">{footer}</div>}
      </div>
    </div>,
    document.body,
  )
}
