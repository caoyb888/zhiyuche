import { useMutation, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { Building2, ChevronDown, KeyRound, LogOut } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { logout as apiLogout } from '../api/auth'
import { getHealth } from '../api/system'
import type { MenuNode } from '../api/types'
import { useWebSocket } from '../hooks/useWebSocket'
import { findAppRoute } from '../router/routes'
import { useAuthStore } from '../store/auth'
import ChangePasswordModal from './ChangePasswordModal'
import MapProvider from './map/MapProvider'
import MenuIcon from './MenuIcon'
import NotificationBell from './NotificationBell'
import TenantSwitcher from './TenantSwitcher'

/** 后端连接状态 */
type BackendStatus =
  | { kind: 'checking' }
  | { kind: 'ok'; version: string }
  | { kind: 'down' }

const HEALTH_POLL_MS = 30_000
const MOBILE_NAV_MAX = 5

function BackendBadge({ status }: { status: BackendStatus }) {
  if (status.kind === 'ok') {
    return (
      <div className="hidden sm:flex items-center gap-1.5 text-xs text-emerald-600 bg-emerald-50 px-2.5 py-1 rounded-full">
        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 pulse-dot inline-block" />
        服务正常 · v{status.version}
      </div>
    )
  }
  if (status.kind === 'down') {
    return (
      <div className="hidden sm:flex items-center gap-1.5 text-xs text-amber-700 bg-amber-50 px-2.5 py-1 rounded-full">
        <span className="w-1.5 h-1.5 rounded-full bg-amber-500 inline-block" />
        后端未连接
      </div>
    )
  }
  return (
    <div className="hidden sm:flex items-center gap-1.5 text-xs text-slate-500 bg-slate-100 px-2.5 py-1 rounded-full">
      <span className="w-1.5 h-1.5 rounded-full bg-slate-400 pulse-dot inline-block" />
      连接中…
    </div>
  )
}

/** 分组菜单（如"系统管理"）的跳转目标：第一个子菜单 */
function menuHref(node: MenuNode): string {
  return node.children && node.children.length > 0 ? node.children[0].path : node.path
}

interface MenuHit {
  node: MenuNode
  parent?: MenuNode
}

function findMenuByPath(menus: MenuNode[], pathname: string, parent?: MenuNode): MenuHit | undefined {
  for (const m of menus) {
    if (m.path === pathname && !(m.children && m.children.length > 0)) return { node: m, parent }
    if (m.children) {
      const hit = findMenuByPath(m.children, pathname, m)
      if (hit) return hit
    }
  }
  return undefined
}

/** 侧栏可折叠分组 */
function SidebarGroup({ node, pathname }: { node: MenuNode; pathname: string }) {
  const children = node.children ?? []
  const childActive = children.some((c) => c.path === pathname)
  // null = 跟随"是否有子项激活"自动展开；用户手动点击后以手动状态为准
  const [manual, setManual] = useState<boolean | null>(null)
  const open = manual ?? childActive

  return (
    <div>
      <button type="button" onClick={() => setManual(!open)} className={clsx('sidebar-group-btn', childActive && 'text-white')} aria-expanded={open}>
        <MenuIcon name={node.icon} size={17} className="shrink-0" />
        <span className="flex-1 text-left">{node.name}</span>
        <ChevronDown size={14} className={clsx('shrink-0 transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <div className="mt-0.5 space-y-0.5">
          {children.map((c) => (
            <NavLink key={c.code} to={c.path} className={({ isActive }) => clsx('sidebar-sublink', isActive && 'active')}>
              <span className="truncate">{c.name}</span>
            </NavLink>
          ))}
        </div>
      )}
    </div>
  )
}

/** 顶栏用户下拉 */
function UserMenu({ onChangePassword, onLogout, loggingOut }: { onChangePassword: () => void; onLogout: () => void; loggingOut: boolean }) {
  const profile = useAuthStore((s) => s.profile)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  if (!profile) return null
  const initial = (profile.name || profile.username).slice(0, 1)

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 rounded-full pl-1 pr-2 py-0.5 hover:bg-slate-100 transition-colors"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <div className="w-7 h-7 rounded-full bg-brand-700 text-white text-xs flex items-center justify-center font-medium">{initial}</div>
        <div className="hidden sm:flex flex-col items-start leading-tight">
          <span className="text-xs font-medium text-slate-700 max-w-[8rem] truncate">{profile.name}</span>
          <span className="text-[10px] text-slate-400 max-w-[8rem] truncate">{profile.tenant.name}</span>
        </div>
        <ChevronDown size={14} className={clsx('text-slate-400 transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <div role="menu" className="absolute right-0 mt-1.5 w-56 rounded-xl border border-slate-100 bg-white shadow-lg py-1.5 z-40 slide-up">
          <div className="px-3 py-2 border-b border-slate-100">
            <div className="text-sm font-medium text-slate-800 truncate">{profile.name}</div>
            <div className="text-xs text-slate-400 truncate">@{profile.username}</div>
            <div className="mt-1 flex items-center gap-1 text-xs text-slate-500 truncate">
              <Building2 size={12} />
              <span className="truncate">{profile.tenant.name}</span>
              {profile.is_super && <span className="badge-purple ml-1">平台管理员</span>}
            </div>
          </div>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setOpen(false)
              onChangePassword()
            }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50"
          >
            <KeyRound size={15} className="text-slate-400" />
            修改密码
          </button>
          <button
            type="button"
            role="menuitem"
            disabled={loggingOut}
            onClick={() => {
              setOpen(false)
              onLogout()
            }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-red-600 hover:bg-red-50 disabled:opacity-50"
          >
            <LogOut size={15} />
            退出登录
          </button>
        </div>
      )}
    </div>
  )
}

