import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import clsx from 'clsx'
import { Calculator, Play } from 'lucide-react'
import { Controller, useForm } from 'react-hook-form'
import { simulateBillingRule } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { BillingLine, BillingResult, BillingRuleDoc } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import DateTimeInput from '../../../components/ui/DateTimeInput'
import Empty from '../../../components/ui/Empty'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select from '../../../components/ui/Select'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { formatMoney, formatNumber } from '../../../utils/format'
import { TRIP_TYPE_OPTIONS } from '../../approval/style'
import { ACCOUNT_LEVEL_LABEL, LINE_KIND_BADGE, LINE_KIND_LABEL } from '../style'
import { defaultSimulateValues, simulateSchema, simulateToTrip, type SimulateFormValues } from './schemas'

interface SimulatePanelProps {
  /** 当前编辑中的规则文档；表单校验失败时为 null */
  doc: BillingRuleDoc | null
  ruleId?: string
  dirty: boolean
}

function amountClass(line: BillingLine): string {
  if (line.kind === 'penalty') return 'text-red-600'
  if (line.amount < 0) return 'text-emerald-600'
  return 'text-slate-800'
}

const lineColumns: Column<BillingLine>[] = [
  {
    key: 'item',
    title: '项目',
    render: (l) => (
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="font-medium text-slate-800">{l.item}</span>
        <Badge color={LINE_KIND_BADGE[l.kind]}>{LINE_KIND_LABEL[l.kind]}</Badge>
        {l.note && <span className="text-xs text-slate-400">{l.note}</span>}
      </div>
    ),
  },
  { key: 'qty', title: '数量', align: 'right', render: (l) => <span className="font-mono text-xs">{formatNumber(l.qty, l.unit === '次' || l.unit === '天' ? 0 : 2)}</span> },
  { key: 'unit', title: '单位', render: (l) => <span className="text-xs text-slate-500">{l.unit}</span> },
  {
    key: 'unit_price',
    title: '单价',
    align: 'right',
    render: (l) => <span className="font-mono text-xs">{l.kind === 'multiplier' ? `×${formatNumber(l.unit_price, 2)}` : l.kind === 'surcharge' ? `${formatMoney(l.unit_price)} 元 基数` : `${formatMoney(l.unit_price)} 元/${l.unit}`}</span>,
  },
  { key: 'amount', title: '金额（元）', align: 'right', render: (l) => <span className={clsx('font-mono font-medium', amountClass(l))}>{l.amount < 0 ? `-${formatMoney(Math.abs(l.amount))}` : formatMoney(l.amount)}</span> },
]

function Stat({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className="rounded-lg bg-slate-50 px-3 py-2">
      <div className="text-[11px] text-slate-400">{label}</div>
      <div className={clsx('mt-0.5 font-mono text-sm font-medium text-slate-800', className)}>{value}</div>
    </div>
  )
}

function ResultView({ r }: { r: BillingResult }) {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Stat label="合计（元）" value={formatMoney(r.total)} className="text-base text-brand-700" />
        <Stat label="基础费（未乘系数）" value={formatMoney(r.base)} />
        <Stat label="时段系数" value={r.multiplier === 1 ? '无（×1）' : `×${formatNumber(r.multiplier, 2)}`} />
        <Stat label="日封顶" value={r.cap_applied ? '已触发' : '未触发'} className={r.cap_applied ? 'text-emerald-600' : undefined} />
        <Stat label="低电附加" value={formatMoney(r.surcharge)} />
        <Stat label="电费" value={formatMoney(r.electricity)} />
        <Stat label="罚金" value={formatMoney(r.penalty)} className={r.penalty > 0 ? 'text-red-600' : undefined} />
        <Stat label="扣费账户" value={`${ACCOUNT_LEVEL_LABEL[r.attribution]}账户`} />
      </div>
      <div>
        <div className="mb-2 flex items-center justify-between">
          <span className="text-xs font-medium text-slate-500">账单明细 · {r.rule_name}</span>
          <span className="text-[11px] text-slate-400">罚金红色单列；减免（如日封顶）为负数绿色</span>
        </div>
        <Table columns={lineColumns} data={r.lines} rowKey={(l) => `${l.kind}:${l.item}`} empty={<Empty size="sm" title="无费用明细" description="基础费率与其他项均未产生费用" />} />
      </div>
    </div>
  )
}

