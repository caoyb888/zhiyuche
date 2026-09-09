import { keepPreviousData, useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { Pencil, ReceiptText } from 'lucide-react'
import { useState } from 'react'
import { billingKeys, getAccount, listAccountTransactions, type AccountTransactionParams } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { Account, TransactionType } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import DescriptionList from '../../../components/ui/DescriptionList'
import Drawer from '../../../components/ui/Drawer'
import ErrorState from '../../../components/ui/ErrorState'
import Pagination from '../../../components/ui/Pagination'
import Select from '../../../components/ui/Select'
import Spinner from '../../../components/ui/Spinner'
import { formatDateTime, formatMoney, text } from '../../../utils/format'
import { ACCOUNT_LEVEL_BADGE, ACCOUNT_LEVEL_LABEL, ACCOUNT_STATUS_BADGE, ACCOUNT_STATUS_LABEL, TRANSACTION_TYPE_OPTIONS, balanceClass, isTransactionType } from '../style'
import { BudgetBar } from './AccountTree'
import TransactionTable from './TransactionTable'

interface AccountDetailDrawerProps {
  id: string | null
  onClose: () => void
  onAdjust?: (a: Account) => void
  onEdit?: (a: Account) => void
}

const PAGE_SIZE = 20

function Body({ id, onAdjust, onEdit }: { id: string } & Pick<AccountDetailDrawerProps, 'onAdjust' | 'onEdit'>) {
  const detail = useQuery({ queryKey: billingKeys.account(id), queryFn: () => getAccount(id) })
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(PAGE_SIZE)
  const [type, setType] = useState<TransactionType | ''>('')
  const params: AccountTransactionParams = { page, pageSize, type: type || undefined }
  const txs = useQuery({ queryKey: billingKeys.accountTransactions(id, params), queryFn: () => listAccountTransactions(id, params), placeholderData: keepPreviousData })

  if (detail.isPending) {
    return (
      <div className="flex justify-center py-10">
        <Spinner label="加载账户…" />
      </div>
    )
  }
  if (detail.isError) return <ErrorState message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />
  const a = detail.data

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h4 className="text-base font-semibold text-slate-800 truncate">{a.owner_name}</h4>
            <Badge color={ACCOUNT_LEVEL_BADGE[a.level]}>{ACCOUNT_LEVEL_LABEL[a.level]}账户</Badge>
            <Badge color={ACCOUNT_STATUS_BADGE[a.status]}>{ACCOUNT_STATUS_LABEL[a.status]}</Badge>
          </div>
          {a.owner_sub && <div className="mt-0.5 text-xs text-slate-400">{a.owner_sub}</div>}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {onAdjust && (
            <Button variant="secondary" size="sm" icon={ReceiptText} onClick={() => onAdjust(a)}>
              调整
            </Button>
          )}
          {onEdit && (
            <Button variant="secondary" size="sm" icon={Pencil} onClick={() => onEdit(a)}>
              编辑
            </Button>
          )}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
        <div className="rounded-lg bg-slate-50 px-3 py-2">
          <div className="text-[11px] text-slate-400">余额（元）</div>
          <div className={clsx('mt-0.5 font-mono text-lg font-semibold', balanceClass(a.balance))}>{formatMoney(a.balance)}</div>
        </div>
        <div className="rounded-lg bg-slate-50 px-3 py-2">
          <div className="text-[11px] text-slate-400">透支额度（元）</div>
          <div className="mt-0.5 font-mono text-lg font-semibold text-slate-800">{formatMoney(a.credit_limit)}</div>
        </div>
        <div className="col-span-2 rounded-lg bg-slate-50 px-3 py-2 sm:col-span-1">
          <div className="text-[11px] text-slate-400">本月支出 / 月预算</div>
          <div className="mt-1">
            <BudgetBar spent={a.month_spent} budget={a.monthly_budget} />
          </div>
        </div>
      </div>

      <DescriptionList
        columns={2}
        items={[
          { label: '账户 ID', value: <span className="font-mono text-xs">{a.id}</span> },
          { label: '归属对象 ID', value: <span className="font-mono text-xs">{text(a.owner_id)}</span> },
          { label: '创建时间', value: formatDateTime(a.created_at) },
          { label: '更新时间', value: formatDateTime(a.updated_at) },
        ]}
      />

      <div>
        <div className="mb-2 flex items-center justify-between gap-2">
          <h4 className="text-sm font-semibold text-slate-700">账户流水</h4>
          <div className="w-40">
            <Select
              options={TRANSACTION_TYPE_OPTIONS}
              placeholder="全部类型"
              value={type}
              onChange={(e) => {
                const v = e.target.value
                setType(isTransactionType(v) ? v : '')
                setPage(1)
              }}
              aria-label="流水类型"
            />
          </div>
        </div>
        {txs.isError ? (
          <ErrorState size="sm" message={errorMessage(txs.error)} onRetry={() => void txs.refetch()} />
        ) : (
          <>
            <TransactionTable data={txs.data?.items} loading={txs.isFetching} />
            <Pagination
              className="mt-3"
              page={page}
              pageSize={pageSize}
              total={txs.data?.total ?? 0}
              pageSizeOptions={[10, 20, 50]}
              onChange={(p, s) => {
                setPage(p)
                setPageSize(s)
              }}
            />
          </>
        )}
      </div>
    </div>
  )
}

/** 账户详情抽屉：信息 + 流水（由 URL ?id= 驱动） */
export default function AccountDetailDrawer({ id, onClose, onAdjust, onEdit }: AccountDetailDrawerProps) {
  return (
    <Drawer open={id !== null} onClose={onClose} title="账户详情" description="余额、额度、本月支出与流水明细" width="lg">
      {id && <Body key={id} id={id} onAdjust={onAdjust} onEdit={onEdit} />}
    </Drawer>
  )
}
