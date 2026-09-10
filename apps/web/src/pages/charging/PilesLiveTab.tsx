import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { AlertTriangle, Play, Square, Wifi, WifiOff } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { chargingKeys, listPilesLive } from '../../api/charging'
import { errorMessage } from '../../api/client'
import type { ChargeTransaction, PileLive, PileLiveConnector, PileStatus } from '../../api/types'
import MapView from '../../components/map/MapView'
import type { MapMarker } from '../../components/map/types'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import Empty from '../../components/ui/Empty'
import ErrorState from '../../components/ui/ErrorState'
import Spinner from '../../components/ui/Spinner'
import { usePermission } from '../../hooks/usePermission'
import { useRealtimeStore } from '../../store/realtime'
import { formatDateTime, formatMinutes } from '../../utils/format'
import { toLngLat } from '../../utils/geo'
import { RemoteStartModal, RemoteStopDialog, type RemoteStartTarget, type RemoteStopTarget } from './ChargingModals'
import { PILES_LIVE_INTERVAL_MS, PILE_CARD_CLASS, PILE_STATUS_BADGE, PILE_STATUS_HEX, PILE_STATUS_LABEL, PILE_TYPE_LABEL, connectorStyle, formatKw, formatKwh, txDurationMin } from './style'

const LEGEND: PileStatus[] = ['available', 'charging', 'offline', 'faulted', 'disabled']

interface PilesLiveTabProps {
  onOpenTransaction: (id: string) => void
}

/** 每分钟 tick 一次，让"时长"随时间走 */
function useMinuteTick(): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60_000)
    return () => window.clearInterval(timer)
  }, [])
  return now
}

function OngoingTransaction({ t, now, onOpen }: { t: ChargeTransaction; now: number; onOpen: () => void }) {
  return (
    <button type="button" onClick={onOpen} className="mt-2 w-full rounded-lg bg-surface-2/80 px-3 py-2 text-left text-xs transition-colors hover:bg-surface-2" title="查看事务详情">
      <div className="flex items-center justify-between">
        <span className="font-mono text-ink-muted">{t.tx_no}</span>
        <span className="font-medium text-warn-200">{formatKwh(t.kwh)}</span>
      </div>
      <div className="mt-1 grid grid-cols-2 gap-x-3 gap-y-0.5 text-ink">
        <span className="truncate">
          <span className="text-ink-faint">用户 </span>
          {t.user?.name ?? '未识别'}
        </span>
        <span className="truncate">
          <span className="text-ink-faint">车辆 </span>
          {t.vehicle?.plate_no ?? <span className="text-warn-200">未绑定</span>}
        </span>
        <span>
          <span className="text-ink-faint">功率 </span>
          {formatKw(t.power_kw)}
        </span>
        <span>
          <span className="text-ink-faint">时长 </span>
          {formatMinutes(txDurationMin(t, now))}
        </span>
        <span className="col-span-2">
          <span className="text-ink-faint">开始 </span>
          {formatDateTime(t.start_at)}
        </span>
      </div>
    </button>
  )
}

