import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { ClipboardCheck, ExternalLink, Square } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { chargingKeys, getChargeMeterValues, getChargeTransaction } from '../../api/charging'
import { errorMessage } from '../../api/client'
import type { ChargeTransaction } from '../../api/types'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import DescriptionList from '../../components/ui/DescriptionList'
import Drawer from '../../components/ui/Drawer'
import ErrorState from '../../components/ui/ErrorState'
import Spinner from '../../components/ui/Spinner'
import { usePermission } from '../../hooks/usePermission'
import { formatDateTime, formatMinutes, formatMoney, text } from '../../utils/format'
import { RemoteStopDialog, ReviewModal, type RemoteStopTarget } from './ChargingModals'
import MeterChart from './MeterChart'
import { CHARGE_STATUS_BADGE, CHARGE_STATUS_LABEL, REVIEW_STATUS_BADGE, REVIEW_STATUS_LABEL, attributionLabel, bindMethodLabel, deviationExceeded, formatKw, formatKwh, formatPct, reviewReason, txDurationMin } from './style'

interface TransactionDetailDrawerProps {
  id: string | null
  onClose: () => void
}

/** 进行中的事务：轮询间隔（WebSocket charging.updated 到达时也会失效重取） */
const LIVE_INTERVAL_MS = 10_000

interface StatCell {
  label: string
  value: string
  sub?: string
  color?: string
  bg?: string
}

function MeterGrid({ t }: { t: ChargeTransaction }) {
  const over = deviationExceeded(t.deviation_pct)
  const cells: StatCell[] = [
    { label: '桩侧计量', value: formatKwh(t.kwh), sub: typeof t.meter_start === 'number' ? `电表 ${t.meter_start} → ${t.meter_stop ?? '—'} Wh` : undefined, color: 'text-amber-600' },
    { label: 'BMS SOC 起止', value: `${formatPct(t.bms_soc_start, 0)} → ${formatPct(t.bms_soc_end, 0)}` },
    { label: 'BMS 估算电量', value: formatKwh(t.bms_kwh_est), sub: 'SOC 增量 × 电池容量' },
    { label: '偏差', value: formatPct(t.deviation_pct), sub: t.deviation_pct === null || t.deviation_pct === undefined ? '无 BMS 数据' : over ? '超过 5%，需人工复核' : '在 5% 阈值内', color: over ? 'text-red-600' : typeof t.deviation_pct === 'number' ? 'text-emerald-600' : undefined, bg: over ? 'bg-red-50' : undefined },
  ]
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {cells.map((c) => (
        <div key={c.label} className={clsx('rounded-xl p-3', c.bg ?? 'bg-slate-50')}>
          <div className="text-xs text-slate-400">{c.label}</div>
          <div className={clsx('mt-0.5 text-base font-semibold', c.color ?? 'text-slate-800')}>{c.value}</div>
          {c.sub && <div className="text-[11px] text-slate-400">{c.sub}</div>}
        </div>
      ))}
    </div>
  )
}

