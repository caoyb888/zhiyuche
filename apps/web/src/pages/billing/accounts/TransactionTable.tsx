import clsx from 'clsx'
import { ExternalLink } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import type { AccountTransaction } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Empty from '../../../components/ui/Empty'
import Table, { type Column } from '../../../components/ui/Table'
import { formatDateTime, formatMoney, text } from '../../../utils/format'
import { ACCOUNT_LEVEL_BADGE, ACCOUNT_LEVEL_LABEL, TRANSACTION_TYPE_BADGE, TRANSACTION_TYPE_LABEL, formatSignedMoney, refLink, signedMoneyClass } from '../style'

interface TransactionTableProps {
  data: AccountTransaction[] | undefined
  loading?: boolean
  /** 显示账户列（全部流水 / 跨账户视图） */
  showAccount?: boolean
  /** 隐藏操作人列（个人视图） */
  hideOperator?: boolean
  empty?: ReactNode
  className?: string
}

/** 关联单号：行程 / 充电事务跳转到对应页面 */
export function RefCell({ tx }: { tx: AccountTransaction }) {
  const label = tx.ref_no || (tx.ref_id ? `${tx.ref_id.slice(0, 8)}…` : '')
  if (!label) return <span className="text-ink-disabled">—</span>
  const href = refLink(tx.ref_type, tx.ref_id)
  if (!href) return <span className="font-mono text-xs text-ink">{label}</span>
  return (
    <Link to={href} className="inline-flex items-center gap-1 font-mono text-xs text-brand-600 hover:underline" title={tx.ref_type === 'trip' ? '查看行程' : '查看充电事务'}>
      {label}
      <ExternalLink size={11} />
    </Link>
  )
}

/** 流水表（账户详情 / 全部流水 / 我的账户共用） */
export default function TransactionTable({ data, loading, showAccount = false, hideOperator = false, empty, className }: TransactionTableProps) {
  const columns: Column<AccountTransaction>[] = [
    { key: 'created_at', title: '时间', width: 140, render: (t) => <span className="text-xs text-ink whitespace-nowrap">{formatDateTime(t.created_at)}</span> },
  ]
  if (showAccount) {
    columns.push({
      key: 'account',
      title: '账户',
      render: (t) => (
        <div className="flex items-center gap-1.5">
          {t.account_level && <Badge color={ACCOUNT_LEVEL_BADGE[t.account_level]}>{ACCOUNT_LEVEL_LABEL[t.account_level]}</Badge>}
          <span className="text-sm text-ink-strong">{text(t.account_name)}</span>
        </div>
      ),
    })
  }
  columns.push(
    { key: 'type', title: '类型', render: (t) => <Badge color={TRANSACTION_TYPE_BADGE[t.type]}>{TRANSACTION_TYPE_LABEL[t.type]}</Badge> },
    { key: 'amount', title: '金额（元）', align: 'right', render: (t) => <span className={clsx('font-mono font-medium', signedMoneyClass(t.amount))}>{formatSignedMoney(t.amount)}</span> },
    { key: 'balance_after', title: '余额（元）', align: 'right', render: (t) => <span className={clsx('font-mono text-xs', t.balance_after < 0 ? 'text-danger-200' : 'text-ink')}>{formatMoney(t.balance_after)}</span> },
    { key: 'ref', title: '关联单号', render: (t) => <RefCell tx={t} /> },
    { key: 'remark', title: '备注', render: (t) => <span className="text-xs text-ink-muted break-words">{text(t.remark)}</span> },
  )
  if (!hideOperator) {
    columns.push({ key: 'created_by_name', title: '操作人', render: (t) => <span className="text-xs text-ink-muted">{t.created_by_name || '系统'}</span> })
  }

  return <Table columns={columns} data={data} rowKey={(t) => String(t.id)} loading={loading} className={className} empty={empty ?? <Empty size="sm" title="暂无流水" />} />
}
