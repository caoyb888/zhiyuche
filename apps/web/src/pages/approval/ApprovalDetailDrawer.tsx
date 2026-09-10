import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { Check, ChevronRight, Clock, ExternalLink, Minus, Paperclip, Play, Undo2, X } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { approvalKeys, getApproval } from '../../api/approvals'
import { errorMessage } from '../../api/client'
import type { Approval, ApprovalStep, ApprovalStepAction } from '../../api/types'
import { RoutePreview } from '../../components/map'
import { VEHICLE_STATUS_BADGE, VEHICLE_STATUS_LABEL, formatKm } from '../../components/map/vehicleStyle'
import Badge from '../../components/ui/Badge'
import Button from '../../components/ui/Button'
import DescriptionList from '../../components/ui/DescriptionList'
import Drawer from '../../components/ui/Drawer'
import ErrorState from '../../components/ui/ErrorState'
import Spinner from '../../components/ui/Spinner'
import { usePermission } from '../../hooks/usePermission'
import { formatDateTime, formatTimeRange, text } from '../../utils/format'
import { toLngLat } from '../../utils/geo'
import { ApproveModal, CancelModal, RejectModal, StartTripModal } from './ApprovalActionModals'
import { APPROVAL_STATUS_BADGE, APPROVAL_STATUS_LABEL, STEP_ACTION_BADGE, STEP_ACTION_LABEL, TRIP_TYPE_BADGE, TRIP_TYPE_LABEL, URGENCY_LABEL, isPendingStatus, purposeText } from './style'
import { TRIP_STATUS_BADGE, TRIP_STATUS_LABEL } from '../trips/style'

interface ApprovalDetailDrawerProps {
  id: string | null
  onClose: () => void
}

const STEP_ICON: Record<ApprovalStepAction, { icon: typeof Check; cls: string }> = {
  pending: { icon: Clock, cls: 'bg-warn-500/20 text-warn-200' },
  approved: { icon: Check, cls: 'bg-ev-500/20 text-ev-200' },
  rejected: { icon: X, cls: 'bg-danger-500/20 text-danger-200' },
  skipped: { icon: Minus, cls: 'bg-surface-4 text-ink-faint' },
}

function StepTimeline({ a }: { a: Approval }) {
  const steps = [...a.steps].sort((x, y) => x.step_no - y.step_no)
  if (steps.length === 0) return <div className="text-xs text-ink-faint">暂无审批步骤</div>
  const pending = isPendingStatus(a.status)
  return (
    <ol className="space-y-0">
      {steps.map((s: ApprovalStep, i) => {
        const meta = STEP_ICON[s.action]
        const Icon = meta.icon
        const current = pending && s.step_no === a.current_step
        const last = i === steps.length - 1
        return (
          <li key={s.step_no} className="relative flex gap-3 pb-4">
            {!last && <span className="absolute left-3 top-6 h-[calc(100%-0.5rem)] w-px bg-line-strong" aria-hidden />}
            <span className={clsx('relative z-[1] flex h-6 w-6 shrink-0 items-center justify-center rounded-full', meta.cls, current && 'ring-2 ring-warn-500/40 ring-offset-1')}>
              <Icon size={13} />
            </span>
            <div className={clsx('min-w-0 flex-1 rounded-lg px-3 py-2', current ? 'bg-warn-500/10' : 'bg-surface-3')}>
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
                <span className="text-xs text-ink-faint">第 {s.step_no} 级</span>
                <span className="font-medium text-ink-strong">{s.approver.name}</span>
                {s.approver.dept_name && <span className="text-xs text-ink-faint">{s.approver.dept_name}</span>}
                <Badge color={STEP_ACTION_BADGE[s.action]}>{current ? '当前步骤' : STEP_ACTION_LABEL[s.action]}</Badge>
              </div>
              {s.remark && <div className="mt-1 text-xs text-ink">备注：{s.remark}</div>}
              <div className="mt-1 text-[11px] text-ink-faint">{s.acted_at ? formatDateTime(s.acted_at) : current ? '等待处理' : '—'}</div>
            </div>
          </li>
        )
      })}
    </ol>
  )
}

