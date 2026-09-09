import { AlertCircle } from 'lucide-react'
import Button from './Button'
import Empty from './Empty'

export interface ErrorStateProps {
  message: string
  onRetry?: () => void
  className?: string
  size?: 'sm' | 'md'
}

/** 请求失败占位（带重试） */
export default function ErrorState({ message, onRetry, className, size }: ErrorStateProps) {
  return (
    <Empty
      icon={AlertCircle}
      title="加载失败"
      description={message}
      className={className}
      size={size}
      action={
        onRetry && (
          <Button variant="secondary" size="sm" onClick={onRetry}>
            重试
          </Button>
        )
      }
    />
  )
}
