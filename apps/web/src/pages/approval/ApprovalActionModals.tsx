import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { AlertTriangle } from 'lucide-react'
import { useMemo, useState } from 'react'
import { approvalKeys, approveApproval, cancelApproval, listAvailableVehicles, rejectApproval } from '../../api/approvals'
import { errorMessage } from '../../api/client'
import { dashboardKeys } from '../../api/dashboard'
import { startTrip, tripKeys } from '../../api/trips'
import type { Approval, Trip, UserBrief } from '../../api/types'
import { VEHICLE_STATUS_LABEL } from '../../components/map/vehicleStyle'
import Button from '../../components/ui/Button'
import Checkbox from '../../components/ui/Checkbox'
import Input from '../../components/ui/Input'
import Modal from '../../components/ui/Modal'
import Select, { type SelectOption } from '../../components/ui/Select'
import Spinner from '../../components/ui/Spinner'
import Textarea from '../../components/ui/Textarea'
import { useToast } from '../../components/ui/toast-context'
import UserPicker from '../../components/UserPicker'
import { usePermission } from '../../hooks/usePermission'
import { formatTimeRange } from '../../utils/format'
import { vehicleOptionLabel } from './style'

interface BaseModalProps {
  /** 为 null 时关闭 */
  approval: Approval | null
  onClose: () => void
}

function useInvalidateApproval() {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: approvalKeys.all })
    void queryClient.invalidateQueries({ queryKey: dashboardKeys.all })
  }
}

function Summary({ a }: { a: Approval }) {
  return (
    <div className="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
      <span className="font-mono text-slate-700">{a.apply_no}</span>
      <span className="mx-1.5">·</span>
      {a.applicant.name}
      <span className="mx-1.5">·</span>
      {formatTimeRange(a.planned_start, a.planned_end)}
      <span className="mx-1.5">·</span>
      {a.destination}
    </div>
  )
}

// ── 通过 ─────────────────────────────────────────────

interface ApproveModalProps extends BaseModalProps {
  onSuccess?: (updated: Approval) => void
}

function ApproveForm({ a, onClose, onSuccess }: { a: Approval; onClose: () => void; onSuccess?: (updated: Approval) => void }) {
  const toast = useToast()
  const { can } = usePermission()
  const invalidate = useInvalidateApproval()
  const needVehicle = !a.vehicle
  const canReassign = can('approval:manage') && !needVehicle
  const [remark, setRemark] = useState('')
  const [reassign, setReassign] = useState(false)
  const [vehicleId, setVehicleId] = useState('')
  const [touched, setTouched] = useState(false)

  const pickVehicle = needVehicle || reassign
  const q = { start: a.planned_start, end: a.planned_end, exclude_approval_id: a.id }
  const available = useQuery({ queryKey: approvalKeys.availableVehicles(q), queryFn: () => listAvailableVehicles(q), enabled: pickVehicle, staleTime: 15_000 })

  const options = useMemo<SelectOption[]>(() => {
    const list = (available.data ?? []).map((v) => ({ value: v.id, label: vehicleOptionLabel(v) }))
    // 改派时保留当前车辆作为选项（它在可用列表中被自身申请占用而排除）
    if (a.vehicle && !list.some((o) => o.value === a.vehicle?.id)) {
      list.unshift({ value: a.vehicle.id, label: `${a.vehicle.plate_no}${a.vehicle.model ? ` · ${a.vehicle.model}` : ''}（当前车辆 · ${VEHICLE_STATUS_LABEL[a.vehicle.status]}）` })
    }
    return list
  }, [available.data, a.vehicle])

  const vehicleInvalid = pickVehicle && !vehicleId
  const mutation = useMutation({
    mutationFn: () => approveApproval(a.id, { remark: remark.trim() || undefined, vehicle_id: pickVehicle && vehicleId ? vehicleId : undefined }),
    onSuccess: (updated) => {
      toast.success(updated.status === 'pending_l2' ? '一级审批已通过，等待二级审批' : '申请已批准')
      invalidate()
      onSuccess?.(updated)
      onClose()
    },
    onError: (e) => toast.error('审批失败', errorMessage(e)),
  })

  const submit = () => {
    setTouched(true)
    if (vehicleInvalid) return
    mutation.mutate()
  }

  return (
    <Modal
      open
      onClose={mutation.isPending ? () => undefined : onClose}
      title="审批通过"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            取消
          </Button>
          <Button onClick={submit} loading={mutation.isPending}>
            确认通过
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Summary a={a} />
        {a.status === 'pending_l1' && a.level_required === 2 && <div className="text-xs text-slate-500">该申请需二级审批：本步骤通过后将转交二级审批人。</div>}

        {!needVehicle && (
          <div className="text-sm">
            <span className="text-xs text-slate-400">申请车辆</span>
            <div className="mt-0.5 flex flex-wrap items-center gap-3">
              <span className="font-medium text-slate-800">
                {a.vehicle?.plate_no}
                {a.vehicle?.model && <span className="ml-1 text-xs text-slate-400">{a.vehicle.model}</span>}
              </span>
              {canReassign && <Checkbox label="改派其他车辆" checked={reassign} onChange={(e) => setReassign(e.target.checked)} />}
            </div>
          </div>
        )}

        {pickVehicle && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-slate-700">
              {needVehicle ? '指派车辆' : '改派为'}
              <span className="ml-0.5 text-red-500">*</span>
            </label>
            {available.isPending ? (
              <div className="flex items-center gap-2 py-1 text-xs text-slate-400">
                <Spinner size="sm" /> 正在查询该时段可用车辆…
              </div>
            ) : available.isError ? (
              <div className="text-xs text-red-500">可用车辆查询失败：{errorMessage(available.error)}</div>
            ) : (
              <>
                <Select options={options} placeholder={options.length === 0 ? '该时段无可用车辆' : '请选择车辆'} value={vehicleId} onChange={(e) => setVehicleId(e.target.value)} invalid={touched && vehicleInvalid} aria-label="指派车辆" />
                {touched && vehicleInvalid && (
                  <p className="text-xs text-red-500" role="alert">
                    {needVehicle ? '申请未指定车辆，通过前必须指派车辆' : '请选择改派的车辆'}
                  </p>
                )}
                <p className="text-xs text-slate-400">仅列出 {formatTimeRange(a.planned_start, a.planned_end)} 内空闲且无冲突申请的车辆</p>
              </>
            )}
          </div>
        )}

        <div className="space-y-1.5">
          <label htmlFor="approve-remark" className="block text-sm font-medium text-slate-700">
            审批备注
          </label>
          <Textarea id="approve-remark" placeholder="可选" value={remark} onChange={(e) => setRemark(e.target.value)} maxLength={500} />
        </div>
      </div>
    </Modal>
  )
}