function ConnectorRow({ pile, c, now, canManage, onStart, onStop, onOpen }: { pile: PileLive; c: PileLiveConnector; now: number; canManage: boolean; onStart: () => void; onStop: () => void; onOpen: (id: string) => void }) {
  const s = connectorStyle(c.status)
  const tx = c.transaction ?? null
  const controllable = canManage && pile.online && pile.status !== 'disabled'
  const showStart = controllable && s.startable && !tx
  const showStop = controllable && tx?.status === 'charging'
  return (
    <div className="rounded-xl border border-line bg-surface-2/60 px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs">
          <span className="font-medium text-ink">{c.connector_id} 号枪</span>
          <Badge color={s.badge}>
            {s.pulse && <span className="inline-block h-1.5 w-1.5 rounded-full bg-warn-500 pulse-dot" />}
            {s.label}
          </Badge>
          {c.error_code && c.error_code !== 'NoError' && (
            <span className="inline-flex items-center gap-0.5 font-mono text-[11px] text-danger-200" title="OCPP errorCode">
              <AlertTriangle size={11} />
              {c.error_code}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {showStart && (
            <Button size="sm" variant="secondary" icon={Play} className="!h-7 !px-2 text-ev-200" onClick={onStart}>
              远程启动
            </Button>
          )}
          {showStop && (
            <Button size="sm" variant="secondary" icon={Square} className="!h-7 !px-2 text-danger-200" onClick={onStop}>
              远程停止
            </Button>
          )}
        </div>
      </div>
      {tx && <OngoingTransaction t={tx} now={now} onOpen={() => onOpen(tx.id)} />}
    </div>
  )
}

function PileCard({ pile, now, selected, canManage, onSelect, onStart, onStop, onOpen }: { pile: PileLive; now: number; selected: boolean; canManage: boolean; onSelect: () => void; onStart: (connectorId: number) => void; onStop: (t: ChargeTransaction) => void; onOpen: (id: string) => void }) {
  const chargingCount = pile.connectors.filter((c) => c.status === 'Charging').length
  return (
    <div className={clsx('card border p-4 transition-shadow', PILE_CARD_CLASS[pile.status], selected && 'ring-2 ring-brand-300')} onClick={onSelect}>
      <div className="mb-3 flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-ink-strong">{pile.name}</div>
          <div className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-ink-faint">
            <span className="font-mono">{pile.pile_code}</span>
            <span>
              {PILE_TYPE_LABEL[pile.type]} {pile.power_kw} kW
            </span>
            {pile.location && <span className="truncate">{pile.location}</span>}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <span className={clsx('inline-flex items-center gap-1 text-[11px]', pile.online ? 'text-ev-200' : 'text-ink-faint')} title={pile.last_heartbeat_at ? `最近心跳 ${formatDateTime(pile.last_heartbeat_at)}` : '无心跳'}>
            <span className={clsx('inline-block h-2 w-2 rounded-full', pile.online ? 'bg-ev-500 pulse-dot' : 'bg-ink-disabled')} />
            {pile.online ? <Wifi size={12} /> : <WifiOff size={12} />}
          </span>
          <Badge color={PILE_STATUS_BADGE[pile.status]}>{PILE_STATUS_LABEL[pile.status]}</Badge>
        </div>
      </div>

      <div className="space-y-2">
        {pile.connectors.length === 0 ? (
          <div className="text-xs text-ink-faint">桩尚未上报连接器状态</div>
        ) : (
          pile.connectors.map((c) => <ConnectorRow key={c.connector_id} pile={pile} c={c} now={now} canManage={canManage} onStart={() => onStart(c.connector_id)} onStop={() => c.transaction && onStop(c.transaction)} onOpen={onOpen} />)
        )}
      </div>

      {pile.status === 'faulted' && (
        <div className="mt-3 flex items-center gap-1.5 text-xs text-danger-200">
          <AlertTriangle size={13} />
          桩上报故障，请检查现场或联系厂商
        </div>
      )}
      {pile.status === 'charging' && chargingCount > 1 && <div className="mt-2 text-[11px] text-ink-faint">{chargingCount} 个连接器同时充电</div>}
    </div>
  )
}

/** 桩位小地图：按桩状态着色 */
function PilesLiveMap({ piles, selectedId, onSelect }: { piles: PileLive[]; selectedId: string | null; onSelect: (id: string | null) => void }) {
  const located = useMemo(() => piles.filter((p) => toLngLat(p.lng, p.lat) !== null), [piles])
  const markers = useMemo<MapMarker[]>(
    () =>
      located.map((p) => {
        const lnglat = toLngLat(p.lng, p.lat) as [number, number]
        const charging = p.connectors.filter((c) => c.status === 'Charging').length
        return {
          id: p.id,
          lnglat,
          icon: 'pile',
          label: p.name,
          color: PILE_STATUS_HEX[p.status],
          opacity: p.online ? 1 : 0.55,
          title: `${p.pile_code} · ${PILE_TYPE_LABEL[p.type]} ${p.power_kw}kW · ${PILE_STATUS_LABEL[p.status]}${charging > 0 ? ` · ${charging} 枪充电中` : ''}`,
          selected: p.id === selectedId,
          zIndex: p.id === selectedId ? 10 : 1,
          onClick: (m) => onSelect(m.id),
        }
      }),
    [located, selectedId, onSelect],
  )
  return (
    <MapView
      height={300}
      markers={markers}
      fitKey={located.length}
      onClick={() => onSelect(null)}
      hint={located.length === 0 ? '暂无带坐标的充电桩' : undefined}
      overlay={
        <div className="absolute bottom-8 left-2 flex flex-wrap items-center gap-x-2.5 gap-y-1 rounded-md bg-surface-2/90 px-2 py-1 text-[11px] text-ink shadow-sm">
          {LEGEND.map((s) => (
            <span key={s} className="inline-flex items-center gap-1">
              <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ background: PILE_STATUS_HEX[s] }} />
              {PILE_STATUS_LABEL[s]}
            </span>
          ))}
        </div>
      }
    />
  )
}

