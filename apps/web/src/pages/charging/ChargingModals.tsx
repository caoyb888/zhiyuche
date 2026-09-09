import { useMutation } from '@tanstack/react-query'
import clsx from 'clsx'
import { AlertTriangle, CheckCircle2 } from 'lucide-react'
import { useState } from 'react'
import { remoteStartCharging, remoteStopCharging, reviewChargeTransaction, type ReviewAction } from '../../api/charging'
import { errorMessage } from '../../api/client'
import type { ChargeTransaction, PileLive, UserBrief } from '../../api/types'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import ConfirmDialog from '../../components/ui/ConfirmDialog'
import Input from '../../components/ui/Input'
import Modal from '../../components/ui/Modal'
import Select, { type SelectOption } from '../../components/ui/Select'
import Textarea from '../../components/ui/Textarea'
import { useToast } from '../../components/ui/toast-context'
import UserPicker from '../../components/UserPicker'
import { usePermission } from '../../hooks/usePermission'
import { useVehicleOptions } from '../../hooks/useVehicleOptions'
import { useInvalidateCharging } from './hooks'
import { useAuthStore } from '../../store/auth'
import { formatMoney, formatTimeRange, text } from '../../utils/format'
import { REVIEW_STATUS_BADGE, REVIEW_STATUS_LABEL, attributionLabel, bindMethodLabel, connectorStyle, deviationExceeded, formatKwh, formatPct, reviewReason } from './style'


// ── 远程启动 ──────────────────────────────────────────
export interface RemoteStartTarget {
  pile: PileLive
  /** 预选的连接器 */
  connectorId?: number
}

interface RemoteStartModalProps {
  target: RemoteStartTarget | null
  onClose: () => void
}

function RemoteStartForm({ pile, connectorId, onClose }: { pile: PileLive; connectorId?: number; onClose: () => void }) {
  const toast = useToast()
  const invalidate = useInvalidateCharging()
  const { can } = usePermission()
  const profile = useAuthStore((s) => s.profile)
  const canPickVehicle = can('asset:vehicle:view')
  const { options: vehicleOptions, query: vehicleQuery } = useVehicleOptions(canPickVehicle)

  const startable = pile.connectors.filter((c) => connectorStyle(c.status).startable && !c.transaction)
  const connectorOptions: SelectOption[] = pile.connectors.map((c) => {
    const s = connectorStyle(c.status)
    const ok = s.startable && !c.transaction
    return { value: String(c.connector_id), label: `${c.connector_id} 号枪 · ${s.label}${ok ? '' : '（不可启动）'}`, disabled: !ok }
  })
  const initial = connectorId !== undefined && startable.some((c) => c.connector_id === connectorId) ? connectorId : startable[0]?.connector_id
  const [connector, setConnector] = useState<string>(initial !== undefined ? String(initial) : '')
  const [idTag, setIdTag] = useState('')
  const [vehicleId, setVehicleId] = useState('')

  const mutation = useMutation({
    mutationFn: () =>
      remoteStartCharging(pile.id, {
        connector_id: Number(connector),
        id_tag: idTag.trim() || undefined,
        vehicle_id: vehicleId || undefined,
      }),
    onSuccess: (res) => {
      if (res?.status === 'Rejected') {
        toast.error('桩拒绝了启动请求', '请检查连接器是否已插枪、卡号是否有效')
      } else {
        toast.success('已下发远程启动', `${pile.name} · ${connector} 号枪`)
        onClose()
      }
      invalidate()
    },
    onError: (e) => toast.error('远程启动失败', errorMessage(e)),
  })

  const disabled = !pile.online || startable.length === 0 || !connector

  return (
    <Modal
      open
      onClose={mutation.isPending ? () => undefined : onClose}
      title="远程启动充电"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            取消
          </Button>
          <Button onClick={() => mutation.mutate()} loading={mutation.isPending} disabled={disabled}>
            启动
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
          <span className="font-medium text-slate-700">{pile.name}</span>
          <span className="mx-1.5">·</span>
          <span className="font-mono">{pile.pile_code}</span>
          <span className="mx-1.5">·</span>
          {pile.power_kw} kW
          <span className="mx-1.5">·</span>
          <span className={pile.online ? 'text-emerald-600' : 'text-red-500'}>{pile.online ? '在线' : '离线'}</span>
        </div>
        {!pile.online && (
          <div className="flex gap-2 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            桩当前离线，无法下发远程指令。
          </div>
        )}
        {pile.online && startable.length === 0 && (
          <div className="flex gap-2 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            没有可启动的连接器（需为空闲或已插枪且无进行中事务）。
          </div>
        )}
        <div className="space-y-1.5">
          <label htmlFor="rs-connector" className="block text-sm font-medium text-slate-700">
            连接器<span className="ml-0.5 text-red-500">*</span>
          </label>
          <Select id="rs-connector" options={connectorOptions} value={connector} onChange={(e) => setConnector(e.target.value)} placeholder={connectorOptions.length === 0 ? '无连接器' : undefined} />
        </div>
        <div className="space-y-1.5">
          <label htmlFor="rs-idtag" className="block text-sm font-medium text-slate-700">
            卡号（id_tag）
          </label>
          <Input id="rs-idtag" value={idTag} onChange={(e) => setIdTag(e.target.value)} placeholder={profile ? `缺省使用 ${profile.name} 的第一张有效卡` : '缺省使用当前用户的第一张有效卡'} maxLength={32} className="font-mono" />
          <p className="text-xs text-slate-400">留空时按当前登录用户归属；填写其他人的卡号则事务归属该持卡人。</p>
        </div>
        {canPickVehicle && (
          <div className="space-y-1.5">
            <label htmlFor="rs-vehicle" className="block text-sm font-medium text-slate-700">
              归属车辆
            </label>
            <Select id="rs-vehicle" options={vehicleOptions} value={vehicleId} onChange={(e) => setVehicleId(e.target.value)} placeholder={vehicleQuery.isError ? '车辆数据暂不可用' : '可选，不选则按桩位 / 最近行程自动匹配'} />
          </div>
        )}
      </div>
    </Modal>
  )
}

