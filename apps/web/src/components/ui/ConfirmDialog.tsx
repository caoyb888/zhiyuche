import { AlertTriangle } from 'lucide-react'
import type { ReactNode } from 'react'
import Button from './Button'
import Modal from './Modal'

export interface ConfirmDialogProps {
  open: boolean
  title: ReactNode
  description?: ReactNode
  confirmText?: string
  cancelText?: string
  /** 危险操作（红色确认按钮） */
  danger?: boolean
  loading?: boolean
  onConfirm: () => void
  onCancel: () => void
}

/** 二次确认弹窗 */
export default function ConfirmDialog({
  open,
  title,
  description,
  confirmText = '确定',
  cancelText = '取消',
  danger = false,
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <Modal
      open={open}
      onClose={loading ? () => undefined : onCancel}
      size="sm"
      hideClose
      closeOnBackdrop={!loading}
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={loading}>
            {cancelText}
          </Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={onConfirm} loading={loading}>
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="flex gap-3">
        <div className={danger ? 'text-red-500' : 'text-amber-500'}>
          <AlertTriangle size={22} />
        </div>
        <div className="min-w-0">
          <div className="text-sm font-semibold text-slate-800">{title}</div>
          {description && <div className="mt-1 text-sm text-slate-500">{description}</div>}
        </div>
      </div>
    </Modal>
  )
}
