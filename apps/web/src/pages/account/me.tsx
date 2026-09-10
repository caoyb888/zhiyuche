import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { RefreshCw, Wallet } from 'lucide-react'
import type { ReactNode } from 'react'
import { billingKeys, getMyAccount } from '../../api/billing'
import { errorMessage } from '../../api/client'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import ErrorState from '../../components/ui/ErrorState'
import PageHeader from '../../components/ui/PageHeader'
import Spinner from '../../components/ui/Spinner'
import { useAuthStore } from '../../store/auth'
import { formatDateTime, formatMoney } from '../../utils/format'
import { BudgetBar } from '../billing/accounts/AccountTree'
import TransactionTable from '../billing/accounts/TransactionTable'
import { ACCOUNT_STATUS_BADGE, ACCOUNT_STATUS_LABEL, balanceClass } from '../billing/style'

function Stat({ label, value, hint, className }: { label: string; value: ReactNode; hint?: string; className?: string }) {
  return (
    <div className="card p-5">
      <div className="text-xs text-ink-faint">{label}</div>
      <div className={clsx('mt-1 font-mono text-2xl font-semibold text-ink-strong', className)}>{value}</div>
      {hint && <div className="mt-1 text-[11px] text-ink-faint">{hint}</div>}
    </div>
  )
}

/** 我的账户（/me/account，任何登录用户）：员工账户余额、额度、本月支出与最近流水 */
export default function MyAccountPage() {
  const profile = useAuthStore((s) => s.profile)
  const me = useQuery({ queryKey: billingKeys.myAccount, queryFn: getMyAccount })

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="我的账户"
        description={`${profile?.name ?? ''}${profile?.dept_name ? ` · ${profile.dept_name}` : ''} 的员工账户；日常用车费用与个人归属的充电费从此扣除`}
        extra={
          <Button variant="secondary" icon={RefreshCw} loading={me.isFetching && !me.isPending} onClick={() => void me.refetch()}>
            刷新
          </Button>
        }
      />

      {me.isPending ? (
        <div className="card flex justify-center p-10">
          <Spinner label="加载账户…" />
        </div>
      ) : me.isError ? (
        <div className="card p-4">
          <ErrorState message={errorMessage(me.error)} onRetry={() => void me.refetch()} />
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Stat label="账户余额（元）" value={formatMoney(me.data.balance)} className={balanceClass(me.data.balance)} hint={me.data.balance < 0 ? '余额为负，已使用透支额度' : undefined} />
            <Stat label="透支额度（元）" value={formatMoney(me.data.credit_limit)} hint="余额可透支到 -额度" />
            <Stat label="本月支出（元）" value={formatMoney(me.data.month_spent ?? 0)} hint="行程 + 充电 + 罚金" />
            <div className="card p-5">
              <div className="flex items-center justify-between">
                <div className="text-xs text-ink-faint">月预算</div>
                <Badge color={ACCOUNT_STATUS_BADGE[me.data.status]}>{ACCOUNT_STATUS_LABEL[me.data.status]}</Badge>
              </div>
              <div className="mt-2">
                <BudgetBar spent={me.data.month_spent} budget={me.data.monthly_budget} />
              </div>
            </div>
          </div>

          <div className="card p-4">
            <div className="mb-3 flex items-center justify-between">
              <div className="flex items-center gap-1.5 text-sm font-medium text-ink">
                <Wallet size={15} className="text-ink-faint" />
                最近流水
              </div>
              <span className="text-xs text-ink-faint">最近 20 条 · 更新于 {formatDateTime(me.data.updated_at)}</span>
            </div>
            <TransactionTable data={me.data.transactions ?? []} hideOperator />
          </div>
        </>
      )}
    </div>
  )
}
