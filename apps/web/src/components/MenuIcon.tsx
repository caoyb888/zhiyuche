import { createElement } from 'react'
import { getMenuIcon } from './icons'

export interface MenuIconProps {
  /** 后端菜单 icon 字段（lucide 图标名），未知或缺省时显示 Circle */
  name?: string
  size?: number
  className?: string
}

/** 按后端菜单 icon 名渲染 lucide 图标 */
export default function MenuIcon({ name, size = 17, className }: MenuIconProps) {
  return createElement(getMenuIcon(name), { size, className })
}