/** 桩状态页签：卡片网格（连接器 + 进行中事务 + 远程启停）与桩位小地图 */
export default function PilesLiveTab({ onOpenTransaction }: PilesLiveTabProps) {
  const { can } = usePermission()
  const canManage = can('charging:manage')
  const wsStatus = useRealtimeStore((s) => s.status)
  const now = useMinuteTick()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [startTarget, setStartTarget] = useState<RemoteStartTarget | null>(null)
  const [stopTarget, setStopTarget] = useState<RemoteStopTarget | null>(null)

  const live = useQuery({ queryKey: chargingKeys.pilesLive, queryFn: listPilesLive, refetchInterval: PILES_LIVE_INTERVAL_MS, staleTime: 10_000 })
  const piles = useMemo(() => live.data ?? [], [live.data])
  const sorted = useMemo(() => {
    // 故障 / 充电中靠前，其余按编号
    const rank: Record<PileStatus, number> = { faulted: 0, charging: 1, available: 2, offline: 3, disabled: 4 }
    return [...piles].sort((a, b) => rank[a.status] - rank[b.status] || a.pile_code.localeCompare(b.pile_code))
  }, [piles])
  const counts = useMemo(() => {
    const c = { total: piles.length, online: 0, charging: 0, faulted: 0 }
    for (const p of piles) {
      if (p.online) c.online += 1
      if (p.status === 'charging' || p.connectors.some((x) => x.status === 'Charging')) c.charging += 1
      if (p.status === 'faulted' || p.connectors.some((x) => x.status === 'Faulted')) c.faulted += 1
    }
    return c
  }, [piles])

  if (live.isError) {
    return (
      <div className="card p-4">
        <ErrorState message={errorMessage(live.error)} onRetry={() => void live.refetch()} />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-ink-faint">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
          <span>
            共 <span className="font-medium text-ink">{counts.total}</span> 桩
          </span>
          <span>
            在线 <span className="font-medium text-ev-200">{counts.online}</span>
          </span>
          <span>
            充电中 <span className="font-medium text-warn-200">{counts.charging}</span>
          </span>
          <span>
            故障 <span className={clsx('font-medium', counts.faulted > 0 ? 'text-danger-200' : 'text-ink')}>{counts.faulted}</span>
          </span>
        </div>
        <span className={clsx('inline-flex items-center gap-1', wsStatus === 'open' && 'text-ev-200')}>
          <span className={clsx('inline-block h-1.5 w-1.5 rounded-full', wsStatus === 'open' ? 'bg-tech-400 pulse-dot' : wsStatus === 'reconnecting' ? 'bg-warn-400' : 'bg-ink-disabled')} />
          {wsStatus === 'open' ? '实时推送 + 每 30 秒刷新' : wsStatus === 'reconnecting' ? '重连中 · 每 30 秒刷新' : '每 30 秒刷新'}
          {live.isFetching && <Spinner size="sm" className="ml-1" />}
        </span>
      </div>

      {counts.faulted > 0 && (
        <div className="flex items-center gap-3 rounded-2xl border border-danger-500/30 bg-danger-500/10 p-4">
          <AlertTriangle size={18} className="shrink-0 text-danger-200" />
          <div className="text-sm font-semibold text-danger-200">{counts.faulted} 个充电桩故障</div>
          <div className="text-xs text-danger-200">
            {sorted
              .filter((p) => p.status === 'faulted' || p.connectors.some((x) => x.status === 'Faulted'))
              .map((p) => p.name)
              .join('、')}
          </div>
        </div>
      )}

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
        <div>
          {live.isPending ? (
            <div className="flex h-48 items-center justify-center rounded-xl bg-surface-3">
              <Spinner label="加载桩状态…" />
            </div>
          ) : sorted.length === 0 ? (
            <div className="card">
              <Empty title="还没有充电桩" description="在“充电桩档案”登记桩后，桩通过 OCPP 上线即显示在这里" />
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 2xl:grid-cols-3">
              {sorted.map((p) => (
                <PileCard key={p.id} pile={p} now={now} selected={p.id === selectedId} canManage={canManage} onSelect={() => setSelectedId(p.id)} onStart={(connectorId) => setStartTarget({ pile: p, connectorId })} onStop={(t) => setStopTarget({ pileId: p.id, pileName: p.name, transaction: t })} onOpen={onOpenTransaction} />
              ))}
            </div>
          )}
        </div>
        <div className="card p-4 xl:sticky xl:top-4 xl:self-start">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-ink">桩位分布</h3>
            <span className="text-xs text-ink-faint">{live.isPending ? '加载中…' : `${piles.filter((p) => toLngLat(p.lng, p.lat) !== null).length} / ${piles.length} 个桩有坐标`}</span>
          </div>
          <PilesLiveMap piles={piles} selectedId={selectedId} onSelect={setSelectedId} />
        </div>
      </div>

      <RemoteStartModal target={startTarget} onClose={() => setStartTarget(null)} />
      <RemoteStopDialog target={stopTarget} onClose={() => setStopTarget(null)} />
    </div>
  )
}