/** 远程启动（charging:manage）：选择连接器、卡号（缺省当前用户）、可选归属车辆 */
export function RemoteStartModal({ target, onClose }: RemoteStartModalProps) {
  if (!target) return null
  return <RemoteStartForm key={`${target.pile.id}-${target.connectorId ?? ''}`} pile={target.pile} connectorId={target.connectorId} onClose={onClose} />
}

// ── 远程停止 ──────────────────────────────────────────
export interface RemoteStopTarget {
  pileId: string
  pileName: string
  transaction: ChargeTransaction
}

interface RemoteStopDialogProps {
  target: RemoteStopTarget | null
  onClose: () => void
  onSuccess?: () => void
}

/** 远程停止（charging:manage）：二次确认后下发 RemoteStopTransaction */
export function RemoteStopDialog({ target, onClose, onSuccess }: RemoteStopDialogProps) {
  const toast = useToast()
  const invalidate = useInvalidateCharging()
  const mutation = useMutation({
    mutationFn: (t: RemoteStopTarget) => remoteStopCharging(t.pileId, t.transaction.id),
    onSuccess: (res, t) => {
      if (res?.status === 'Rejected') toast.error('桩拒绝了停止请求', '事务可能已结束，稍后刷新查看')
      else toast.success('已下发远程停止', `${t.pileName} · ${t.transaction.tx_no}`)
      invalidate()
      onClose()
      onSuccess?.()
    },
    onError: (e) => toast.error('远程停止失败', errorMessage(e)),
  })
  const tx = target?.transaction
  return (
    <ConfirmDialog
      open={target !== null}
      danger
      title={`停止 ${target?.pileName ?? ''} ${tx?.connector_id ?? ''} 号枪的充电？`}
      description={
        tx ? (
          <span>
            事务 <span className="font-mono">{tx.tx_no}</span>
            {tx.vehicle && <> · {tx.vehicle.plate_no}</>}
            {tx.user && <> · {tx.user.name}</>}
            {typeof tx.kwh === 'number' && <> · 已充 {tx.kwh.toFixed(2)} kWh</>}
            。停止后按桩侧计量结算，并与 BMS 估算交叉校验。
          </span>
        ) : undefined
      }
      confirmText="停止充电"
      loading={mutation.isPending}
      onConfirm={() => {
        if (target) mutation.mutate(target)
      }}
      onCancel={onClose}
    />
  )
}

