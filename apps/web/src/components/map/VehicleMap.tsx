import clsx from 'clsx'
import { BatteryCharging, Lightbulb, Lock, Unlock, Wifi, WifiOff, X } from 'lucide-react'
import { useCallback, useMemo, useState, type ReactNode } from 'react'
import type { VehicleLive } from '../../api/types'
import { formatDateTime, text } from '../../utils/format'
import { toLngLat } from '../../utils/geo'
import Badge from '../ui/Badge'
import MapView from './MapView'
import type { MapMarker } from './types'
import { useMapEngine } from './useMapEngine'
import { OFFLINE_OPACITY, VEHICLE_STATUS_BADGE, VEHICLE_STATUS_COLOR, VEHICLE_STATUS_LABEL, formatKm, formatPercent, formatSpeed, socBarClass, socTextClass } from './vehicleStyle'

export interface VehicleMapProps {
  vehicles: VehicleLive[]
  /** 受控选中；不传则内部维护 */
  selectedId?: string | null
  onSelect?: (id: string | null) => void
  height?: number | string
  /** 变化时重新适配视野（缺省首次有数据时适配一次） */
  fitKey?: string | number
  className?: string
  /** 信息卡底部的附加操作（如"查看详情"） */
  renderActions?: (v: VehicleLive) => ReactNode
  showLegend?: boolean
}

const LEGEND: Array<keyof typeof VEHICLE_STATUS_LABEL> = ['idle', 'in_use', 'charging', 'maintenance', 'disabled']

function VehicleInfoCard({ v, onClose, actions }: { v: VehicleLive; onClose: () => void; actions?: ReactNode }) {
  const soc = v.soc ?? null
  return (
    <div className="absolute right-11 top-2 w-64 rounded-card border border-line bg-surface-2/95 p-3 shadow-float backdrop-blur slide-up">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            <span className="font-mono text-sm font-semibold text-ink-strong">{v.plate_no}</span>
            <Badge color={VEHICLE_STATUS_BADGE[v.status]}>{VEHICLE_STATUS_LABEL[v.status]}</Badge>
          </div>
          <div className="mt-0.5 truncate text-xs text-ink-faint">{[v.brand, v.model].filter(Boolean).join(' ') || '—'}</div>
        </div>
        <button type="button" aria-label="关闭" onClick={onClose} className="rounded p-0.5 text-ink-faint hover:bg-surface-3 hover:text-ink-strong">
          <X size={14} />
        </button>
      </div>
      <dl className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1.5 text-xs">
        <div>
          <dt className="text-ink-faint">驾驶员</dt>
          <dd className="font-medium text-ink">{text(v.driver_name)}</dd>
        </div>
        <div>
          <dt className="text-ink-faint">速度</dt>
          <dd className="font-medium text-ink">{formatSpeed(v.speed)}</dd>
        </div>
        <div>
          <dt className="text-ink-faint">电量</dt>
          <dd className={clsx('font-medium', soc !== null ? socTextClass(soc) : 'text-ink')}>
            {soc !== null ? (
              <span className="flex items-center gap-1.5">
                <span className="h-1.5 w-12 overflow-hidden rounded-full bg-surface-4">
                  <span className={clsx('block h-full rounded-full', socBarClass(soc))} style={{ width: `${Math.max(0, Math.min(100, soc))}%` }} />
                </span>
                {formatPercent(soc)}
              </span>
            ) : (
              '—'
            )}
          </dd>
        </div>
        <div>
          <dt className="text-ink-faint">续航</dt>
          <dd className="font-medium text-ink">{formatKm(v.range_km)}</dd>
        </div>
        <div className="col-span-2">
          <dt className="text-ink-faint">更新时间</dt>
          <dd className="font-medium text-ink">{formatDateTime(v.last_telemetry_at ?? v.updated_at)}</dd>
        </div>
      </dl>
      <div className="mt-2 flex flex-wrap gap-1">
        <span className={clsx('inline-flex items-center gap-1 rounded-[5px] border px-1.5 py-0.5 text-[10px]', v.online ? 'border-ev-500/30 bg-ev-500/10 text-ev-200' : 'border-line bg-white/[0.06] text-ink-muted')}>
          {v.online ? <Wifi size={10} /> : <WifiOff size={10} />}
          {v.online ? '在线' : '离线'}
        </span>
        <span className={clsx('inline-flex items-center gap-1 rounded-[5px] border px-1.5 py-0.5 text-[10px]', v.sign_on ? 'border-brand-600/40 bg-brand-600/15 text-brand-300' : 'border-line bg-white/[0.06] text-ink-muted')}>
          <Lightbulb size={10} />
          灯牌{v.sign_on ? '亮' : '灭'}
        </span>
        <span className={clsx('inline-flex items-center gap-1 rounded-[5px] border px-1.5 py-0.5 text-[10px]', v.locked ? 'border-line bg-white/[0.06] text-ink-muted' : 'border-warn-500/30 bg-warn-500/10 text-warn-200')}>
          {v.locked ? <Lock size={10} /> : <Unlock size={10} />}
          {v.locked ? '已锁车' : '未锁车'}
        </span>
        {v.charging && (
          <span className="inline-flex items-center gap-1 rounded-[5px] border border-warn-500/30 bg-warn-500/10 px-1.5 py-0.5 text-[10px] text-warn-200">
            <BatteryCharging size={10} />
            充电中
          </span>
        )}
      </div>
      {actions && <div className="mt-2 border-t border-line-soft pt-2">{actions}</div>}
    </div>
  )
}