/** 模拟计算：用当前编辑中的规则（内联提交，不需要先保存）对一次假想行程出账单 */
export default function SimulatePanel({ doc, ruleId, dirty }: SimulatePanelProps) {
  const toast = useToast()
  const form = useForm<SimulateFormValues>({ resolver: zodResolver(simulateSchema), defaultValues: defaultSimulateValues() })

  // 表单有校验错误时：未修改的已保存规则退化为按 rule_id 计算；否则不能模拟
  const useInline = doc !== null
  const canRun = useInline || (Boolean(ruleId) && !dirty)

  const run = useMutation({
    mutationFn: (v: SimulateFormValues) => {
      const trip = simulateToTrip(v)
      return simulateBillingRule(doc !== null ? { rule: doc, trip } : { rule_id: ruleId, trip })
    },
    onError: (e) => toast.error('模拟计算失败', errorMessage(e)),
  })

  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,20rem)_minmax(0,1fr)] items-start">
      <div className="rounded-xl border border-slate-100 p-4">
        <h4 className="text-sm font-semibold text-slate-700">行程参数</h4>
        <p className="mt-0.5 mb-4 text-xs text-slate-400">{useInline ? '按当前编辑中的规则计算（含未保存修改）' : canRun ? '表单有校验错误，按已保存的规则计算' : '表单存在校验错误，请先修正后再模拟'}</p>
        <Form form={form} onSubmit={(v) => run.mutate(v)}>
          <FormField name="trip_type" label="用车类型">
            <Select options={TRIP_TYPE_OPTIONS} {...form.register('trip_type')} />
          </FormField>
          <FormField name="start_at" label="开始时间" required>
            {({ id, invalid }) => <Controller control={form.control} name="start_at" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
          </FormField>
          <FormField name="end_at" label="结束时间" required>
            {({ id, invalid }) => <Controller control={form.control} name="end_at" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
          </FormField>
          <div className="grid grid-cols-2 gap-3">
            <FormField name="distance_km" label="里程（km）" required>
              <Input type="number" inputMode="decimal" min={0} step="0.1" {...form.register('distance_km')} />
            </FormField>
            <FormField name="energy_kwh" label="耗电（kWh）">
              <Input type="number" inputMode="decimal" min={0} step="0.1" {...form.register('energy_kwh')} />
            </FormField>
            <FormField name="end_soc" label="还车 SOC（%）">
              <Input type="number" inputMode="numeric" min={0} max={100} step="1" placeholder="留空不判定" {...form.register('end_soc')} />
            </FormField>
            <FormField name="overspeed_events" label="超速次数">
              <Input type="number" inputMode="numeric" min={0} step="1" {...form.register('overspeed_events')} />
            </FormField>
            <FormField name="harsh_events" label="急加减速次数">
              <Input type="number" inputMode="numeric" min={0} step="1" {...form.register('harsh_events')} />
            </FormField>
          </div>
          <FormField name="planned_end" label="计划还车时间" hint="留空不判定超时还车">
            {({ id, invalid }) => <Controller control={form.control} name="planned_end" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
          </FormField>
          <Button type="submit" icon={Play} block loading={run.isPending} disabled={!canRun}>
            开始模拟
          </Button>
        </Form>
      </div>

      <div className="min-w-0 rounded-xl border border-slate-100 p-4">
        {run.data ? (
          <ResultView r={run.data} />
        ) : (
          <Empty icon={Calculator} title="尚未计算" description="填写左侧行程参数后点击“开始模拟”，结果不会写入任何账户" />
        )}
      </div>
    </div>
  )
}