function DetailBody({ id }: { id: string }) {
  const { can } = usePermission()
  const [reviewing, setReviewing] = useState(false)
  const [stopTarget, setStopTarget] = useState<RemoteStopTarget | null>(null)

  const detail = useQuery({
    queryKey: chargingKeys.txDetail(id),
    queryFn: () => getChargeTransaction(id),
    refetchInterval: (q) => (q.state.data?.status === 'charging' ? LIVE_INTERVAL_MS : false),
  })
  const tx = detail.data
  const ongoing = tx?.status === 'charging'
  const meter = useQuery({
    queryKey: chargingKeys.meterValues(id),
    queryFn: () => getChargeMeterValues(id),
    enabled: Boolean(tx),
    refetchInterval: ongoing ? LIVE_INTERVAL_MS : false,
    staleTime: ongoing ? 0 : 5 * 60_000,
  })

  if (detail.isPending) {
    return (
      <div className="flex justify-center py-10">
        <Spinner label="加载中" />
      </div>
    )
  }
  if (detail.isError || !tx) return <ErrorState message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />

  const t = tx
  const canReview = can('charging:review') && t.review_status === 'pending'
  const canStop = can('charging:manage') && ongoing
  // 曲线接口失败时退化为详情里的抽样点
  const meterValues = meter.data ?? t.meter_values ?? []
  const reason = t.review_status === 'pending' ? reviewReason(t) : null
  const attribution = attributionLabel(t.attribution)

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-base font-semibold text-slate-800">{t.tx_no}</span>
        <Badge color={CHARGE_STATUS_BADGE[t.status]}>
          {ongoing && <span className="inline-block h-1.5 w-1.5 rounded-full bg-amber-500 pulse-dot" />}
          {CHARGE_STATUS_LABEL[t.status]}
        </Badge>
        <Badge color={REVIEW_STATUS_BADGE[t.review_status]}>{REVIEW_STATUS_LABEL[t.review_status]}</Badge>
        {reason && <Badge color={reason.kind === 'other' ? 'gray' : 'amber'}>{reason.label}</Badge>}
        {ongoing && <span className="text-xs text-slate-400">每 10 秒刷新</span>}
      </div>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-slate-700">基本信息</h4>
        <DescriptionList
          columns={2}
          items={[
            {
              label: '充电桩 / 连接器',
              value: (
                <span>
                  {t.pile_name} <span className="font-mono text-xs text-slate-400">{t.pile_code}</span> · {t.connector_id} 号枪
                </span>
              ),
            },
            { label: 'OCPP 事务 / 卡号', value: <span className="font-mono text-xs">{typeof t.ocpp_tx_id === 'number' ? `#${t.ocpp_tx_id}` : '—'} · {text(t.id_tag)}</span> },
            { label: '开始时间', value: formatDateTime(t.start_at) },
            { label: '结束时间', value: t.end_at ? formatDateTime(t.end_at) : <span className="text-amber-600">充电中</span> },
            { label: '时长', value: formatMinutes(txDurationMin(t)) },
            { label: ongoing ? '当前功率' : '停止原因', value: ongoing ? formatKw(t.power_kw) : text(t.stop_reason) },
          ]}
        />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-slate-700">归属</h4>
        <DescriptionList
          columns={2}
          items={[
            {
              label: '用户',
              value: t.user ? (
                <span>
                  {t.user.name}
                  {(t.dept_name ?? t.user.dept_name) && <span className="ml-1 text-xs text-slate-400">{t.dept_name ?? t.user.dept_name}</span>}
                </span>
              ) : (
                <span className="text-slate-400">未识别</span>
              ),
            },
            {
              label: '车辆',
              value: t.vehicle ? (
                <span className="flex flex-wrap items-center gap-1.5">
                  <Link to={`/assets/vehicles?id=${encodeURIComponent(t.vehicle.id)}`} className="font-medium text-brand-700 hover:underline">
                    {t.vehicle.plate_no}
                  </Link>
                  {t.vehicle.model && <span className="text-xs text-slate-400">{t.vehicle.model}</span>}
                </span>
              ) : (
                <span className="text-amber-600">未绑定车辆</span>
              ),
            },
            { label: '绑定方式', value: bindMethodLabel(t.bind_method) },
            { label: '扣费账户级别', value: attribution ?? (t.status === 'settled' ? '—' : <span className="text-slate-400">结算后确定</span>) },
          ]}
        />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-slate-700">计量与交叉校验</h4>
        <MeterGrid t={t} />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-slate-700">费用</h4>
        <div className="rounded-xl border border-slate-100">
          <div className="grid grid-cols-3 divide-x divide-slate-100 text-sm">
            <div className="px-4 py-3">
              <div className="text-xs text-slate-400">单价</div>
              <div className="mt-0.5 font-medium text-slate-800">{typeof t.unit_price === 'number' ? `${t.unit_price.toFixed(2)} 元/kWh` : '—'}</div>
            </div>
            <div className="px-4 py-3">
              <div className="text-xs text-slate-400">费用</div>
              <div className={clsx('mt-0.5 font-semibold', typeof t.cost === 'number' ? 'text-emerald-700' : 'text-slate-400')}>{typeof t.cost === 'number' ? `¥ ${formatMoney(t.cost)}` : t.status === 'settled' ? '¥ 0.00' : t.review_status === 'rejected' ? '不计费' : '待结算'}</div>
            </div>
            <div className="px-4 py-3">
              <div className="text-xs text-slate-400">扣费流水</div>
              <div className="mt-0.5 font-medium text-slate-800">
                {typeof t.account_txn_id === 'number' ? (
                  <Link to={`/billing/accounts?txn=${t.account_txn_id}`} className="inline-flex items-center gap-1 text-brand-700 hover:underline">
                    #{t.account_txn_id}
                    <ExternalLink size={12} />
                  </Link>
                ) : (
                  <span className="text-slate-400">—</span>
                )}
              </div>
            </div>
          </div>
        </div>
      </section>

      <section>
        <div className="mb-2 flex items-center justify-between">
          <h4 className="text-sm font-semibold text-slate-700">电表曲线</h4>
          <span className="text-xs text-slate-400">
            {meter.isFetching ? '刷新中…' : meter.isError ? `曲线接口不可用，显示抽样 ${meterValues.length} 点` : `${meterValues.length} 点`}
          </span>
        </div>
        {meter.isPending && !t.meter_values?.length ? (
          <div className="flex h-[220px] items-center justify-center rounded-xl bg-slate-50">
            <Spinner label="加载曲线…" />
          </div>
        ) : (
          <MeterChart values={meterValues} />
        )}
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-slate-700">复核信息</h4>
        {t.review_status === 'none' ? (
          <div className="rounded-xl bg-slate-50 px-4 py-3 text-sm text-slate-400">归属明确且计量偏差在阈值内，无需人工复核</div>
        ) : (
          <DescriptionList
            columns={2}
            items={[
              { label: '复核状态', value: <Badge color={REVIEW_STATUS_BADGE[t.review_status]}>{REVIEW_STATUS_LABEL[t.review_status]}</Badge> },
              { label: '复核人 / 时间', value: t.reviewed_by_name ? `${t.reviewed_by_name} · ${formatDateTime(t.reviewed_at)}` : null },
              { label: '备注', value: t.review_note, span: 2 },
            ]}
          />
        )}
      </section>

      {(canReview || canStop) && (
        <div className="sticky bottom-0 -mx-5 -mb-4 flex flex-wrap items-center justify-end gap-2 border-t border-slate-100 bg-white px-5 py-3">
          {canStop && (
            <Button variant="secondary" icon={Square} className="text-red-600" onClick={() => setStopTarget({ pileId: t.pile_id, pileName: t.pile_name, transaction: t })}>
              远程停止
            </Button>
          )}
          {canReview && (
            <Button icon={ClipboardCheck} onClick={() => setReviewing(true)}>
              复核
            </Button>
          )}
        </div>
      )}

      <ReviewModal transaction={reviewing ? t : null} onClose={() => setReviewing(false)} />
      {/* 复核 / 停止后详情由 invalidate 重取；抽屉保持打开以便查看结果 */}
      <RemoteStopDialog target={stopTarget} onClose={() => setStopTarget(null)} onSuccess={() => void detail.refetch()} />
    </div>
  )
}

/** 充电事务详情抽屉：基本信息 + 归属 + 计量交叉校验 + 费用 + 电表曲线 + 复核信息 + 操作 */
export default function TransactionDetailDrawer({ id, onClose }: TransactionDetailDrawerProps) {
  return (
    <Drawer open={id !== null} onClose={onClose} title="充电事务详情" width="lg">
      {id && <DetailBody key={id} id={id} />}
    </Drawer>
  )
}