/**
 * 车辆实时地图：按状态着色（离线半透明），标记显示车牌，点击弹出信息卡。
 * 两种引擎（高德 / 示意图）通用。
 */
export default function VehicleMap({ vehicles, selectedId, onSelect, height = 380, fitKey, className, renderActions, showLegend = true }: VehicleMapProps) {
  const { engine } = useMapEngine()
  const [innerSelected, setInnerSelected] = useState<string | null>(null)
  const controlled = selectedId !== undefined
  const current = controlled ? selectedId : innerSelected
  const select = useCallback(
    (id: string | null) => {
      if (!controlled) setInnerSelected(id)
      onSelect?.(id)
    },
    [controlled, onSelect],
  )

  const located = useMemo(() => vehicles.filter((v) => toLngLat(v.lng, v.lat) !== null), [vehicles])

  const markers = useMemo<MapMarker[]>(
    () =>
      located.map((v) => {
        const lnglat = toLngLat(v.lng, v.lat) as [number, number]
        return {
          id: v.vehicle_id,
          lnglat,
          label: v.plate_no,
          color: VEHICLE_STATUS_COLOR[v.status],
          icon: 'vehicle',
          heading: v.heading,
          opacity: v.online ? 1 : OFFLINE_OPACITY,
          title: `${v.plate_no} · ${VEHICLE_STATUS_LABEL[v.status]}${v.online ? '' : '（离线）'}`,
          selected: v.vehicle_id === current,
          zIndex: v.vehicle_id === current ? 10 : v.status === 'in_use' ? 2 : 1,
          onClick: (m) => select(m.id),
        }
      }),
    [located, current, select],
  )

  const sel = current ? vehicles.find((v) => v.vehicle_id === current) ?? null : null
  const unlocated = vehicles.length - located.length

  return (
    <MapView
      className={className}
      height={height}
      markers={markers}
      fitKey={fitKey}
      onClick={() => select(null)}
      hint={unlocated > 0 ? `${unlocated} 辆无位置` : undefined}
      overlay={
        <>
          {sel && <VehicleInfoCard v={sel} onClose={() => select(null)} actions={renderActions?.(sel)} />}
          {showLegend && (
            <div className={clsx('absolute left-2 flex flex-wrap items-center gap-x-2.5 gap-y-1 rounded-lg border border-line bg-surface-2/85 px-2.5 py-1.5 text-[11px] text-ink backdrop-blur', engine === 'amap' ? 'top-2' : 'bottom-8')}>
              {LEGEND.map((s) => (
                <span key={s} className="inline-flex items-center gap-1">
                  <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ background: VEHICLE_STATUS_COLOR[s] }} />
                  {VEHICLE_STATUS_LABEL[s]}
                </span>
              ))}
              <span className="inline-flex items-center gap-1">
                <span className="inline-block h-2.5 w-2.5 rounded-full bg-ink-faint opacity-40" />
                离线
              </span>
            </div>
          )}
        </>
      }
    />
  )
}
