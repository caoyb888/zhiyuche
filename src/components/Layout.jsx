import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { LayoutDashboard, FileText, Map, BarChart2, HeartPulse, Zap } from 'lucide-react'

const navItems = [
  { to:'/',         icon:LayoutDashboard, label:'总览',      sub:'实时车辆状态' },
  { to:'/approval', icon:FileText,        label:'公务审批',   sub:'申请与审批流' },
  { to:'/trips',    icon:Map,             label:'行程管理',   sub:'轨迹与记录' },
  { to:'/reports',  icon:BarChart2,       label:'费用报表',   sub:'部门费用分析' },
  { to:'/health',   icon:HeartPulse,      label:'车辆健康',   sub:'AI诊断' },
  { to:'/charging', icon:Zap,             label:'充电管理',   sub:'充电桩状态' },
]

export default function Layout() {
  const loc = useLocation()
  const cur = navItems.find(n => n.to === loc.pathname) || navItems[0]

  return (
    <div className="flex h-screen overflow-hidden bg-slate-50">
      {/* ── Desktop Sidebar ── */}
      <aside className="hidden md:flex flex-col w-60 shrink-0 bg-brand-900 text-white">
        {/* Logo */}
        <div className="px-5 py-5 border-b border-white/10">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-brand-600 flex items-center justify-center text-white font-bold text-sm">智</div>
            <div>
              <div className="text-sm font-semibold tracking-wide">智御系统</div>
              <div className="text-[10px] text-slate-400 mt-0.5">边缘控车 · 智慧驾驭</div>
            </div>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-3 py-4 space-y-0.5 overflow-y-auto">
          {navItems.map(({ to, icon:Icon, label, sub }) => (
            <NavLink key={to} to={to} end={to==='/'} className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}>
              <Icon size={17} className="shrink-0" />
              <span>{label}</span>
            </NavLink>
          ))}
        </nav>

        {/* Bottom info */}
        <div className="px-4 py-4 border-t border-white/10">
          <div className="text-xs text-slate-500">演示版 V1.0</div>
          <div className="text-xs text-slate-500 mt-0.5">公司A · 车队管理</div>
        </div>
      </aside>

      {/* ── Main ── */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Top bar */}
        <header className="shrink-0 h-14 bg-white border-b border-slate-100 flex items-center px-5 gap-4">
          <div className="flex-1 min-w-0">
            <h1 className="text-sm font-semibold text-slate-800 truncate">{cur.label}</h1>
            <p className="text-xs text-slate-400 hidden sm:block">{cur.sub}</p>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <div className="flex items-center gap-1.5 text-xs text-emerald-600 bg-emerald-50 px-2.5 py-1 rounded-full">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 pulse-dot inline-block" />
              系统运行正常
            </div>
            <div className="w-7 h-7 rounded-full bg-brand-700 text-white text-xs flex items-center justify-center font-medium">管</div>
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-4 md:p-6">
          <Outlet />
        </main>

        {/* ── Mobile Bottom Nav ── */}
        <nav className="md:hidden shrink-0 bg-white border-t border-slate-100 flex justify-around">
          {navItems.map(({ to, icon:Icon, label }) => (
            <NavLink key={to} to={to} end={to==='/'} className={({ isActive }) => `mobile-nav-item ${isActive ? 'active' : ''}`}>
              <Icon size={20} />
              <span className="text-[10px]">{label}</span>
            </NavLink>
          ))}
        </nav>
      </div>
    </div>
  )
}
