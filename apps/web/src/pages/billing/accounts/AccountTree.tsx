import clsx from 'clsx'
import { ArrowDownToLine, ChevronRight, Coins, Pencil, PlusCircle, ReceiptText, Send } from 'lucide-react'
import { useMemo, useState } from 'react'
import type { AccountNode } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import { formatMoney } from '../../../utils/format'
import { ACCOUNT_LEVEL_BADGE, ACCOUNT_LEVEL_LABEL, ACCOUNT_STATUS_BADGE, ACCOUNT_STATUS_LABEL, balanceClass, budgetUsage, childLevel, usageBarClass } from '../style'
import { nodeExists, nodeKey } from './utils'

export interface AccountTreeActions {
  canRecharge: boolean
  canAllocate: boolean
  canAdjust: boolean
  onRecharge: () => void
  /** source 向下划拨；target 给定时目标固定（可为未创建节点 → 划拨即创建） */
  onAllocate: (source: AccountNode, target?: AccountNode) => void
  onAdjust: (node: AccountNode) => void
  onEdit: (node: AccountNode) => void
  onDetail: (node: AccountNode) => void
}

interface AccountTreeProps extends AccountTreeActions {
  root: AccountNode
}

interface Row {
  node: AccountNode
  parent: AccountNode | null
  depth: number
  key: string
  hasChildren: boolean
}

/** 深度优先展平可见节点 */
function flatten(root: AccountNode, collapsed: ReadonlySet<string>): Row[] {
  const out: Row[] = []
  const walk = (node: AccountNode, parent: AccountNode | null, depth: number) => {
    const key = nodeKey(node)
    const children = node.children ?? []
    out.push({ node, parent, depth, key, hasChildren: children.length > 0 })
    if (children.length > 0 && !collapsed.has(key)) {
      for (const c of children) walk(c, node, depth + 1)
    }
  }
  walk(root, null, 0)
  return out
}

/** 月预算与本月支出进度条 */
export function BudgetBar({ spent, budget }: { spent: number | null | undefined; budget: number }) {
  const pct = budgetUsage(spent, budget)
  if (pct === null) {
    return (
      <div className="text-xs text-slate-400">
        未设预算 · 本月支出 <span className="font-mono text-slate-600">{formatMoney(spent ?? 0)}</span>
      </div>
    )
  }
  return (
    <div className="min-w-[9rem]">
      <div className="flex items-center justify-between text-[11px]">
        <span className="font-mono text-slate-600">{formatMoney(spent ?? 0)}</span>
        <span className="text-slate-400">/ {formatMoney(budget)}</span>
      </div>
      <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-slate-100">
        <div className={clsx('h-full rounded-full transition-all', usageBarClass(pct))} style={{ width: `${Math.min(100, pct)}%` }} />
      </div>
      <div className={clsx('mt-0.5 text-[11px]', pct >= 100 ? 'text-red-600 font-medium' : 'text-slate-400')}>{pct.toFixed(0)}%{pct >= 100 ? ' 已超支' : ''}</div>
    </div>
  )
}