export default function Layout() {
  const loc = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const profile = useAuthStore((s) => s.profile)
  const viewTenant = useAuthStore((s) => s.viewTenant)
  const clear = useAuthStore((s) => s.clear)
  const menus = profile?.menus ?? []

  const [backend, setBackend] = useState<BackendStatus>({ kind: 'checking' })
  const [pwdOpen, setPwdOpen] = useState(false)

  // 登录期间维持 WebSocket 实时推送；退出（token 清空）或 Layout 卸载时断开
  useWebSocket()

  useEffect(() => {
    let cancelled = false
    const poll = async () => {
      try {
        const health = await getHealth()
        if (!cancelled) setBackend({ kind: 'ok', version: health.version })
      } catch {
        if (!cancelled) setBackend({ kind: 'down' })
      }
    }
    void poll()
    const timer = window.setInterval(() => {
      void poll()
    }, HEALTH_POLL_MS)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [])

  const logout = useMutation({
    mutationFn: () => apiLogout(useAuthStore.getState().refreshToken),
    // 无论后端是否成功吊销，本地都清空并回到登录页
    onSettled: () => {
      clear()
      queryClient.clear()
      navigate('/login', { replace: true })
    },
  })

  const hit = findMenuByPath(menus, loc.pathname)
  const route = findAppRoute(loc.pathname)
  const title = hit?.node.name ?? route?.title ?? (loc.pathname === '/403' ? '无权限' : '智御系统')
  const subtitle = hit?.parent?.name ?? route?.description ?? ''

  const mobileItems = menus.slice(0, MOBILE_NAV_MAX)

  return (
    <div className="flex h-screen overflow-hidden bg-slate-50">
      {/* ── Desktop Sidebar ── */}
      <aside className="hidden md:flex flex-col w-60 shrink-0 bg-brand-900 text-white">
        <div className="px-5 py-5 border-b border-white/10">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-brand-600 flex items-center justify-center text-white font-bold text-sm">智</div>
            <div>
              <div className="text-sm font-semibold tracking-wide">智御系统</div>
              <div className="text-[10px] text-slate-400 mt-0.5">边缘控车 · 智慧驾驭</div>
            </div>
          </div>
        </div>

        <nav className="flex-1 px-3 py-4 space-y-0.5 overflow-y-auto">
          {menus.length === 0 && <div className="px-3 py-2 text-xs text-slate-500">暂无可用菜单</div>}
          {menus.map((m) => {
            if (m.children && m.children.length > 0) {
              return <SidebarGroup key={m.code} node={m} pathname={loc.pathname} />
            }
            return (
              <NavLink key={m.code} to={m.path} end={m.path === '/'} className={({ isActive }) => clsx('sidebar-link', isActive && 'active')}>
                <MenuIcon name={m.icon} size={17} className="shrink-0" />
                <span>{m.name}</span>
              </NavLink>
            )
          })}
        </nav>

        <div className="px-4 py-4 border-t border-white/10">
          <div className="text-xs text-slate-500">测试版 V2.0</div>
          <div className="text-xs text-slate-500 mt-0.5 truncate">
            {viewTenant ? `查看：${viewTenant.name}` : (profile?.tenant.name ?? '—')} · 车队管理
          </div>
        </div>
      </aside>

      {/* ── Main ── */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <header className="shrink-0 h-14 bg-white border-b border-slate-100 flex items-center px-4 md:px-5 gap-3">
          <div className="flex-1 min-w-0">
            <h1 className="text-sm font-semibold text-slate-800 truncate">{title}</h1>
            {subtitle && <p className="text-xs text-slate-400 hidden sm:block truncate">{subtitle}</p>}
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <TenantSwitcher />
            <BackendBadge status={backend} />
            <NotificationBell />
            <UserMenu onChangePassword={() => setPwdOpen(true)} onLogout={() => logout.mutate()} loggingOut={logout.isPending} />
          </div>
        </header>

        <main className="flex-1 overflow-y-auto p-4 md:p-6">
          <MapProvider>
            <Outlet />
          </MapProvider>
        </main>

        {/* ── Mobile Bottom Nav：一级菜单前 5 个 ── */}
        {mobileItems.length > 0 && (
          <nav className="md:hidden shrink-0 bg-white border-t border-slate-100 flex justify-around">
            {mobileItems.map((m) => {
              const isGroup = Boolean(m.children && m.children.length > 0)
              const groupActive = isGroup && (loc.pathname === m.path || loc.pathname.startsWith(`${m.path}/`))
              return (
                <NavLink
                  key={m.code}
                  to={menuHref(m)}
                  end={m.path === '/'}
                  className={({ isActive }) => clsx('mobile-nav-item', (isGroup ? groupActive : isActive) && 'active')}
                >
                  <MenuIcon name={m.icon} size={20} />
                  <span className="text-[10px]">{m.name}</span>
                </NavLink>
              )
            })}
          </nav>
        )}
      </div>

      <ChangePasswordModal open={pwdOpen} onClose={() => setPwdOpen(false)} />
    </div>
  )
}
