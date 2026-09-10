import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import dayjs from 'dayjs'
import { BatteryCharging, Lightbulb, Lock, Pencil, Unlock, Wifi, WifiOff } from 'lucide-react'
import { useMemo, useState } from 'react'
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { errorMessage } from '../../../api/client'
import type { TelemetryPoint, Vehicle, VehicleLive } from '../../../api/types'
import { getVehicle, listVehicleTelemetry, vehicleKeys, type TelemetryQuery } from '../../../api/vehicles'
import MapView from '../../../components/map/MapView'
import type { MapMarker } from '../../../components/map/types'
import { VEHICLE_STATUS_BADGE, VEHICLE_STATUS_COLOR, VEHICLE_STATUS_LABEL, formatKm, formatPercent, formatSpeed, socBarClass, socTextClass } from '../../../components/map/vehicleStyle'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import DescriptionList from '../../../components/ui/DescriptionList'
import Drawer from '../../../components/ui/Drawer'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import Spinner from '../../../components/ui/Spinner'
import { formatDate, formatDateTime, text } from '../../../utils/format'
import { sampleEvenly, toLngLat } from '../../../utils/geo'

interface VehicleDetailDrawerProps {
  id: string | null
  onClose: () => void
  /** 有编辑权限时显示"编辑"按钮 */
  onEdit?: (vehicle: Vehicle) => void
}

const CHART_MAX_POINTS = 300
const TELEMETRY_HOURS = 24

interface ChartPoint {
  t: number
  soc: number | null
  speed: number | null
}

function toChartPoints(points: TelemetryPoint[]): ChartPoint[] {
  const valid = points
    .map((p) => ({ t: Date.parse(p.ts), soc: typeof p.soc === 'number' ? p.soc : null, speed: typeof p.speed === 'number' ? p.speed : null }))
    .filter((p) => Number.isFinite(p.t))
    .sort((a, b) => a.t - b.t)
  return sampleEvenly(valid, CHART_MAX_POINTS)
}

type Domain = [number | 'auto', number | 'auto']

function TelemetryChart({ data, dataKey, color, unit, domain }: { data: ChartPoint[]; dataKey: 'soc' | 'speed'; color: string; unit: string; domain?: Domain }) {
  return (
    <ResponsiveContainer width="100%" height={150}>
      <LineChart data={data} margin={{ top: 8, right: 12, bottom: 0, left: -12 }}>
        <CartesianGrid vertical={false} stroke="#16304f" />
        <XAxis dataKey="t" type="number" domain={['dataMin', 'dataMax']} tickFormatter={(v: number) => dayjs(v).format('HH:mm')} tick={{ fontSize: 10, fill: '#93aac4' }} axisLine={false} tickLine={false} minTickGap={32} />
        <YAxis domain={domain ?? ['auto', 'auto']} tick={{ fontSize: 10, fill: '#93aac4' }} axisLine={false} tickLine={false} width={40} />
        <Tooltip
          labelFormatter={(v) => dayjs(Number(v)).format('MM-DD HH:mm:ss')}
          formatter={(v) => [`${typeof v === 'number' ? Math.round(v * 10) / 10 : v} ${unit}`, '']}
          separator=""
          contentStyle={{ fontSize: 12, borderRadius: 8, border: '1px solid #23456B', background: '#0D1E33' }} labelStyle={{ color: '#93AAC4' }} itemStyle={{ color: '#DCE7F2' }}
          cursor={{ stroke: '#465e7e', strokeDasharray: '3 3' }}
        />
        <Line type="monotone" dataKey={dataKey} stroke={color} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  )
}

