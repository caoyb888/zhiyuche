import { useEffect } from 'react'

let lockCount = 0

/** 弹层公共行为：打开时锁定 body 滚动、ESC 关闭 */
export function useOverlay(open: boolean, onClose: () => void, closeOnEsc = true): void {
  useEffect(() => {
    if (!open) return
    lockCount += 1
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKey = (e: KeyboardEvent) => {
      if (closeOnEsc && e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      lockCount = Math.max(0, lockCount - 1)
      if (lockCount === 0) document.body.style.overflow = prev
    }
  }, [open, onClose, closeOnEsc])
}