// ── 复核 ──────────────────────────────────────────────
interface ReviewModalProps {
  transaction: ChargeTransaction | null
  onClose: () => void
  onSuccess?: (updated: ChargeTransaction) => void
}

function MeterCompare({ t }: { t: ChargeTransaction }) {
  const over = deviationExceeded(t.deviation_pct)
  const cells: Array<{ label: string; value: string; sub?: string; cls?: string }> = [
    { label: '桩侧计量', value: formatKwh(t.kwh), sub: typeof t.meter_start === 'number' ? `${t.meter_start} → ${t.meter_stop ?? '—'} Wh` : undefined, cls: 'text-slate-800' },
    { label: 'BMS 估算', value: formatKwh(t.bms_kwh_est), sub: `SOC ${formatPct(t.bms_soc_start, 0)} → ${formatPct(t.bms_soc_end, 0)}`, cls: 'text-slate-800' },
    { label: '偏差', value: formatPct(t.deviation_pct), sub: over ? '超过 5% 阈值' : '在阈值内', cls: over ? 'text-red-600' : 'text-emerald-600' },
  ]
  return (
    <div className="grid grid-cols-3 gap-2">
      {cells.map((c) => (
        <div key={c.label} className={clsx('rounded-xl p-3', c.cls === 'text-red-600' ? 'bg-red-50' : 'bg-slate-50')}>
          <div className="text-xs text-slate-400">{c.label}</div>
          <div className={clsx('mt-0.5 text-base font-semibold', c.cls)}>{c.value}</div>
          {c.sub && <div className="text-[11px] text-slate-400">{c.sub}</div>}
        </div>
      ))}
    </div>
  )
}

