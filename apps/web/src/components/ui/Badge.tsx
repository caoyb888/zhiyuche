import clsx from 'clsx'
import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import type { BadgeClass } from '../../types'

export type BadgeColor = 'green' | 'blue' | 'amber' | 'red' | 'gray' | 'purple'

export interface BadgeProps {
  color?: BadgeColor
  icon?: LucideIcon
  children: ReactNode
  className?: string
}

const classByColor: Record<BadgeColor, BadgeClass> = {
  green: 'badge-green',
  blue: 'badge-blue',
  amber: 'badge-amber',
  red: 'badge-red',
  gray: 'badge-gray',
  purple: 'badge-purple',
}

/** 状态徽标，复用 index.css 中的 .badge-* 样式 */
export default function Badge({ color = 'gray', icon: Icon, children, className }: BadgeProps) {
  return (
    <span className={clsx(classByColor[color], className)}>
      {Icon && <Icon size={12} />}
      {children}
    </span>
  )
}
