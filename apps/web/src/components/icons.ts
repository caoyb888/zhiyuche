import {
  BarChart2,
  Bell,
  BookOpen,
  Building2,
  Calculator,
  Car,
  Circle,
  Cpu,
  CreditCard,
  FileText,
  HeartPulse,
  KeyRound,
  LayoutDashboard,
  Map,
  MessageSquare,
  Network,
  PlugZap,
  Receipt,
  ScrollText,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Users,
  Wallet,
  Zap,
  type LucideIcon,
} from 'lucide-react'

/**
 * 后端菜单 `icon` 字段（lucide 图标名，见 apps/api/internal/perm/registry.go）→ 组件。
 * 显式映射而不是整包导入，保证 tree-shaking。
 */
const menuIcons: Record<string, LucideIcon> = {
  LayoutDashboard,
  FileText,
  Map,
  BarChart2,
  HeartPulse,
  Zap,
  Settings,
  Users,
  Network,
  ShieldCheck,
  KeyRound,
  BookOpen,
  SlidersHorizontal,
  MessageSquare,
  ScrollText,
  Building2,
  // 资产管理 / 通知
  Car,
  Cpu,
  CreditCard,
  PlugZap,
  Bell,
  // 计费账户
  Wallet,
  Calculator,
  Receipt,
}

/** 按名称取图标，未知名称回退为 Circle */
export function getMenuIcon(name: string | undefined): LucideIcon {
  if (!name) return Circle
  return menuIcons[name] ?? Circle
}