/** 三级账户树：企业 → 部门 → 员工，表格式呈现，可展开收起 */
export default function AccountTree({ root, canRecharge, canAllocate, canAdjust, onRecharge, onAllocate, onAdjust, onEdit, onDetail }: AccountTreeProps) {
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set())
  const rows = useMemo(() => flatten(root, collapsed), [root, collapsed])

  const toggle = (key: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })

  return (
    <div className="overflow-x-auto">
      <table className="data-table">
        <thead>
          <tr>
            <th className="pr-3 min-w-[16rem]">账户</th>
            <th className="pr-3 text-right whitespace-nowrap">余额（元）</th>
            <th className="pr-3 text-right whitespace-nowrap">透支额度</th>
            <th className="pr-3 whitespace-nowrap">月预算 / 本月支出</th>
            <th className="pr-3 whitespace-nowrap">状态</th>
            <th className="text-right whitespace-nowrap">操作</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ node, parent, depth, key, hasChildren }) => {
            const exists = nodeExists(node)
            const open = hasChildren && !collapsed.has(key)
            const parentExists = parent !== null && nodeExists(parent)
            const isEnterprise = node.level === 'enterprise'
            const canAllocateDown = canAllocate && exists && childLevel(node.level) !== null
            const canAllocateIn = canAllocate && !isEnterprise && parentExists
            return (
              <tr key={key} className={clsx(!exists && 'opacity-60')}>
                <td className="pr-3 align-middle">
                  <div className="flex items-center gap-1" style={{ paddingLeft: depth * 20 }}>
                    <button
                      type="button"
                      tabIndex={-1}
                      aria-label={open ? '收起' : '展开'}
                      onClick={() => hasChildren && toggle(key)}
                      className={clsx('flex h-5 w-5 shrink-0 items-center justify-center rounded text-slate-400', hasChildren ? 'hover:bg-slate-200/70 hover:text-slate-600' : 'invisible')}
                    >
                      <ChevronRight size={14} className={clsx('transition-transform', open && 'rotate-90')} />
                    </button>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <button type="button" onClick={() => exists && onDetail(node)} className={clsx('truncate text-sm text-left', exists ? 'font-medium text-slate-800 hover:text-brand-700' : 'text-slate-500 cursor-default')}>
                          {node.owner_name}
                        </button>
                        <Badge color={ACCOUNT_LEVEL_BADGE[node.level]}>{ACCOUNT_LEVEL_LABEL[node.level]}</Badge>
                        {!exists && <span className="text-[11px] text-slate-400">未创建</span>}
                      </div>
                      {node.owner_sub && <div className="text-[11px] text-slate-400 truncate">{node.owner_sub}</div>}
                    </div>
                  </div>
                </td>
                <td className={clsx('pr-3 text-right align-middle font-mono font-medium', exists ? balanceClass(node.balance) : 'text-slate-400')}>{formatMoney(node.balance)}</td>
                <td className="pr-3 text-right align-middle font-mono text-xs text-slate-600">{formatMoney(node.credit_limit)}</td>
                <td className="pr-3 align-middle">{exists ? <BudgetBar spent={node.month_spent} budget={node.monthly_budget} /> : <span className="text-xs text-slate-300">—</span>}</td>
                <td className="pr-3 align-middle">{exists ? <Badge color={ACCOUNT_STATUS_BADGE[node.status]}>{ACCOUNT_STATUS_LABEL[node.status]}</Badge> : <span className="text-xs text-slate-300">—</span>}</td>
                <td className="align-middle whitespace-nowrap">
                  <div className="flex items-center justify-end gap-0.5">
                    {isEnterprise && canRecharge && <Button variant="ghost" size="sm" icon={Coins} className="!px-2 text-emerald-600 hover:bg-emerald-50" title="充值" aria-label="充值" onClick={onRecharge} />}
                    {canAllocateDown && <Button variant="ghost" size="sm" icon={Send} className="!px-2" title="向下划拨" aria-label="向下划拨" onClick={() => onAllocate(node)} />}
                    {canAllocateIn && (
                      <Button
                        variant="ghost"
                        size="sm"
                        icon={exists ? ArrowDownToLine : PlusCircle}
                        className={clsx('!px-2', !exists && 'text-brand-600 hover:bg-brand-50')}
                        title={exists ? `从「${parent?.owner_name ?? ''}」划入` : '划拨额度并创建账户'}
                        aria-label={exists ? '划入' : '划拨创建'}
                        onClick={() => parent && onAllocate(parent, node)}
                      >
                        {!exists && '划拨创建'}
                      </Button>
                    )}
                    {exists && canAdjust && <Button variant="ghost" size="sm" icon={ReceiptText} className="!px-2" title="余额调整" aria-label="余额调整" onClick={() => onAdjust(node)} />}
                    {exists && canAdjust && <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑额度 / 预算 / 冻结" aria-label="编辑" onClick={() => onEdit(node)} />}
                    {exists && (
                      <Button variant="ghost" size="sm" className="!px-2" onClick={() => onDetail(node)}>
                        流水
                      </Button>
                    )}
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
