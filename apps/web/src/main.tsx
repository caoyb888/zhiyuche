import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import './index.css'
import { ErrorCode, isApiError } from './api/client'
import { ToastProvider } from './components/ui/Toast'
import AppRouter from './router'

const NO_RETRY_CODES = new Set<number>([ErrorCode.BadRequest, ErrorCode.Unauthorized, ErrorCode.Forbidden, ErrorCode.NotFound, ErrorCode.Conflict])

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      // 业务错误（参数/权限/不存在/冲突）不重试，网络抖动最多重试 1 次
      retry: (failureCount, error) => {
        if (isApiError(error) && NO_RETRY_CODES.has(error.code)) return false
        return failureCount < 1
      },
    },
    mutations: { retry: false },
  },
})

const container = document.getElementById('root')
if (!container) {
  throw new Error('未找到挂载节点 #root')
}

createRoot(container).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <AppRouter />
      </ToastProvider>
    </QueryClientProvider>
  </StrictMode>,
)
