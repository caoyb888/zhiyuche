import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link2, Link2Off, Pencil, Plus, ShieldAlert, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { cardKeys, deleteCard, listCards, reportCardLoss, unbindCard, type CardFilter, type CardListParams } from '../../../api/cards'
import { errorMessage } from '../../../api/client'
import type { Card, CardStatus } from '../../../api/types'
import Can from '../../../components/Can'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import FilterBar from '../../../components/ui/FilterBar'
import PageHeader from '../../../components/ui/PageHeader'
import Pagination from '../../../components/ui/Pagination'
import Select from '../../../components/ui/Select'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { usePermission } from '../../../hooks/usePermission'
import { formatDateTime, text } from '../../../utils/format'
import BindUserModal from './BindUserModal'
import CardDrawer, { type CardDrawerState } from './CardDrawer'
import { boundOptions, cardStatusColor, cardStatusLabel, cardStatusOptions, isCardStatus, parseBool } from './schemas'

interface Filters {
  keyword: string
  status: CardStatus | ''
  bound: string
}

const EMPTY_FILTERS: Filters = { keyword: '', status: '', bound: '' }
const DEFAULT_PAGE_SIZE = 20

function toFilter(f: Filters): CardFilter {
  return { keyword: f.keyword || undefined, status: f.status || undefined, bound: parseBool(f.bound) }
}

