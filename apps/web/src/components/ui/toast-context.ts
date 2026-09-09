import { createContext, useContext } from 'react'

export type ToastType = 'success' | 'error' | 'info' | 'warning'

export interface ToastOptions {
  type?: ToastType
  title: string
  description?: string
  /** 毫秒；缺省成功/信息 3.5s、错误 5s */
  duration?: number
}

export interface ToastApi {
  show: (opts: ToastOptions) => void
  success: (title: string, description?: string) => void
  error: (title: string, description?: string) => void
  info: (title: string, description?: string) => void
  warning: (title: string, description?: string) => void
}

export const ToastContext = createContext<ToastApi | null>(null)

/** 全局提示：`const toast = useToast(); toast.success('已保存')` */
export function useToast(): ToastApi {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast 必须在 <ToastProvider> 内使用')
  return ctx
}