function DetailBody({ id, onClose }: { id: string; onClose: () => void }) {
  const { can } = usePermission()
  const detail = useQuery({ queryKey: approvalKeys.detail(id), queryFn: () => getApproval(id) })
  const [action, setAction] = useState<'approve' | 'reject' | 'cancel' | 'start' | null>(null)

  if (detail.isPending) {
    return (
      <div className="flex justify-center py-10">
        <Spinner label="加载中" />
      </div>
    )
  }
  if (detail.isError) return <ErrorState message={errorMessage(detail.error)} onRetry={() => void detail.refetch()} />

  const a = detail.data
  const dest = toLngLat(a.dest_lng, a.dest_lat)
  const canApprove = a.can_approve && can('approval:approve')
  const canCancel = a.can_cancel
  // 行程作废后申请回到 approved（trip 仍指向已作废行程），此时允许再次手动开始
  const canStart = can('trip:manage') && a.status === 'approved'
  const hasActions = canApprove || canCancel || canStart
  const target = action ? a : null

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-base font-semibold text-ink-strong">{a.apply_no}</span>
        <Badge color={APPROVAL_STATUS_BADGE[a.status]}>{APPROVAL_STATUS_LABEL[a.status]}</Badge>
        <Badge color={TRIP_TYPE_BADGE[a.trip_type]}>{TRIP_TYPE_LABEL[a.trip_type]}</Badge>
        {a.urgency === 'urgent' && <Badge color="red">{URGENCY_LABEL.urgent}</Badge>}
        {a.level_required === 2 && <span className="text-xs text-ink-faint">需二级审批</span>}
      </div>

      {(a.reject_reason || a.cancel_reason) && (
        <div className={clsx('rounded-xl border p-3 text-sm', a.reject_reason ? 'border-danger-500/30 bg-danger-500/10 text-danger-200' : 'border-line-strong bg-surface-3 text-ink')}>
          <div className="mb-0.5 text-xs font-medium">{a.reject_reason ? '驳回原因' : '撤销原因'}</div>
          {a.reject_reason ?? a.cancel_reason}
        </div>
      )}

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">申请信息</h4>
        <DescriptionList
          columns={2}
          items={[
            {
              label: '申请人',
              value: (
                <span>
                  {a.applicant.name}
                  {a.dept_name && <span className="ml-1 text-xs text-ink-faint">{a.dept_name}</span>}
                </span>
              ),
            },
            { label: '事由', value: purposeText(a) },
            { label: '计划时段', value: formatTimeRange(a.planned_start, a.planned_end) },
            { label: '预计里程', value: formatKm(a.planned_km, 1) },
            { label: '目的地', value: a.destination, span: 2 },
            { label: '具体说明', value: a.purpose_detail, span: 2 },
            {
              label: '车辆',
              value: a.vehicle ? (
                <span className="flex flex-wrap items-center gap-1.5">
                  <span className="font-medium">{a.vehicle.plate_no}</span>
                  {a.vehicle.model && <span className="text-xs text-ink-faint">{a.vehicle.model}</span>}
                  <Badge color={VEHICLE_STATUS_BADGE[a.vehicle.status]}>{VEHICLE_STATUS_LABEL[a.vehicle.status]}</Badge>
                </span>
              ) : (
                <span className="text-ink-faint">未指定（由审批人指派）</span>
              ),
            },
            { label: '随行人员', value: a.passengers && a.passengers.length > 0 ? a.passengers.map((p) => p.name).join('、') : null },
            { label: '提交时间', value: formatDateTime(a.created_at) },
            { label: '批准时间', value: formatDateTime(a.approved_at) },
          ]}
        />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">目的地与计划路线</h4>
        <RoutePreview destination={dest} destinationLabel={a.destination || '目的地'} route={a.planned_route ?? null} height={240} />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">审批流</h4>
        <StepTimeline a={a} />
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">关联行程</h4>
        {a.trip ? (
          <Link to={`/trips?id=${encodeURIComponent(a.trip.id)}`} className="flex items-center justify-between rounded-xl border border-line bg-surface-3 px-3 py-2.5 text-sm transition-colors hover:border-brand-600/40 hover:bg-brand-600/10">
            <span className="flex flex-wrap items-center gap-2">
              <span className="font-mono font-medium text-ink-strong">{a.trip.trip_no}</span>
              <Badge color={TRIP_STATUS_BADGE[a.trip.status]}>{TRIP_STATUS_LABEL[a.trip.status]}</Badge>
              <span className="text-xs text-ink-muted">{formatTimeRange(a.trip.start_at, a.trip.end_at)}</span>
              <span className="text-xs text-ink-muted">{formatKm(a.trip.distance_km, 1)}</span>
            </span>
            <ChevronRight size={16} className="shrink-0 text-ink-disabled" />
          </Link>
        ) : (
          <div className="text-xs text-ink-faint">{a.status === 'approved' ? '已批准，等待刷卡取车或手动开始行程' : '暂无关联行程'}</div>
        )}
        {a.trip && a.trip.status === 'cancelled' && a.status === 'approved' && (
          <div className="mt-2 text-xs text-ink-faint">上一次行程已作废，申请已回到「已批准」，可重新刷卡取车或手动开始行程</div>
        )}
      </section>

      <section>
        <h4 className="mb-3 text-sm font-semibold text-ink">附件</h4>
        {a.attachments && a.attachments.length > 0 ? (
          <ul className="space-y-1.5">
            {a.attachments.map((f, i) => (
              <li key={`${f.url ?? ''}-${i}`}>
                <a href={f.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-sm text-brand-300 hover:underline">
                  <Paperclip size={13} />
                  {text(f.name) === '—' ? f.url : f.name}
                  <ExternalLink size={11} className="text-ink-faint" />
                </a>
              </li>
            ))}
          </ul>
        ) : (
          <div className="text-xs text-ink-faint">无附件</div>
        )}
      </section>

      {hasActions && (
        <div className="sticky bottom-0 -mx-5 -mb-4 flex flex-wrap items-center justify-end gap-2 border-t border-line bg-surface-2 px-5 py-3">
          {canStart && (
            <Button variant="secondary" icon={Play} onClick={() => setAction('start')}>
              手动开始行程
            </Button>
          )}
          {canCancel && (
            <Button variant="secondary" icon={Undo2} className="text-ink" onClick={() => setAction('cancel')}>
              撤销
            </Button>
          )}
          {canApprove && (
            <>
              <Button variant="danger" icon={X} onClick={() => setAction('reject')}>
                驳回
              </Button>
              <Button icon={Check} onClick={() => setAction('approve')}>
                通过
              </Button>
            </>
          )}
        </div>
      )}

      <ApproveModal approval={action === 'approve' ? target : null} onClose={() => setAction(null)} />
      <RejectModal approval={action === 'reject' ? target : null} onClose={() => setAction(null)} />
      <CancelModal approval={action === 'cancel' ? target : null} onClose={() => setAction(null)} />
      <StartTripModal approval={action === 'start' ? target : null} onClose={() => setAction(null)} onSuccess={() => onClose()} />
    </div>
  )
}

/** 申请详情抽屉：申请信息 + 路线预览 + 审批流时间线 + 关联行程 + 附件 + 操作区 */
export default function ApprovalDetailDrawer({ id, onClose }: ApprovalDetailDrawerProps) {
  return (
    <Drawer open={id !== null} onClose={onClose} title="申请详情" width="lg">
      {id && <DetailBody key={id} id={id} onClose={onClose} />}
    </Drawer>
  )
}
