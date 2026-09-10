import { useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { Coins, RefreshCw, Wallet } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { billingKeys, getAccountTree } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { AccountNode } from '../../../api/types'
import Button from '../../../components/ui/Button'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import PageHeader from '../../../components/ui/PageHeader'
import Spinner from '../../../components/ui/Spinner'
import Tabs from '../../../components/ui/Tabs'
import { usePermission } from '../../../hooks/usePermission'
import { formatMoney } from '../../../utils/format'
import { balanceClass } from '../style'
import AccountDetailDrawer from './AccountDetailDrawer'
import AccountModals, { type AccountModalState } from './AccountModals'
import AccountTree from './AccountTree'
import TransactionsTab from './TransactionsTab'
import { nodeExists } from './utils'

type PageTab = 'tree' | 'transactions'

function countNodes(n: AccountNode): { total: number; existing: number; negative: number } {
  const acc = { total: 0, existing: 0, negative: 0 }
  const walk = (node: AccountNode) => {
    acc.total += 1
    if (nodeExists(node)) {
      acc.existing += 1
      if (node.balance < 0) acc.negative += 1
    }
    node.children?.forEach(walk)
  }
  walk(n)
  return acc
}

function Stat({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className="rounded-xl border border-line px-4 py-3">
      <div className="text-[11px] text-ink-faint">{label}</div>
      <div className={clsx('mt-0.5 font-mono text-lg font-semibold text-ink-strong', className)}>{value}</div>
    </div>
  )
}

/** 账户管理（/billing/accounts）：账户树 + 全部流水 */
export default function BillingAccountsPage() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const [searchParams, setSearchParams] = useSearchParams()
  const rawTab = searchParams.get('tab')
  const tab: PageTab = rawTab === 'transactions' ? 'transactions' : 'tree'
  const detailId = searchParams.get('id')

  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set(key, value)
    else next.delete(key)
    setSearchParams(next, { replace: true })
  }

  const tree = useQuery({ queryKey: billingKeys.accountTree, queryFn: getAccountTree })
  const [modal, setModal] = useState<AccountModalState>(null)

  const canRecharge = can('billing:account:recharge')
  const canAllocate = can('billing:account:allocate')
  const canAdjust = can('billing:account:adjust')

  const invalidate = () => queryClient.invalidateQueries({ queryKey: billingKeys.all })
  const root = tree.data
  const stats = root ? countNodes(root) : null

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="账户管理"
        description="企业 → 部门 → 员工三级账户：充值、划拨、调整与流水"
        extra={
          <>
            <Button variant="secondary" icon={RefreshCw} loading={tree.isFetching && !tree.isPending} onClick={() => void tree.refetch()}>
              刷新
            </Button>
            {canRecharge && (
              <Button icon={Coins} onClick={() => setModal({ kind: 'recharge' })}>
                企业账户充值
              </Button>
            )}
          </>
        }
      />

      {root && stats && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label="企业账户余额（元）" value={formatMoney(root.balance)} className={balanceClass(root.balance)} />
          <Stat label="企业本月支出（元）" value={formatMoney(root.month_spent ?? 0)} />
          <Stat label="已创建账户" value={`${stats.existing} / ${stats.total}`} />
          <Stat label="余额为负" value={String(stats.negative)} className={stats.negative > 0 ? 'text-danger-200' : undefined} />
        </div>
      )}

      <div className="card p-4">
        <Tabs<PageTab>
          className="mb-4"
          items={[
            { key: 'tree', label: '账户树' },
            { key: 'transactions', label: '全部流水' },
          ]}
          value={tab}
          onChange={(k) => setParam('tab', k === 'tree' ? null : k)}
        />

        {tab === 'tree' ? (
          tree.isPending ? (
            <div className="flex justify-center py-12">
              <Spinner label="加载账户树…" />
            </div>
          ) : tree.isError ? (
            <ErrorState message={errorMessage(tree.error)} onRetry={() => void tree.refetch()} />
          ) : !root ? (
            <Empty icon={Wallet} title="暂无账户" description="企业账户会在首次充值或扣费时自动创建" />
          ) : (
            <AccountTree
              root={root}
              canRecharge={canRecharge}
              canAllocate={canAllocate}
              canAdjust={canAdjust}
              onRecharge={() => setModal({ kind: 'recharge' })}
              onAllocate={(source, target) => setModal({ kind: 'allocate', source, target })}
              onAdjust={(node) => setModal({ kind: 'adjust', account: node })}
              onEdit={(node) => setModal({ kind: 'edit', account: node })}
              onDetail={(node) => setParam('id', node.id)}
            />
          )
        ) : (
          <TransactionsTab />
        )}
      </div>

      <AccountModals
        state={modal}
        onClose={() => setModal(null)}
        onDone={() => {
          setModal(null)
          void invalidate()
        }}
      />
      <AccountDetailDrawer id={detailId} onClose={() => setParam('id', null)} onAdjust={canAdjust ? (a) => setModal({ kind: 'adjust', account: a }) : undefined} onEdit={canAdjust ? (a) => setModal({ kind: 'edit', account: a }) : undefined} />
    </div>
  )
}