function LiveCard({ live }: { live: VehicleLive }) {
  const pos = toLngLat(live.lng, live.lat)
  const soc = live.soc ?? null
  const markers = useMemo<MapMarker[]>(
    () => (pos ? [{ id: live.vehicle_id, lnglat: pos, icon: 'vehicle', label: live.plate_no, color: VEHICLE_STATUS_COLOR[live.status], heading: live.heading, opacity: live.online ? 1 : 0.45 }] : []),
    [pos, live],
  )
  const chip = (on: boolean, onCls: string, offCls: string, label: string, Icon: typeof Wifi) => (
    <span className={clsx('inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px]', on ? onCls : offCls)}>
      <Icon size={11} />
      {label}
    </span>
  )
  return (
    <div className="space-y-3">
      <MapView height={200} markers={markers} fitKey={pos ? 'pos' : 'none'} hint={!pos ? '暂无位置' : undefined} />
      <div className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm sm:grid-cols-3">
        <div>
          <div className="text-xs text-ink-faint">速度</div>
          <div className="font-medium text-ink-strong">{formatSpeed(live.speed)}</div>
        </div>
        <div>
          <div className="text-xs text-ink-faint">电量 SOC</div>
          <div className={clsx('font-medium', soc !== null ? socTextClass(soc) : 'text-ink-strong')}>
            {soc !== null ? (
              <span className="flex items-center gap-2">
                <span className="h-1.5 w-16 overflow-hidden rounded-full bg-surface-4">
                  <span className={clsx('block h-full rounded-full', socBarClass(soc))} style={{ width: `${Math.max(0, Math.min(100, soc))}%` }} />
                </span>
                {formatPercent(soc)}
              </span>
            ) : (
              '—'
            )}
          </div>
        </div>
        <div>
          <div className="text-xs text-ink-faint">续航</div>
          <div className="font-medium text-ink-strong">{formatKm(live.range_km)}</div>
        </div>
        <div>
          <div className="text-xs text-ink-faint">电池健康 SOH</div>
          <div className="font-medium text-ink-strong">{formatPercent(live.soh)}</div>
        </div>
        <div>
          <div className="text-xs text-ink-faint">里程</div>
          <div className="font-medium text-ink-strong">{formatKm(live.odometer_km, 1)}</div>
        </div>
        <div>
          <div className="text-xs text-ink-faint">驾驶员</div>
          <div className="font-medium text-ink-strong">{text(live.driver_name)}</div>
        </div>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {chip(live.online, 'bg-ev-500/10 text-ev-200', 'bg-surface-4 text-ink-muted', live.online ? '在线' : '离线', live.online ? Wifi : WifiOff)}
        {chip(live.sign_on, 'bg-brand-600/10 text-brand-300', 'bg-surface-4 text-ink-muted', live.sign_on ? '灯牌亮' : '灯牌灭', Lightbulb)}
        {chip(live.locked, 'bg-surface-4 text-ink', 'bg-warn-500/10 text-warn-200', live.locked ? '已锁车' : '未锁车', live.locked ? Lock : Unlock)}
        {chip(live.charging, 'bg-warn-500/10 text-warn-200', 'bg-surface-4 text-ink-muted', live.charging ? '充电中' : '未充电', BatteryCharging)}
      </div>
      <div className="text-xs text-ink-faint">最后遥测 {formatDateTime(live.last_telemetry_at)} · 状态更新 {formatDateTime(live.updated_at)}</div>
    </div>
  )
}

