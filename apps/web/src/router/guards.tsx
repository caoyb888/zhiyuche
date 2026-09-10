import { useQuery } from '@tanstack/react-query'
import { useEffect, type ReactNode } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { authKeys, getMe } from '../api/auth'
import { errorMessage } from '../api/client'
import PageLoading from '../components/PageLoading'
import Button from '../components/ui/Button'
import ErrorState from '../components/ui/ErrorState'
import { usePermission } from '../hooks/usePermission'
import Forbidden from '../pages/Forbidden'
import { useAuthStore } from '../store/auth'

/** 构造带 redirect 参数的登录地址 */
function loginUrl(pathname: string, search: string): string {
  const target = pathname + search
  if (!target || target === '/') return '/login'
  return `/login?redirect=${encodeURIComponent(target)}`
}

/**
 * 认证守卫：未登录重定向 /login；已登录时后台刷新 /auth/me 保持权限与菜单最新。
 * 401 且刷新失败时 client 会清空 store，本组件随之跳转登录页。
 */
export function RequireAuth() {
  const token = useAuthStore((s) => s.accessToken)
  const profile = useAuthStore((s) => s.profile)
  const setProfile = useAuthStore((s) => s.setProfile)
  const clear = useAuthStore((s) => s.clear)
  const loc = useLocation()

  const me = useQuery({
    queryKey: authKeys.me,
    queryFn: getMe,
    enabled: Boolean(token),
    staleTime: 5 * 60_000,
  })

  useEffect(() => {
    if (me.data) setProfile(me.data)
  }, [me.data, setProfile])

  if (!token) {
    return <Navigate to={loginUrl(loc.pathname, loc.search)} replace />
  }

  // 有令牌但本地没有资料（极少见：存储被清理），必须等 /auth/me
  if (!profile) {
    if (me.isError) {
      return (
        <div className="flex min-h-screen items-center justify-center bg-surface-3 p-4">
          <div className="card w-full max-w-sm p-6">
            <ErrorState message={errorMessage(me.error)} onRetry={() => void me.refetch()} />
            <div className="mt-2 text-center">
              <Button variant="ghost" size="sm" onClick={clear}>
                重新登录
              </Button>
            </div>
          </div>
        </div>
      )
    }
    return (
      <div className="min-h-screen bg-surface-3">
        <PageLoading label="正在加载用户资料…" />
      </div>
    )
  }

  return <Outlet />
}

/** 页面级权限守卫：无权限时原地渲染 403（不跳转，保留 URL） */
export function PermissionGuard({ perm, children }: { perm: string; children: ReactNode }) {
  const { can } = usePermission()
  return can(perm) ? <>{children}</> : <Forbidden />
}

/** 仅未登录可见（登录页）：已登录则回到首页或 redirect */
export function PublicOnly({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.accessToken)
  const profile = useAuthStore((s) => s.profile)
  const loc = useLocation()
  if (token && profile) {
    const redirect = new URLSearchParams(loc.search).get('redirect')
    return <Navigate to={redirect && redirect.startsWith('/') ? redirect : '/'} replace />
  }
  return <>{children}</>
}