/** 通过弹窗：申请无车辆时必须指派；approval:manage 可改派 */
export function ApproveModal({ approval, onClose, onSuccess }: ApproveModalProps) {
  if (!approval) return null
  return <ApproveForm key={approval.id} a={approval} onClose={onClose} onSuccess={onSuccess} />
}

// ── 驳回 ─────────────────────────────────────────────

function ReasonForm({
  a,
  onClose,
  title,
  label,
  required,
  confirmText,
  danger,
  description,
  placeholder,
  mutationFn,
  successText,
  errorTitle,
  onSuccess,
}: {
  a: Approval
  onClose: () => void
  title: string
  label: string
  required: boolean
  confirmText: string
  danger?: boolean
  description?: string
  placeholder?: string
  mutationFn: (reason: string) => Promise<Approval>
  successText: string
  errorTitle: string
  onSuccess?: (updated: Approval) => void
}) {
  const toast = useToast()
  const invalidate = useInvalidateApproval()
  const [reason, setReason] = useState('')
  const [touched, setTouched] = useState(false)
  const invalid = required && !reason.trim()

  const mutation = useMutation({
    mutationFn: () => mutationFn(reason.trim()),
    onSuccess: (updated) => {
      toast.success(successText)
      invalidate()
      onSuccess?.(updated)
      onClose()
    },
    onError: (e) => toast.error(errorTitle, errorMessage(e)),
  })

  const submit = () => {
    setTouched(true)
    if (invalid) return
    mutation.mutate()
  }

  return (
    <Modal
      open
      onClose={mutation.isPending ? () => undefined : onClose}
      title={title}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            取消
          </Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={submit} loading={mutation.isPending}>
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {description && (
          <div className="flex gap-2 text-sm text-slate-600">
            <AlertTriangle size={18} className={clsx('mt-0.5 shrink-0', danger ? 'text-red-500' : 'text-amber-500')} />
            <span>{description}</span>
          </div>
        )}
        <Summary a={a} />
        <div className="space-y-1.5">
          <label htmlFor="reason-input" className="block text-sm font-medium text-slate-700">
            {label}
            {required && <span className="ml-0.5 text-red-500">*</span>}
          </label>
          <Textarea id="reason-input" placeholder={placeholder} value={reason} onChange={(e) => setReason(e.target.value)} invalid={touched && invalid} maxLength={500} />
          {touched && invalid && (
            <p className="text-xs text-red-500" role="alert">
              请填写{label}
            </p>
          )}
        </div>
      </div>
    </Modal>
  )
}