export default function CardsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()

  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<string | undefined>(undefined)

  const [drawer, setDrawer] = useState<CardDrawerState>(null)
  const [bindTarget, setBindTarget] = useState<Card | null>(null)
  const [unbindTarget, setUnbindTarget] = useState<Card | null>(null)
  const [lossTarget, setLossTarget] = useState<Card | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Card | null>(null)

  const params: CardListParams = { ...toFilter(applied), page, pageSize, sort }
  const list = useQuery({
    queryKey: cardKeys.list(params),
    queryFn: () => listCards(params),
    placeholderData: keepPreviousData,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: cardKeys.all })

  const unbind = useMutation({
    mutationFn: (id: string) => unbindCard(id),
    onSuccess: () => {
      toast.success('已解绑持卡人')
      setUnbindTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('解绑失败', errorMessage(e)),
  })

  const loss = useMutation({
    mutationFn: (id: string) => reportCardLoss(id),
    onSuccess: () => {
      toast.success('已挂失', '该卡刷卡取车将被拒绝')
      setLossTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('挂失失败', errorMessage(e)),
  })

  const remove = useMutation({
    mutationFn: (id: string) => deleteCard(id),
    onSuccess: () => {
      toast.success('卡片已删除')
      setDeleteTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  const search = () => {
    setApplied(draft)
    setPage(1)
  }
  const reset = () => {
    setDraft(EMPTY_FILTERS)
    setApplied(EMPTY_FILTERS)
    setPage(1)
    setSort(undefined)
  }

  const canUpdate = can('asset:card:update')
  const canDelete = can('asset:card:delete')

  const columns: Column<Card>[] = [
    { key: 'card_uid', title: '卡片 UID', sortable: true, render: (c) => <span className="font-mono text-sm font-medium text-ink-strong">{c.card_uid}</span> },
    {
      key: 'user_name',
      title: '持卡人',
      render: (c) =>
        c.user_id ? (
          <div>
            <div className="text-ink-strong">{text(c.user_name)}</div>
            {c.username && <div className="text-xs text-ink-faint">@{c.username}</div>}
          </div>
        ) : (
          <span className="text-xs text-ink-faint">未绑定</span>
        ),
    },
    { key: 'status', title: '状态', sortable: true, render: (c) => <Badge color={cardStatusColor[c.status]}>{cardStatusLabel[c.status]}</Badge> },
    { key: 'issued_at', title: '发卡时间', sortable: true, render: (c) => <span className="text-xs text-ink-muted">{formatDateTime(c.issued_at)}</span> },
    { key: 'remark', title: '备注', render: (c) => <span className="line-clamp-1 max-w-xs text-xs text-ink-muted">{text(c.remark)}</span> },
    { key: 'created_at', title: '创建时间', sortable: true, render: (c) => <span className="text-xs text-ink-muted">{formatDateTime(c.created_at)}</span> },
  ]

  const renderActions = (c: Card) => (
    <div className="flex items-center justify-end gap-0.5">
      {canUpdate && <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="编辑" aria-label="编辑" onClick={() => setDrawer({ mode: 'edit', card: c })} />}
      {canUpdate &&
        (c.user_id ? (
          <Button variant="ghost" size="sm" icon={Link2Off} className="!px-2" title="解绑持卡人" aria-label="解绑持卡人" onClick={() => setUnbindTarget(c)} />
        ) : (
          <Button variant="ghost" size="sm" icon={Link2} className="!px-2" title="绑定持卡人" aria-label="绑定持卡人" onClick={() => setBindTarget(c)} />
        ))}
      {canUpdate && c.status === 'active' && (
        <Button variant="ghost" size="sm" icon={ShieldAlert} className="!px-2 text-warn-200 hover:bg-warn-500/10" title="挂失" aria-label="挂失" onClick={() => setLossTarget(c)} />
      )}
      {canDelete && (
        <Button variant="ghost" size="sm" icon={Trash2} className="!px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200" title="删除" aria-label="删除" onClick={() => setDeleteTarget(c)} />
      )}
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="NFC 卡"
        description="员工刷卡取车用的 NFC 卡；挂失 / 停用后刷卡被拒"
        extra={
          <Can perm="asset:card:create">
            <Button icon={Plus} onClick={() => setDrawer({ mode: 'create' })}>
              发卡
            </Button>
          </Can>
        }
      />

      <FilterBar keyword={draft.keyword} onKeywordChange={(v) => setDraft((d) => ({ ...d, keyword: v }))} keywordPlaceholder="卡号 / 持卡人姓名 / 用户名" onSearch={search} onReset={reset} loading={list.isFetching}>
        <div className="w-full sm:w-36">
          <Select
            options={cardStatusOptions}
            placeholder="全部状态"
            value={draft.status}
            onChange={(e) => {
              const v = e.target.value
              setDraft((d) => ({ ...d, status: isCardStatus(v) ? v : '' }))
            }}
            aria-label="状态"
          />
        </div>
        <div className="w-full sm:w-40">
          <Select options={boundOptions} placeholder="绑定情况" value={draft.bound} onChange={(e) => setDraft((d) => ({ ...d, bound: e.target.value }))} aria-label="是否绑定" />
        </div>
      </FilterBar>

      <div className="card p-4">
        {list.isError ? (
          <ErrorState message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
        ) : (
          <>
            <Table
              columns={columns}
              data={list.data?.items}
              rowKey="id"
              loading={list.isFetching}
              sort={sort}
              onSortChange={(s) => {
                setSort(s)
                setPage(1)
              }}
              actions={canUpdate || canDelete ? { render: renderActions, width: 170 } : undefined}
              empty={<Empty title="没有匹配的卡片" description="调整筛选条件，或发卡" />}
            />
            <Pagination
              className="mt-4"
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

      <CardDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSaved={() => {
          setDrawer(null)
          void invalidate()
        }}
      />
      <BindUserModal
        card={bindTarget}
        onClose={() => setBindTarget(null)}
        onSaved={() => {
          setBindTarget(null)
          void invalidate()
        }}
      />
      <ConfirmDialog
        open={unbindTarget !== null}
        title={`解绑卡片「${unbindTarget?.card_uid ?? ''}」的持卡人？`}
        description={`将与 ${unbindTarget?.user_name ?? '当前持卡人'} 解除绑定，解绑后该卡无法刷卡取车。`}
        confirmText="解绑"
        loading={unbind.isPending}
        onConfirm={() => {
          if (unbindTarget) unbind.mutate(unbindTarget.id)
        }}
        onCancel={() => setUnbindTarget(null)}
      />
      <ConfirmDialog
        open={lossTarget !== null}
        danger
        title={`挂失卡片「${lossTarget?.card_uid ?? ''}」？`}
        description="挂失后状态变为「已挂失」，刷卡取车立即被拒；找回后可在编辑中恢复为正常。"
        confirmText="挂失"
        loading={loss.isPending}
        onConfirm={() => {
          if (lossTarget) loss.mutate(lossTarget.id)
        }}
        onCancel={() => setLossTarget(null)}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除卡片「${deleteTarget?.card_uid ?? ''}」？`}
        description="软删除；删除后该卡不可再用。"
        confirmText="删除"
        loading={remove.isPending}
        onConfirm={() => {
          if (deleteTarget) remove.mutate(deleteTarget.id)
        }}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
