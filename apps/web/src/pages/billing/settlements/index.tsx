import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { CheckCircle2, Download, Eye, Printer, RefreshCw, Sparkles } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { billingKeys, confirmSettlement, exportSettlements, generateSettlement, getSettlement, listSettlementPeriods, listSettlements } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { Settlement } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import Input from '../../../components/ui/Input'
import PageHeader from '../../../components/ui/PageHeader'
import Select, { type SelectOption } from '../../../components/ui/Select'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { usePermission } from '../../../hooks/usePermission'
import { useAuthStore } from '../../../store/auth'
import { saveBlob } from '../../../utils/download'
import { formatDateTime, formatMoney } from '../../../utils/format'
import { PERIOD_STATUS_LABEL, SETTLEMENT_STATUS_BADGE, SETTLEMENT_STATUS_LABEL, budgetUsage, usageBarClass } from '../style'
import { openSettlementPrint } from './print'
import SettlementDetailDrawer from './SettlementDetailDrawer'

const PERIOD_RULE = /^\d{4}-\d{2}$/

function currentPeriod(): string {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

/** 企业汇总行置顶，其余按部门名 */
function sortSettlements(list: Settlement[]): Settlement[] {
  return [...list].sort((a, b) => {
    if (!a.dept_id && b.dept_id) return -1
    if (a.dept_id && !b.dept_id) return 1
    return (a.dept_name ?? '').localeCompare(b.dept_name ?? '', 'zh-CN')
  })
}

function UsageCell({ s }: { s: Settlement }) {
  const pct = budgetUsage(s.total, s.budget)
  if (pct === null) return <span className="text-xs text-ink-faint">未设预算</span>
  return (
    <div className="min-w-[7rem]">
      <div className="flex items-center justify-between text-[11px]">
        <span className={clsx(pct >= 100 ? 'font-medium text-danger-200' : 'text-ink-muted')}>{pct.toFixed(0)}%</span>
        <span className="text-ink-faint">/ {formatMoney(s.budget)}</span>
      </div>
      <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-surface-4">
        <div className={clsx('h-full rounded-full', usageBarClass(pct))} style={{ width: `${Math.min(100, pct)}%` }} />
      </div>
    </div>
  )
}

/** 月度结算（/billing/settlements）：期选择、生成 / 重算、企业汇总 + 各部门结算单 */
export default function BillingSettlementsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const profile = useAuthStore((s) => s.profile)
  const viewTenant = useAuthStore((s) => s.viewTenant)
  const [searchParams, setSearchParams] = useSearchParams()

  const canGenerate = can('billing:settlement:generate')
  const canConfirm = can('billing:settlement:confirm')
  const canExport = can('billing:settlement:export')

  const periods = useQuery({ queryKey: billingKeys.settlementPeriods, queryFn: listSettlementPeriods })
  const periodList = useMemo(() => periods.data ?? [], [periods.data])

  const urlPeriod = searchParams.get('period')
  const detailId = searchParams.get('id')
  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set(key, value)
    else next.delete(key)
    setSearchParams(next, { replace: true })
  }

  // 当前期：URL 优先；否则期列表第一项（最近一期）；期列表不可用 / 为空时用手动选择的月份（缺省本月）
  const [manualPeriod, setManualPeriod] = useState(currentPeriod())
  const manualMode = periods.isError || (periods.isSuccess && periodList.length === 0)
  const period = urlPeriod && PERIOD_RULE.test(urlPeriod) ? urlPeriod : manualMode ? manualPeriod : (periodList[0]?.period ?? '')
  useEffect(() => {
    const first = periodList[0]?.period
    if (urlPeriod || !first) return
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('period', first)
        return next
      },
      { replace: true },
    )
  }, [periodList, urlPeriod, setSearchParams])

  const periodInfo = periodList.find((p) => p.period === period)
  const periodOptions: SelectOption[] = periodList.map((p) => ({ value: p.period, label: `${p.period} · ${PERIOD_STATUS_LABEL[p.status]}${p.total !== undefined && p.status !== 'none' ? ` ¥${formatMoney(p.total)}` : ''}` }))
  if (period && !periodOptions.some((o) => o.value === period)) periodOptions.unshift({ value: period, label: `${period}` })

  const list = useQuery({
    queryKey: billingKeys.settlements({ period }),
    queryFn: () => listSettlements({ period }),
    enabled: period !== '',
    placeholderData: keepPreviousData,
  })
  const rows = useMemo(() => sortSettlements(list.data ?? []), [list.data])
  const enterprise = rows.find((s) => !s.dept_id)
  const confirmed = enterprise?.status === 'confirmed' || periodInfo?.status === 'confirmed'
  const hasDraft = rows.length > 0 && !confirmed

  const invalidate = () => queryClient.invalidateQueries({ queryKey: [...billingKeys.all, 'settlements'] })

  const [generateOpen, setGenerateOpen] = useState(false)
  const [confirmTarget, setConfirmTarget] = useState<Settlement | null>(null)

  const generate = useMutation({
    mutationFn: () => generateSettlement(period),
    onSuccess: (result) => {
      toast.success(hasDraft ? '结算单已重算' : '结算单已生成', `${period} 共 ${result.length} 条（含企业汇总）`)
      setGenerateOpen(false)
      void invalidate()
    },
    onError: (e) => toast.error('生成失败', errorMessage(e)),
  })

  const confirm = useMutation({
    mutationFn: (s: Settlement) => confirmSettlement(s.id),
    onSuccess: (saved) => {
      toast.success('结算单已确认', saved.dept_id ? `${saved.dept_name ?? '部门'} · ${saved.period}` : `企业汇总 · ${saved.period}，该期全部结算单已确认`)
      setConfirmTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('确认失败', errorMessage(e)),
  })

  const exporting = useMutation({
    mutationFn: (s: Settlement) => exportSettlements(s.period, s.dept_id ?? undefined),
    onSuccess: (file) => {
      saveBlob(file.blob, file.filename)
      toast.success('已开始下载', file.filename)
    },
    onError: (e) => toast.error('导出失败', errorMessage(e)),
  })

  const print = async (s: Settlement) => {
    try {
      // 列表行没有明细：打印前取详情
      const full = s.lines ? s : await queryClient.fetchQuery({ queryKey: billingKeys.settlement(s.id), queryFn: () => getSettlement(s.id) })
      const ok = openSettlementPrint(full, { tenantName: viewTenant?.name ?? profile?.tenant.name ?? '智御系统', printedBy: profile?.name ?? '' })
      if (!ok) toast.warning('打印窗口被浏览器拦截', '请允许本站弹出窗口后重试')
    } catch (e) {
      toast.error('打印失败', errorMessage(e))
    }
  }

  const columns: Column<Settlement>[] = [
    {
      key: 'dept_name',
      title: '结算对象',
      render: (s) => (
        <div className="flex items-center gap-2">
          <span className={clsx('text-sm', s.dept_id ? 'text-ink-strong' : 'font-semibold text-ink-strong')}>{s.dept_id ? (s.dept_name ?? '未命名部门') : '企业汇总'}</span>
          {!s.dept_id && <Badge color="purple">全企业</Badge>}
        </div>
      ),
    },
    { key: 'trip_count', title: '行程数', align: 'right', render: (s) => <span className="font-mono text-xs">{s.trip_count}</span> },
    { key: 'trip_cost', title: '行程费', align: 'right', render: (s) => <span className={clsx('font-mono', !s.dept_id && 'font-semibold')}>{formatMoney(s.trip_cost)}</span> },
    { key: 'charge_count', title: '充电数', align: 'right', render: (s) => <span className="font-mono text-xs">{s.charge_count}</span> },
    { key: 'charge_cost', title: '充电费', align: 'right', render: (s) => <span className={clsx('font-mono', !s.dept_id && 'font-semibold')}>{formatMoney(s.charge_cost)}</span> },
    { key: 'penalty', title: '罚金', align: 'right', render: (s) => <span className={clsx('font-mono', s.penalty > 0 ? 'text-danger-200' : 'text-ink-muted', !s.dept_id && 'font-semibold')}>{formatMoney(s.penalty)}</span> },
    { key: 'total', title: '合计（元）', align: 'right', render: (s) => <span className={clsx('font-mono font-semibold', !s.dept_id ? 'text-brand-300' : 'text-ink-strong')}>{formatMoney(s.total)}</span> },
    { key: 'budget', title: '预算 / 使用率', render: (s) => <UsageCell s={s} /> },
    { key: 'status', title: '状态', render: (s) => <Badge color={SETTLEMENT_STATUS_BADGE[s.status]}>{SETTLEMENT_STATUS_LABEL[s.status]}</Badge> },
  ]

  const renderActions = (s: Settlement) => (
    <div className="flex items-center justify-end gap-0.5">
      <Button variant="ghost" size="sm" icon={Eye} className="!px-2" title="查看明细" aria-label="查看明细" onClick={() => setParam('id', s.id)} />
      {canConfirm && s.status === 'draft' && <Button variant="ghost" size="sm" icon={CheckCircle2} className="!px-2 text-ev-200 hover:bg-ev-500/10" title="确认" aria-label="确认" onClick={() => setConfirmTarget(s)} />}
      {canExport && <Button variant="ghost" size="sm" icon={Download} className="!px-2" title="导出 xlsx" aria-label="导出" disabled={exporting.isPending} onClick={() => exporting.mutate(s)} />}
      <Button variant="ghost" size="sm" icon={Printer} className="!px-2" title="打印" aria-label="打印" onClick={() => void print(s)} />
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="月度结算"
        description="按期生成企业汇总与各部门结算单；确认后不可重算，可导出 xlsx 或打印存档"
        extra={
          <>
            <Button variant="secondary" icon={RefreshCw} loading={list.isFetching && !list.isPending} onClick={() => void Promise.all([periods.refetch(), list.refetch()])}>
              刷新
            </Button>
            {canGenerate && (
              <Button icon={Sparkles} disabled={!period || confirmed} title={confirmed ? '已确认的期不可重算' : undefined} onClick={() => setGenerateOpen(true)}>
                {hasDraft ? '重算本期' : '生成结算单'}
              </Button>
            )}
          </>
        }
      />

      <div className="card p-4 flex flex-col gap-3 sm:flex-row sm:items-center">
        <span className="text-sm font-medium text-ink shrink-0">结算期</span>
        {periods.isError || (periods.isSuccess && periodList.length === 0) ? (
          <div className="flex items-center gap-2">
            <div className="w-44">
              <Input
                type="month"
                value={manualPeriod}
                onChange={(e) => {
                  setManualPeriod(e.target.value)
                  setParam('period', e.target.value || null)
                }}
                aria-label="结算期"
              />
            </div>
            <span className="text-xs text-ink-faint">{periods.isError ? `期列表暂不可用（${errorMessage(periods.error)}），可手动选择月份` : '暂无可结算的月份，可手动选择'}</span>
          </div>
        ) : (
          <div className="w-full sm:w-72">
            <Select options={periodOptions} value={period} onChange={(e) => setParam('period', e.target.value || null)} disabled={periods.isPending} placeholder={periods.isPending ? '加载中…' : undefined} aria-label="结算期" />
          </div>
        )}
        {periodInfo && (
          <div className="flex items-center gap-2 text-xs text-ink-muted">
            <Badge color={periodInfo.status === 'none' ? 'gray' : SETTLEMENT_STATUS_BADGE[periodInfo.status]}>{PERIOD_STATUS_LABEL[periodInfo.status]}</Badge>
            {enterprise?.confirmed_at && <span>确认于 {formatDateTime(enterprise.confirmed_at)}{enterprise.confirmed_by_name ? ` · ${enterprise.confirmed_by_name}` : ''}</span>}
          </div>
        )}
      </div>

      <div className="card p-4">
        {!period ? (
          <Empty title="请选择结算期" />
        ) : list.isError ? (
          <ErrorState message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
        ) : (
          <Table
            columns={columns}
            data={rows}
            rowKey="id"
            loading={list.isFetching}
            onRowClick={(s) => setParam('id', s.id)}
            actions={{ render: renderActions, width: 150 }}
            empty={
              <Empty
                title={`${period} 尚未生成结算单`}
                description={canGenerate ? '点击"生成结算单"汇总本期行程、充电与罚金' : '等待管理员生成'}
                action={
                  canGenerate && (
                    <Button size="sm" icon={Sparkles} onClick={() => setGenerateOpen(true)}>
                      生成结算单
                    </Button>
                  )
                }
              />
            }
          />
        )}
      </div>

      <SettlementDetailDrawer
        id={detailId}
        onClose={() => setParam('id', null)}
        canConfirm={canConfirm}
        canExport={canExport}
        exporting={exporting.isPending}
        onConfirm={(s) => setConfirmTarget(s)}
        onExport={(s) => exporting.mutate(s)}
        onPrint={(s) => void print(s)}
      />

      <ConfirmDialog
        open={generateOpen}
        title={hasDraft ? `重算 ${period} 结算单？` : `生成 ${period} 结算单？`}
        description={hasDraft ? '将先补算未计费行程，再重建本期草稿与全部明细；已有草稿会被覆盖。' : '汇总本期已结束的行程、充电事务与罚金，生成企业汇总与各部门结算单（草稿）。'}
        confirmText={hasDraft ? '重算' : '生成'}
        danger={hasDraft}
        loading={generate.isPending}
        onConfirm={() => generate.mutate()}
        onCancel={() => setGenerateOpen(false)}
      />
      <ConfirmDialog
        open={confirmTarget !== null}
        title={confirmTarget?.dept_id ? `确认「${confirmTarget.dept_name ?? '部门'}」${confirmTarget.period} 结算单？` : `确认 ${confirmTarget?.period ?? ''} 企业汇总结算单？`}
        description={confirmTarget?.dept_id ? '确认后该部门结算单不可修改。' : '企业汇总确认即视为本期全部结算单确认，之后不可重算。'}
        confirmText="确认结算"
        loading={confirm.isPending}
        onConfirm={() => {
          if (confirmTarget) confirm.mutate(confirmTarget)
        }}
        onCancel={() => setConfirmTarget(null)}
      />
    </div>
  )
}