function DetailBody({ id, onEdit }: { id: string; onEdit?: (v: Vehicle) => void }) {
  const detail = useQuery({ queryKey: vehicleKeys.detail(id), queryFn: () => getVehicle(id) })
  // 时间窗在挂载时固定，避免每次渲染变更 query key
  const [range] = useState<TelemetryQuery>(() => {
    const to = dayjs()
    return { from: to.subtract(TELEMETRY_HOURS, 'hour').toISOString(), to: to.toISOString(), limit: 5000 }
  })
  const telemetry = useQuery({ queryKey: vehicleKeys.telemetry(id, range), queryFn: () => listVehicleTelemetry(id, range), staleTime: 60_000 })
  const chart = useMemo(() => toChartPoints(telemetry.data ?? []), [telemetry.data])

  if (detail.isPending) {
    return (
      <div className="flex justify-center py-10">
        <Spinner label="加载中" />
      </div>
    )
  }
  if (detail.isError) return <ErrorState message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />

  const v = detail.data
  const live = v.live ?? null
  const status = live?.status ?? v.status

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="text-lg font-semibold text-ink-strong">{v.plate_no}</span>
          <Badge color={VEHICLE_STATUS_BADGE[status]}>{VEHICLE_STATUS_LABEL[status]}</Badge>
          {live && (
            <span className={clsx('inline-flex items-center gap-1 text-xs', live.online ? 'text-ev-200' : 'text-ink-faint')}>
              <span className={clsx('inline-block h-1.5 w-1.5 rounded-full', live.online ? 'bg-ev-500 pulse-dot' : 'bg-ink-disabled')} />
              {live.online ? '在线' : '离线'}
            </span>
          )}
        </div>
        {onEdit && (
          <Button variant="secondary" size="sm" icon={Pencil} onClick={() => onEdit(v)}>
            编辑
          </Button>
        )}
      </div>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">基本信息</h4>
        <DescriptionList
          columns={3}
          items={[
            { label: '品牌 / 型号', value: [v.brand, v.model].filter(Boolean).join(' ') },
            { label: '颜色', value: v.color },
            { label: 'VIN', value: v.vin ? <span className="font-mono text-xs">{v.vin}</span> : null },
            { label: '座位数', value: v.seat_count },
            { label: '电池容量', value: `${v.battery_kwh} kWh` },
            { label: '满电续航', value: `${v.range_km_full} km` },
            { label: '总里程', value: formatKm(v.odometer_km, 1) },
            { label: '累计行程', value: typeof v.trip_count === 'number' ? `${v.trip_count} 次` : null },
            { label: '归属部门', value: v.home_dept_name },
            { label: '购置日期', value: formatDate(v.purchase_date) },
            { label: '保险到期', value: formatDate(v.insurance_expire) },
            { label: '年检到期', value: formatDate(v.inspection_expire) },
            { label: '绑定设备', value: v.device_serial ? <span className="font-mono text-xs">{v.device_serial}</span> : <span className="text-ink-faint">未绑定网关</span> },
            { label: '创建时间', value: formatDateTime(v.created_at) },
            { label: '更新时间', value: formatDateTime(v.updated_at) },
            { label: '备注', value: v.remark, span: 3 },
          ]}
        />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">实时状态</h4>
        {live ? <LiveCard live={live} /> : <Empty size="sm" title="暂无实时状态" description="车辆尚未绑定网关或未上报遥测" />}
      </section>

      <section>
        <div className="mb-2 flex items-center justify-between">
          <h4 className="text-sm font-semibold text-ink">最近 {TELEMETRY_HOURS} 小时遥测</h4>
          {telemetry.data && telemetry.data.length > 0 && (
            <span className="text-xs text-ink-faint">
              {telemetry.data.length} 点{telemetry.data.length > CHART_MAX_POINTS ? `，抽样至 ${chart.length} 点` : ''}
            </span>
          )}
        </div>
        {telemetry.isPending ? (
          <div className="flex justify-center py-6">
            <Spinner size="sm" label="加载遥测" />
          </div>
        ) : telemetry.isError ? (
          <ErrorState size="sm" message={errorMessage(telemetry.error)} onRetry={() => void telemetry.refetch()} />
        ) : chart.length === 0 ? (
          <Empty size="sm" title="暂无遥测数据" />
        ) : (
          <div className="space-y-3">
            <div>
              <div className="mb-1 flex items-center gap-1.5 text-xs text-ink-muted">
                <span className="inline-block h-2 w-2 rounded-full" style={{ background: '#10b981' }} />
                电量 SOC（%）
              </div>
              <TelemetryChart data={chart} dataKey="soc" color="#10b981" unit="%" domain={[0, 100]} />
            </div>
            <div>
              <div className="mb-1 flex items-center gap-1.5 text-xs text-ink-muted">
                <span className="inline-block h-2 w-2 rounded-full" style={{ background: '#5c9df0' }} />
                速度（km/h）
              </div>
              <TelemetryChart data={chart} dataKey="speed" color="#5c9df0" unit="km/h" domain={[0, 'auto']} />
            </div>
          </div>
        )}
      </section>
    </div>
  )
}

/** 车辆详情抽屉：基本信息 + 实时状态卡（小地图）+ 遥测曲线 */
export default function VehicleDetailDrawer({ id, onClose, onEdit }: VehicleDetailDrawerProps) {
  return (
    <Drawer open={id !== null} onClose={onClose} title="车辆详情" width="lg">
      {id && <DetailBody key={id} id={id} onEdit={onEdit} />}
    </Drawer>
  )
}