interface ReasonModalProps extends BaseModalProps {
  onSuccess?: (updated: Approval) => void
}

/** 驳回弹窗：原因必填 */
export function RejectModal({ approval, onClose, onSuccess }: ReasonModalProps) {
  if (!approval) return null
  return (
    <ReasonForm
      key={approval.id}
      a={approval}
      onClose={onClose}
      title="驳回申请"
      label="驳回原因"
      required
      confirmText="确认驳回"
      danger
      description="任一级驳回即终止审批流程，并通知申请人。"
      placeholder="如：该时段无可用车辆，请调整用车时间"
      mutationFn={(reason) => rejectApproval(approval.id, { reason })}
      successText="申请已驳回"
      errorTitle="驳回失败"
      onSuccess={onSuccess}
    />
  )
}

/** 撤销弹窗：原因可选 */
export function CancelModal({ approval, onClose, onSuccess }: ReasonModalProps) {
  if (!approval) return null
  return (
    <ReasonForm
      key={approval.id}
      a={approval}
      onClose={onClose}
      title="撤销申请"
      label="撤销原因"
      required={false}
      confirmText="确认撤销"
      danger
      description="撤销后申请终止，占用的车辆时段将被释放；已开始行程的申请无法撤销。"
      placeholder="可选，如：行程取消"
      mutationFn={(reason) => cancelApproval(approval.id, reason ? { reason } : {})}
      successText="申请已撤销"
      errorTitle="撤销失败"
      onSuccess={onSuccess}
    />
  )
}

// ── 手动开始行程 ──────────────────────────────────────

interface StartTripModalProps extends BaseModalProps {
  onSuccess?: (trip: Trip) => void
}

function StartTripForm({ a, onClose, onSuccess }: { a: Approval; onClose: () => void; onSuccess?: (trip: Trip) => void }) {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const [driver, setDriver] = useState<UserBrief | null>(a.applicant)
  const [remark, setRemark] = useState('')
  const canPickDriver = can('system:user:view')

  const mutation = useMutation({
    mutationFn: () => startTrip({ approval_id: a.id, driver_id: driver && driver.id !== a.applicant.id ? driver.id : undefined, remark: remark.trim() || undefined }),
    onSuccess: (trip) => {
      toast.success('行程已开始', `行程号 ${trip.trip_no}`)
      void queryClient.invalidateQueries({ queryKey: tripKeys.all })
      void queryClient.invalidateQueries({ queryKey: approvalKeys.all })
      void queryClient.invalidateQueries({ queryKey: dashboardKeys.all })
      onSuccess?.(trip)
      onClose()
    },
    onError: (e) => toast.error('开始行程失败', errorMessage(e)),
  })

  return (
    <Modal
      open
      onClose={mutation.isPending ? () => undefined : onClose}
      title="手动开始行程"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            取消
          </Button>
          <Button onClick={() => mutation.mutate()} loading={mutation.isPending}>
            开始行程
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Summary a={a} />
        <div className="text-xs text-slate-500">车辆将立即进入在途状态；通常由网关刷卡自动开始，仅在网关异常时手动调度。</div>
        <div className="space-y-1.5">
          <label htmlFor="start-trip-driver" className="block text-sm font-medium text-slate-700">
            驾驶员
          </label>
          {canPickDriver ? (
            <UserPicker id="start-trip-driver" value={driver} onChange={setDriver} placeholder="缺省为申请人" />
          ) : (
            <Input id="start-trip-driver" value={`${a.applicant.name}（申请人）`} readOnly />
          )}
          <p className="text-xs text-slate-400">缺省为申请人 {a.applicant.name}</p>
        </div>
        <div className="space-y-1.5">
          <label htmlFor="start-trip-remark" className="block text-sm font-medium text-slate-700">
            备注
          </label>
          <Textarea id="start-trip-remark" placeholder="可选" value={remark} onChange={(e) => setRemark(e.target.value)} maxLength={500} />
        </div>
      </div>
    </Modal>
  )
}

/** 手动开始行程（trip:manage，申请须为 approved） */
export function StartTripModal({ approval, onClose, onSuccess }: StartTripModalProps) {
  if (!approval) return null
  return <StartTripForm key={approval.id} a={approval} onClose={onClose} onSuccess={onSuccess} />
}
