import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle } from 'lucide-react'
import { useState } from 'react'
import { approvalKeys } from '../../api/approvals'
import { errorMessage } from '../../api/client'
import { dashboardKeys } from '../../api/dashboard'
import { cancelTrip, endTrip, tripKeys } from '../../api/trips'
import type { Trip } from '../../api/types'
import { vehicleKeys } from '../../api/vehicles'
import Button from '../../components/ui/Button'
import Input from '../../components/ui/Input'
import Modal from '../../components/ui/Modal'
import Textarea from '../../components/ui/Textarea'
import { useToast } from '../../components/ui/toast-context'
import { formatTimeRange } from '../../utils/format'

interface BaseProps {
  trip: Trip | null
  onClose: () => void
  onSuccess?: (updated: Trip) => void
}

function useInvalidateTrip() {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: tripKeys.all })
    void queryClient.invalidateQueries({ queryKey: approvalKeys.all })
    void queryClient.invalidateQueries({ queryKey: vehicleKeys.all })
    void queryClient.invalidateQueries({ queryKey: dashboardKeys.all })
  }
}

function Summary({ t }: { t: Trip }) {
  return (
    <div className="rounded-lg bg-surface-3 px-3 py-2 text-xs text-ink-muted">
      <span className="font-mono text-ink">{t.trip_no}</span>
      <span className="mx-1.5">·</span>
      {t.vehicle.plate_no}
      {t.driver && (
        <>
          <span className="mx-1.5">·</span>
          {t.driver.name}
        </>
      )}
      <span className="mx-1.5">·</span>
      {formatTimeRange(t.start_at, t.end_at)}
    </div>
  )
}

function numOrUndefined(v: string): number | undefined {
  const t = v.trim()
  if (t === '') return undefined
  const n = Number(t)
  return Number.isFinite(n) ? n : undefined
}

function EndTripForm({ t, onClose, onSuccess }: { t: Trip; onClose: () => void; onSuccess?: (updated: Trip) => void }) {
  const toast = useToast()
  const invalidate = useInvalidateTrip()
  const [odometer, setOdometer] = useState('')
  const [soc, setSoc] = useState('')
  const [remark, setRemark] = useState('')
  const [touched, setTouched] = useState(false)

  const odo = numOrUndefined(odometer)
  const socN = numOrUndefined(soc)
  const odoInvalid = odometer.trim() !== '' && (odo === undefined || odo < 0 || (typeof t.start_odometer === 'number' && odo < t.start_odometer))
  const socInvalid = soc.trim() !== '' && (socN === undefined || socN < 0 || socN > 100)

  const mutation = useMutation({
    mutationFn: () => endTrip(t.id, { end_odometer: odo, end_soc: socN, remark: remark.trim() || undefined }),
    onSuccess: (updated) => {
      toast.success('行程已结束', `里程 ${updated.distance_km?.toFixed(1) ?? '—'} km`)
      invalidate()
      onSuccess?.(updated)
      onClose()
    },
    onError: (e) => toast.error('结束行程失败', errorMessage(e)),
  })

  const submit = () => {
    setTouched(true)
    if (odoInvalid || socInvalid) return
    mutation.mutate()
  }

  return (
    <Modal
      open
      onClose={mutation.isPending ? () => undefined : onClose}
      title="结束行程"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            取消
          </Button>
          <Button onClick={submit} loading={mutation.isPending}>
            确认结束
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Summary t={t} />
        <div className="text-xs text-ink-muted">结束后按轨迹汇总里程、能耗、速度与急加减速；车辆回到空闲，关联申请置为已完成。里程表 / SOC 留空时取最后一条遥测。</div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1.5">
            <label htmlFor="end-odo" className="block text-sm font-medium text-ink">
              结束里程表（km）
            </label>
            <Input id="end-odo" type="number" inputMode="decimal" min={0} step="0.1" placeholder={typeof t.start_odometer === 'number' ? `起始 ${t.start_odometer.toFixed(1)}` : '可选'} value={odometer} onChange={(e) => setOdometer(e.target.value)} invalid={touched && odoInvalid} />
            {touched && odoInvalid && (
              <p className="text-xs text-danger-200" role="alert">
                须为不小于起始里程的数字
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label htmlFor="end-soc" className="block text-sm font-medium text-ink">
              结束 SOC（%）
            </label>
            <Input id="end-soc" type="number" inputMode="decimal" min={0} max={100} step="1" placeholder={typeof t.start_soc === 'number' ? `起始 ${Math.round(t.start_soc)}%` : '可选'} value={soc} onChange={(e) => setSoc(e.target.value)} invalid={touched && socInvalid} />
            {touched && socInvalid && (
              <p className="text-xs text-danger-200" role="alert">
                须为 0–100 的数字
              </p>
            )}
          </div>
        </div>
        <div className="space-y-1.5">
          <label htmlFor="end-remark" className="block text-sm font-medium text-ink">
            备注
          </label>
          <Textarea id="end-remark" placeholder="可选" value={remark} onChange={(e) => setRemark(e.target.value)} maxLength={500} />
        </div>
      </div>
    </Modal>
  )
}

/** 手动结束行程（trip:manage，行程须为 ongoing） */
export function EndTripModal({ trip, onClose, onSuccess }: BaseProps) {
  if (!trip) return null
  return <EndTripForm key={trip.id} t={trip} onClose={onClose} onSuccess={onSuccess} />
}

function CancelTripForm({ t, onClose, onSuccess }: { t: Trip; onClose: () => void; onSuccess?: (updated: Trip) => void }) {
  const toast = useToast()
  const invalidate = useInvalidateTrip()
  const [reason, setReason] = useState('')

  const mutation = useMutation({
    mutationFn: () => cancelTrip(t.id, reason.trim() ? { reason: reason.trim() } : {}),
    onSuccess: (updated) => {
      toast.success('行程已作废')
      invalidate()
      onSuccess?.(updated)
      onClose()
    },
    onError: (e) => toast.error('作废失败', errorMessage(e)),
  })

  return (
    <Modal
      open
      onClose={mutation.isPending ? () => undefined : onClose}
      title="作废行程"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            取消
          </Button>
          <Button variant="danger" onClick={() => mutation.mutate()} loading={mutation.isPending}>
            确认作废
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="flex gap-2 text-sm text-ink">
          <AlertTriangle size={18} className="mt-0.5 shrink-0 text-danger-200" />
          <span>用于误触发等情况：车辆回到空闲，关联申请回到已批准状态，轨迹与事件保留但不计入统计。</span>
        </div>
        <Summary t={t} />
        <div className="space-y-1.5">
          <label htmlFor="cancel-reason" className="block text-sm font-medium text-ink">
            作废原因
          </label>
          <Textarea id="cancel-reason" placeholder="可选，如：网关误触发" value={reason} onChange={(e) => setReason(e.target.value)} maxLength={500} />
        </div>
      </div>
    </Modal>
  )
}

/** 作废进行中的行程（trip:manage） */
export function CancelTripModal({ trip, onClose, onSuccess }: BaseProps) {
  if (!trip) return null
  return <CancelTripForm key={trip.id} t={trip} onClose={onClose} onSuccess={onSuccess} />
}
