import clsx from 'clsx'
import { AlertTriangle } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { ChargeTransaction } from '../../api/types'
import Badge from '../../components/ui/Badge'
import { formatMinutes, formatMoney, formatTimeRange, text } from '../../utils/format'
import { CHARGE_STATUS_BADGE, CHARGE_STATUS_LABEL, REVIEW_STATUS_BADGE, REVIEW_STATUS_LABEL, attributionLabel, deviationExceeded, formatKw, formatKwh, formatPct, txDurationMin } from './style'

/** 事务号（进行中带呼吸点） */
export function TxNoCell({ t }: { t: ChargeTransaction }) {
  return (
    <span className="inline-flex items-center gap-1.5 font-mono text-xs font-medium text-slate-800">
      {t.status === 'charging' && <span className="inline-block h-1.5 w-1.5 rounded-full bg-amber-500 pulse-dot" aria-label="充电中" />}
      {t.tx_no}
    </span>
  )
}

export function PileCell({ t }: { t: ChargeTransaction }) {
  return (
    <div>
      <div className="text-slate-800">{t.pile_name}</div>
      <div className="font-mono text-xs text-slate-400">
        {t.pile_code} · {t.connector_id} 号枪
      </div>
    </div>
  )
}

export function UserCell({ t }: { t: ChargeTransaction }) {
  return (
    <div>
      <div className="text-slate-800">{t.user?.name ?? <span className="text-slate-400">未识别</span>}</div>
      <div className="text-xs text-slate-400">{text(t.dept_name ?? t.user?.dept_name)}</div>
    </div>
  )
}

export function VehicleCell({ t, link = false }: { t: ChargeTransaction; link?: boolean }) {
  if (!t.vehicle) {
    return (
      <span className="inline-flex items-center gap-1 text-xs text-amber-600">
        <AlertTriangle size={12} /> 未绑定
      </span>
    )
  }
  const plate = link ? (
    <Link to={`/assets/vehicles?id=${encodeURIComponent(t.vehicle.id)}`} className="font-medium text-brand-700 hover:underline" onClick={(e) => e.stopPropagation()}>
      {t.vehicle.plate_no}
    </Link>
  ) : (
    <span className="text-slate-800">{t.vehicle.plate_no}</span>
  )
  return (
    <div>
      <div>{plate}</div>
      <div className="text-xs text-slate-400">{text(t.vehicle.model)}</div>
    </div>
  )
}

export function TimeCell({ t }: { t: ChargeTransaction }) {
  return <span className="whitespace-nowrap text-xs">{formatTimeRange(t.start_at, t.end_at, '充电中')}</span>
}

export function DurationCell({ t }: { t: ChargeTransaction }) {
  return <span className="whitespace-nowrap text-xs">{formatMinutes(txDurationMin(t))}</span>
}

export function KwhCell({ t }: { t: ChargeTransaction }) {
  return (
    <div className="whitespace-nowrap text-right text-xs">
      <div className={clsx('font-medium', t.status === 'charging' ? 'text-amber-600' : 'text-slate-800')}>{formatKwh(t.kwh)}</div>
      {typeof t.bms_kwh_est === 'number' && <div className="text-[11px] text-slate-400">BMS 估算 {t.bms_kwh_est.toFixed(2)}</div>}
    </div>
  )
}

export function PowerCell({ t }: { t: ChargeTransaction }) {
  return <span className="whitespace-nowrap text-xs">{formatKw(t.power_kw)}</span>
}

export function CostCell({ t }: { t: ChargeTransaction }) {
  return (
    <div className="whitespace-nowrap text-right text-xs">
      <div className={clsx(typeof t.cost === 'number' ? 'font-medium text-emerald-700' : 'text-slate-400')}>{typeof t.cost === 'number' ? `¥ ${formatMoney(t.cost)}` : '—'}</div>
      {typeof t.unit_price === 'number' && <div className="text-[11px] text-slate-400">{t.unit_price.toFixed(2)} 元/kWh</div>}
    </div>
  )
}

export function AttributionCell({ t }: { t: ChargeTransaction }) {
  const label = attributionLabel(t.attribution)
  return label ? <span className="whitespace-nowrap text-xs text-slate-600">{label}</span> : <span className="text-xs text-slate-300">—</span>
}

/** 偏差百分比：超过阈值标红 */
export function DeviationCell({ t }: { t: ChargeTransaction }) {
  const over = deviationExceeded(t.deviation_pct)
  if (t.deviation_pct === null || t.deviation_pct === undefined) return <span className="text-xs text-slate-300">—</span>
  return (
    <span className={clsx('inline-flex items-center gap-0.5 whitespace-nowrap text-xs', over ? 'font-medium text-red-600' : 'text-slate-600')} title={over ? '桩侧计量与 BMS 估算偏差超过 5%' : undefined}>
      {over && <AlertTriangle size={12} />}
      {formatPct(t.deviation_pct)}
    </span>
  )
}

export function StatusCell({ t }: { t: ChargeTransaction }) {
  return <Badge color={CHARGE_STATUS_BADGE[t.status]}>{CHARGE_STATUS_LABEL[t.status]}</Badge>
}

export function ReviewCell({ t }: { t: ChargeTransaction }) {
  return <Badge color={REVIEW_STATUS_BADGE[t.review_status]}>{REVIEW_STATUS_LABEL[t.review_status]}</Badge>
}
