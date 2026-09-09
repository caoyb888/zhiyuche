import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { billingKeys, listTransactions, type TransactionFilter, type TransactionListParams } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { AccountLevel, TransactionType } from '../../../api/types'
import DateTimeInput from '../../../components/ui/DateTimeInput'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import FilterBar from '../../../components/ui/FilterBar'
import Pagination from '../../../components/ui/Pagination'
import Select from '../../../components/ui/Select'
import { localToRFC3339 } from '../../../utils/format'
import { ACCOUNT_LEVEL_OPTIONS, TRANSACTION_TYPE_OPTIONS, isAccountLevel, isTransactionType } from '../style'
import TransactionTable from './TransactionTable'

interface Filters {
  keyword: string
  level: AccountLevel | ''
  type: TransactionType | ''
  from: string
  to: string
}

const EMPTY_FILTERS: Filters = { keyword: '', level: '', type: '', from: '', to: '' }
const DEFAULT_PAGE_SIZE = 20

function toFilter(f: Filters): TransactionFilter {
  return {
    keyword: f.keyword || undefined,
    level: f.level || undefined,
    type: f.type || undefined,
    from: localToRFC3339(f.from),
    to: localToRFC3339(f.to),
  }
}

/** 全部流水页签：按账户级别 / 类型 / 时间 / 关键词筛选的分页表 */
export default function TransactionsTab() {
  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)

  const params: TransactionListParams = { ...toFilter(applied), page, pageSize }
  const list = useQuery({ queryKey: billingKeys.transactions(params), queryFn: () => listTransactions(params), placeholderData: keepPreviousData })

  const search = () => {
    setApplied(draft)
    setPage(1)
  }
  const reset = () => {
    setDraft(EMPTY_FILTERS)
    setApplied(EMPTY_FILTERS)
    setPage(1)
  }

  return (
    <div className="space-y-4">
      <FilterBar keyword={draft.keyword} onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))} keywordPlaceholder="账户名 / 备注 / 单号" onSearch={search} onReset={reset} loading={list.isFetching} className="!p-0 !border-0 !shadow-none !rounded-none">
        <div className="w-full sm:w-32">
          <Select
            options={ACCOUNT_LEVEL_OPTIONS}
            placeholder="全部级别"
            value={draft.level}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, level: isAccountLevel(v) ? v : '' }))
            }}
            aria-label="账户级别"
          />
        </div>
        <div className="w-full sm:w-36">
          <Select
            options={TRANSACTION_TYPE_OPTIONS}
            placeholder="全部类型"
            value={draft.type}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, type: isTransactionType(v) ? v : '' }))
            }}
            aria-label="流水类型"
          />
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-full sm:w-48">
            <DateTimeInput value={draft.from} onChange={(v) => setDraft((d) => ({ ...d, from: v }))} aria-label="开始时间" />
          </div>
          <span className="text-xs text-slate-400">至</span>
          <div className="w-full sm:w-48">
            <DateTimeInput value={draft.to} onChange={(v) => setDraft((d) => ({ ...d, to: v }))} aria-label="结束时间" />
          </div>
        </div>
      </FilterBar>

      {list.isError ? (
        <ErrorState message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
      ) : (
        <>
          <TransactionTable data={list.data?.items} loading={list.isFetching} showAccount empty={<Empty title="没有匹配的流水" description="调整筛选条件后重试" />} />
          <Pagination
            page={page}
            pageSize={pageSize}
            total={list.data?.total ?? 0}
            onChange={(p, s) => {
              setPage(p)
              setPageSize(s)
            }}
          />
        </>
      )}
    </div>
  )
}