function ReviewForm({ t, onClose, onSuccess }: { t: ChargeTransaction; onClose: () => void; onSuccess?: (updated: ChargeTransaction) => void }) {
  const toast = useToast()
  const invalidate = useInvalidateCharging()
  const { can } = usePermission()
  const canPickVehicle = can('asset:vehicle:view')
  const { options: vehicleOptions, query: vehicleQuery } = useVehicleOptions(canPickVehicle)
  const [vehicleId, setVehicleId] = useState(t.vehicle?.id ?? '')
  const [user, setUser] = useState<UserBrief | null>(t.user ?? null)
  const [note, setNote] = useState('')
  const [pendingAction, setPendingAction] = useState<ReviewAction | null>(null)

  const reason = reviewReason(t)
  const vehicleChanged = (vehicleId || '') !== (t.vehicle?.id ?? '')
  const userChanged = (user?.id ?? '') !== (t.user?.id ?? '')

  const mutation = useMutation({
    mutationFn: (action: ReviewAction) =>
      reviewChargeTransaction(t.id, {
        action,
        note: note.trim() || undefined,
        vehicle_id: action === 'approve' && vehicleChanged && vehicleId ? vehicleId : undefined,
        user_id: action === 'approve' && userChanged && user ? user.id : undefined,
      }),
    onMutate: (action) => setPendingAction(action),
    onSuccess: (updated, action) => {
      if (action === 'approve') toast.success('复核通过，已计费', typeof updated.cost === 'number' ? `费用 ¥ ${formatMoney(updated.cost)}` : undefined)
      else toast.success('已拒绝，该事务不计费')
      invalidate()
      onSuccess?.(updated)
      onClose()
    },
    onError: (e) => toast.error('复核失败', errorMessage(e)),
    onSettled: () => setPendingAction(null),
  })

  const approveDisabled = !vehicleId && !t.vehicle

  return (
    <Modal
      open
      onClose={mutation.isPending ? () => undefined : onClose}
      title="复核充电事务"
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            取消
          </Button>
          <Button variant="danger" onClick={() => mutation.mutate('reject')} loading={pendingAction === 'reject'} disabled={mutation.isPending && pendingAction !== 'reject'}>
            拒绝（不计费）
          </Button>
          <Button icon={CheckCircle2} onClick={() => mutation.mutate('approve')} loading={pendingAction === 'approve'} disabled={approveDisabled || (mutation.isPending && pendingAction !== 'approve')} title={approveDisabled ? '请先指定归属车辆' : undefined}>
            通过并计费
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <div className="flex flex-wrap items-center gap-2 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
          <span className="font-mono text-slate-700">{t.tx_no}</span>
          <span>·</span>
          <span>
            {t.pile_name} {t.connector_id} 号枪
          </span>
          <span>·</span>
          <span>{formatTimeRange(t.start_at, t.end_at, '充电中')}</span>
          <Badge color={REVIEW_STATUS_BADGE[t.review_status]}>{REVIEW_STATUS_LABEL[t.review_status]}</Badge>
          <Badge color={reason.kind === 'other' ? 'gray' : 'amber'} icon={AlertTriangle}>
            {reason.label}
          </Badge>
        </div>

        <section>
          <h4 className="mb-2 text-sm font-semibold text-slate-700">计量与交叉校验</h4>
          <MeterCompare t={t} />
        </section>

        <section>
          <h4 className="mb-2 text-sm font-semibold text-slate-700">当前归属</h4>
          <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-4">
            <div>
              <div className="text-xs text-slate-400">用户</div>
              <div className="text-slate-800">{t.user?.name ?? <span className="text-slate-400">未识别</span>}</div>
            </div>
            <div>
              <div className="text-xs text-slate-400">部门</div>
              <div className="text-slate-800">{text(t.dept_name ?? t.user?.dept_name)}</div>
            </div>
            <div>
              <div className="text-xs text-slate-400">车辆</div>
              <div className="text-slate-800">{t.vehicle?.plate_no ?? <span className="text-amber-600">未绑定</span>}</div>
            </div>
            <div>
              <div className="text-xs text-slate-400">绑定方式 / 归属级别</div>
              <div className="text-slate-800">
                {bindMethodLabel(t.bind_method)}
                {attributionLabel(t.attribution) && <span className="ml-1 text-xs text-slate-400">· {attributionLabel(t.attribution)}</span>}
              </div>
            </div>
          </div>
        </section>

        <section>
          <h4 className="mb-2 text-sm font-semibold text-slate-700">修正归属（通过时生效）</h4>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <label htmlFor="rv-vehicle" className="block text-sm font-medium text-slate-700">
                车辆{!t.vehicle && <span className="ml-0.5 text-red-500">*</span>}
              </label>
              {canPickVehicle ? (
                <Select id="rv-vehicle" options={vehicleOptions} value={vehicleId} onChange={(e) => setVehicleId(e.target.value)} placeholder={vehicleQuery.isError ? '车辆数据暂不可用' : t.vehicle ? '保持当前车辆' : '请选择车辆'} invalid={approveDisabled} />
              ) : (
                <div className="text-xs text-slate-400">无车辆档案查看权限，无法修正车辆</div>
              )}
              {vehicleChanged && vehicleId && <p className="text-xs text-amber-600">将把车辆改为所选车辆</p>}
            </div>
            <div className="space-y-1.5">
              <label htmlFor="rv-user" className="block text-sm font-medium text-slate-700">
                用户
              </label>
              <UserPicker id="rv-user" value={user} onChange={setUser} placeholder="保持当前用户" />
              {userChanged && <p className="text-xs text-amber-600">{user ? `将把用户改为 ${user.name}` : '将清空用户（按车辆归属计费）'}</p>}
            </div>
          </div>
        </section>

        <div className="space-y-1.5">
          <label htmlFor="rv-note" className="block text-sm font-medium text-slate-700">
            复核备注
          </label>
          <Textarea id="rv-note" value={note} onChange={(e) => setNote(e.target.value)} placeholder="可选：如“BMS 上报延迟，按桩侧计量为准”" maxLength={500} />
        </div>
        <p className="text-xs text-slate-400">通过：按当前生效计费规则计价并从归属账户扣费，状态变为已结算；拒绝：不计费，保留记录。</p>
      </div>
    </Modal>
  )
}

/** 复核（charging:review）：核对桩侧 kWh 与 BMS 估算，修正归属后通过 / 拒绝 */
export function ReviewModal({ transaction, onClose, onSuccess }: ReviewModalProps) {
  if (!transaction) return null
  return <ReviewForm key={transaction.id} t={transaction} onClose={onClose} onSuccess={onSuccess} />
}
