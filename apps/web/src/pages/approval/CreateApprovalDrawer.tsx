import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import dayjs from 'dayjs'
import { AlertCircle, Briefcase, Car, CheckCircle2, Plus, ShieldCheck, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Controller, useFieldArray, useForm, useWatch } from 'react-hook-form'
import { approvalKeys, createApproval, listAvailableVehicles, precheckApproval } from '../../api/approvals'
import { errorMessage } from '../../api/client'
import { dictKeys, getDictByCode } from '../../api/dicts'
import type { Approval, ApprovalCreate, PlannedRoute, PrecheckResult, TripType } from '../../api/types'
import { listVehicleLiveStatus, vehicleKeys } from '../../api/vehicles'
import { MapPicker, useMapEngine } from '../../components/map'
import type { AMapNS } from '../../components/map/amap-types'
import Button from '../../components/ui/Button'
import DateTimeInput from '../../components/ui/DateTimeInput'
import Drawer from '../../components/ui/Drawer'
import { Form, FormField } from '../../components/ui/Form'
import Input from '../../components/ui/Input'
import Select, { type SelectOption } from '../../components/ui/Select'
import Spinner from '../../components/ui/Spinner'
import Textarea from '../../components/ui/Textarea'
import { useToast } from '../../components/ui/toast-context'
import UserPicker from '../../components/UserPicker'
import { usePermission } from '../../hooks/usePermission'
import { useAuthStore } from '../../store/auth'
import { formatTimeRange, localToRFC3339 } from '../../utils/format'
import { formatDistance, haversineMeters, isValidLngLat, toLngLat, type LngLat } from '../../utils/geo'
import { approvalFormSchema, defaultApprovalForm, toApprovalCreate, type ApprovalFormValues } from './schemas'
import { TRIP_TYPE_LABEL, URGENCY_LABEL, vehicleOptionLabel } from './style'

interface CreateApprovalDrawerProps {
  open: boolean
  onClose: () => void
  onCreated: (approval: Approval) => void
}

/** 无路线规划时按直线距离 × 1.3 估算 */
const STRAIGHT_FACTOR = 1.3

const TRIP_TYPE_DESC: Record<TripType, string> = {
  official: '公务出行，行程中亮起车顶灯牌',
  daily: '日常用车，不亮灯牌',
}

const URGENCY_OPTIONS: SelectOption[] = [
  { value: 'normal', label: URGENCY_LABEL.normal },
  { value: 'urgent', label: URGENCY_LABEL.urgent },
]

interface RoutePlan {
  route: PlannedRoute | null
  /** 直线估算（无 Key / 规划失败时） */
  straightKm: number | null
}

/** 高德驾车规划 → PlannedRoute */
async function planWithAMap(ns: AMapNS, origin: LngLat, dest: LngLat): Promise<PlannedRoute> {
  return new Promise((resolve, reject) => {
    const driving = new ns.Driving({ policy: 0, extensions: 'base' })
    driving.search(origin, dest, (status, result) => {
      if (status !== 'complete' || typeof result !== 'object' || !result.routes || result.routes.length === 0) {
        reject(new Error(typeof result === 'string' ? result : '路线规划无结果'))
        return
      }
      const r = result.routes[0]
      const points: number[][] = []
      for (const step of r.steps ?? []) {
        for (const p of step.path ?? []) {
          const lng = typeof p.getLng === 'function' ? p.getLng() : p.lng
          const lat = typeof p.getLat === 'function' ? p.getLat() : p.lat
          if (isValidLngLat([lng, lat])) points.push([lng, lat])
        }
      }
      const roads = (r.steps ?? []).map((s) => s.road).filter((s): s is string => Boolean(s))
      const summary = Array.from(new Set(roads)).slice(0, 4).join(' → ')
      resolve({ points, distance_km: Math.round((r.distance / 1000) * 10) / 10, duration_min: Math.round(r.time / 60), summary: summary || undefined })
    })
  })
}

function PrecheckCard({ r }: { r: PrecheckResult }) {
  const pass = r.problems.length === 0
  return (
    <div className={clsx('rounded-xl border p-4 text-sm', pass ? 'border-ev-500/30 bg-ev-500/15' : 'border-danger-500/30 bg-danger-500/15')}>
      <div className={clsx('flex items-center gap-2 font-medium', pass ? 'text-ev-200' : 'text-danger-200')}>
        {pass ? <CheckCircle2 size={16} /> : <AlertCircle size={16} />}
        {pass ? '预检通过，可以提交申请' : '预检未通过，请根据下列问题调整后重新预检'}
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <div>
          <div className="text-xs text-ink-faint">所需审批级别</div>
          <div className="font-medium text-ink-strong">{r.level_required} 级审批</div>
          {r.level2_reasons && r.level2_reasons.length > 0 && (
            <ul className="mt-1 list-inside list-disc text-xs text-ink-muted">
              {r.level2_reasons.map((s) => (
                <li key={s}>{s}</li>
              ))}
            </ul>
          )}
        </div>
        <div>
          <div className="text-xs text-ink-faint">审批人链</div>
          {r.approvers.length > 0 ? (
            <div className="mt-0.5 flex flex-wrap items-center gap-1">
              {r.approvers.map((u, i) => (
                <span key={`${u.id}-${i}`} className="inline-flex items-center gap-1">
                  <span className="rounded-md bg-surface-2 px-1.5 py-0.5 text-xs text-ink ring-1 ring-line-strong">
                    {i + 1}. {u.name}
                    {u.dept_name && <span className="ml-1 text-ink-faint">{u.dept_name}</span>}
                  </span>
                  {i < r.approvers.length - 1 && <span className="text-ink-disabled">→</span>}
                </span>
              ))}
            </div>
          ) : (
            <div className="text-xs text-ink-muted">未能确定审批人</div>
          )}
        </div>
      </div>
      {r.conflicts.length > 0 && (
        <div className="mt-3">
          <div className="text-xs text-ink-faint">车辆时段冲突</div>
          <ul className="mt-1 space-y-1">
            {r.conflicts.map((c, i) => (
              <li key={`${c.apply_no ?? ''}-${i}`} className="rounded-md bg-surface-2 px-2 py-1 text-xs text-ink ring-1 ring-danger-500/25">
                <span className="font-mono">{c.apply_no}</span> · {c.applicant_name} · {formatTimeRange(c.planned_start, c.planned_end)} · {c.status}
              </li>
            ))}
          </ul>
        </div>
      )}
      {r.problems.length > 0 && (
        <ul className="mt-3 list-inside list-disc space-y-0.5 text-xs text-danger-200">
          {r.problems.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      )}
    </div>
  )
}

function CreateForm({ onCancel, onCreated }: { onCancel: () => void; onCreated: (a: Approval) => void }) {
  const toast = useToast()
  const { can } = usePermission()
  const profile = useAuthStore((s) => s.profile)
  const { engine, loadAMap } = useMapEngine()
  const canManage = can('approval:manage')
  // 用户简表接口对任何登录用户开放，选人不再依赖用户管理权限
  const canSearchUsers = can('approval:view')
  const canSeeLive = can('asset:vehicle:view')

  const form = useForm<ApprovalFormValues>({ resolver: zodResolver(approvalFormSchema), defaultValues: defaultApprovalForm() })
  const attachments = useFieldArray({ control: form.control, name: 'attachments' })
  const values = useWatch({ control: form.control })
  const { planned_start: plannedStart = '', planned_end: plannedEnd = '', dest = null, vehicle_id: vehicleId = '', applicant = null, trip_type: tripType = 'official' } = values

  // ── 事由字典
  const purposes = useQuery({ queryKey: dictKeys.byCode('approval_purpose'), queryFn: () => getDictByCode('approval_purpose'), staleTime: 300_000 })
  const purposeOptions = useMemo<SelectOption[]>(() => (purposes.data ?? []).filter((d) => d.status === 'active').map((d) => ({ value: d.value, label: d.label })), [purposes.data])

  // ── 时段内可用车辆
  const startIso = localToRFC3339(plannedStart)
  const endIso = localToRFC3339(plannedEnd)
  const periodValid = Boolean(startIso && endIso && dayjs(endIso).isAfter(dayjs(startIso)))
  const availQ = { start: startIso ?? '', end: endIso ?? '' }
  const available = useQuery({ queryKey: approvalKeys.availableVehicles(availQ), queryFn: () => listAvailableVehicles(availQ), enabled: periodValid, staleTime: 15_000 })
  const vehicleOptions = useMemo<SelectOption[]>(() => (available.data ?? []).map((v) => ({ value: v.id, label: vehicleOptionLabel(v) })), [available.data])
  // 时段变化后原选车辆不再可用时清空
  useEffect(() => {
    if (available.data && vehicleId && !available.data.some((v) => v.id === vehicleId)) form.setValue('vehicle_id', '')
  }, [available.data, vehicleId, form])

  // ── 起点：所选车辆的当前位置（需 asset:vehicle:view）
  const live = useQuery({ queryKey: vehicleKeys.status, queryFn: listVehicleLiveStatus, enabled: canSeeLive, staleTime: 30_000 })
  const origin = useMemo<LngLat | null>(() => {
    if (!vehicleId || !live.data) return null
    const v = live.data.find((x) => x.vehicle_id === vehicleId)
    return v ? toLngLat(v.lng, v.lat) : null
  }, [vehicleId, live.data])
  const destPoint = useMemo<LngLat | null>(() => (dest ? toLngLat(dest.lng, dest.lat) : null), [dest])

  // ── 路线规划：有 Key 用 AMap.Driving（按起终点缓存），否则直线 × 1.3
  const straightKm = useMemo(() => (origin && destPoint ? Math.round(((haversineMeters(origin, destPoint) / 1000) * STRAIGHT_FACTOR) * 10) / 10 : null), [origin, destPoint])
  const routeQuery = useQuery({
    queryKey: ['amap', 'driving', origin, destPoint],
    queryFn: () => loadAMap().then((ns) => planWithAMap(ns, origin as LngLat, destPoint as LngLat)),
    enabled: engine === 'amap' && origin !== null && destPoint !== null,
    staleTime: Infinity,
    retry: false,
  })
  const plan: RoutePlan = { route: engine === 'amap' ? (routeQuery.data ?? null) : null, straightKm }
  const planning = engine === 'amap' && routeQuery.isFetching
  const planError = engine === 'amap' && routeQuery.isError ? errorMessage(routeQuery.error, '路线规划失败') : null
  // 自动预填预计里程：用户手动改过则不覆盖（清空后恢复自动）
  const [kmTouched, setKmTouched] = useState(false)
  useEffect(() => {
    if (straightKm === null || planning) return
    const km = typeof plan.route?.distance_km === 'number' ? plan.route.distance_km : straightKm
    if (!kmTouched || form.getValues('planned_km').trim() === '') form.setValue('planned_km', String(km), { shouldValidate: true })
  }, [plan.route, straightKm, planning, kmTouched, form])

  // ── 预检：结果绑定提交时的表单快照，表单（或规划路线）任何变化都使其失效
  const snapshot = useMemo(() => JSON.stringify([values, plan.route]), [values, plan.route])
  const [precheckState, setPrecheckState] = useState<{ snapshot: string; result: PrecheckResult } | null>(null)
  const precheck = precheckState && precheckState.snapshot === snapshot ? precheckState.result : null

  const buildBody = (v: ApprovalFormValues): ApprovalCreate => toApprovalCreate(v, plan.route)

  const precheckMutation = useMutation({
    mutationFn: async (input: { v: ApprovalFormValues; snapshot: string }) => ({ snapshot: input.snapshot, result: await precheckApproval(buildBody(input.v)) }),
    onSuccess: (r) => {
      setPrecheckState(r)
      if (r.result.problems.length > 0) toast.warning('预检未通过', r.result.problems[0])
    },
    onError: (e) => toast.error('预检失败', errorMessage(e)),
  })

  const createMutation = useMutation({
    mutationFn: (v: ApprovalFormValues) => createApproval(buildBody(v)),
    onSuccess: (a) => {
      toast.success('申请已提交', `单号 ${a.apply_no}，等待 ${a.steps[0]?.approver.name ?? '审批人'} 审批`)
      onCreated(a)
    },
    onError: (e) => toast.error('提交失败', errorMessage(e)),
  })

  const runPrecheck = form.handleSubmit((v) => precheckMutation.mutate({ v, snapshot }))
  const canSubmit = precheck !== null && precheck.problems.length === 0
  const busy = precheckMutation.isPending || createMutation.isPending

  /** 目的地文本是否由地图选点自动填入（用户手改后不再覆盖） */
  const [destAuto, setDestAuto] = useState(false)
  const applicantName = applicant?.name ?? profile?.name ?? '本人'

  return (
    <Form form={form} onSubmit={(v) => canSubmit && createMutation.mutate(v)}>
      {/* 1. 基本信息 */}
      <section className="space-y-4">
        <h4 className="text-sm font-semibold text-ink">1. 基本信息</h4>
        {canManage && (
          <FormField name="applicant" label="申请人" hint={canSearchUsers ? '代人发起：留空则为本人' : '需要「用户管理-查看」权限才能代人发起'}>
            {({ id, invalid }) => <Controller control={form.control} name="applicant" render={({ field }) => <UserPicker id={id} invalid={invalid} value={field.value} onChange={field.onChange} placeholder={`本人（${profile?.name ?? ''}）`} />} />}
          </FormField>
        )}
        <FormField name="trip_type" label="用车类型" required>
          {() => (
            <div className="grid grid-cols-2 gap-3">
              {(Object.keys(TRIP_TYPE_LABEL) as TripType[]).map((t) => {
                const active = tripType === t
                const Icon = t === 'official' ? Briefcase : Car
                return (
                  <button
                    key={t}
                    type="button"
                    role="radio"
                    aria-checked={active}
                    onClick={() => form.setValue('trip_type', t, { shouldValidate: true })}
                    className={clsx('flex items-start gap-3 rounded-xl border p-3 text-left transition-colors', active ? 'border-brand-400 bg-brand-600/15 ring-2 ring-brand-600/30' : 'border-line-strong hover:border-line-strong')}
                  >
                    <span className={clsx('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', active ? 'bg-brand-600 text-white' : 'bg-surface-4 text-ink-muted')}>
                      <Icon size={16} />
                    </span>
                    <span className="min-w-0">
                      <span className="block text-sm font-medium text-ink-strong">{TRIP_TYPE_LABEL[t]}</span>
                      <span className="block text-xs text-ink-faint">{TRIP_TYPE_DESC[t]}</span>
                    </span>
                  </button>
                )
              })}
            </div>
          )}
        </FormField>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField name="purpose_code" label="用车事由" required hint={purposes.isError ? `事由字典加载失败：${errorMessage(purposes.error)}` : undefined}>
            <Select options={purposeOptions} placeholder={purposes.isPending ? '加载中…' : purposeOptions.length === 0 ? '暂无事由字典（approval_purpose）' : '请选择事由'} disabled={purposes.isPending} {...form.register('purpose_code')} />
          </FormField>
          <FormField name="urgency" label="紧急程度">
            <Select options={URGENCY_OPTIONS} {...form.register('urgency')} />
          </FormField>
        </div>
        <FormField name="purpose_detail" label="具体说明" required hint="2–500 个字符，说明用车目的、对接事项等">
          <Textarea placeholder="如：赴市政务中心办理年度报告备案" maxLength={500} {...form.register('purpose_detail')} />
        </FormField>
      </section>

      {/* 2. 时段与车辆 */}
      <section className="space-y-4 border-t border-line pt-4">
        <h4 className="text-sm font-semibold text-ink">2. 用车时段与车辆</h4>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField name="planned_start" label="开始时间" required>
            {({ id, invalid }) => <Controller control={form.control} name="planned_start" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
          </FormField>
          <FormField name="planned_end" label="结束时间" required>
            {({ id, invalid }) => <Controller control={form.control} name="planned_end" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
          </FormField>
        </div>
        <FormField
          name="vehicle_id"
          label="车辆"
          hint={
            !periodValid
              ? '请先填写有效的用车时段，再选择车辆'
              : available.isError
                ? `可用车辆查询失败：${errorMessage(available.error)}`
                : available.data && available.data.length === 0
                  ? '该时段没有空闲且无冲突的车辆；可不选车辆，由审批人指派'
                  : '可不选，由审批人在通过时指派；列表仅含该时段空闲且无冲突的车辆'
          }
        >
          <Select options={vehicleOptions} placeholder={!periodValid ? '请先选择时段' : available.isPending ? '正在查询可用车辆…' : '由审批人指派'} disabled={!periodValid || available.isPending} {...form.register('vehicle_id')} />
        </FormField>
      </section>

      {/* 3. 目的地与路线 */}
      <section className="space-y-4 border-t border-line pt-4">
        <h4 className="text-sm font-semibold text-ink">3. 目的地与路线</h4>
        <FormField name="destination" label="目的地" required hint="地图选点后自动填入地址，可修改">
          <Input
            placeholder="如：某某市政务中心"
            maxLength={200}
            {...form.register('destination', { onChange: () => setDestAuto(false) })}
          />
        </FormField>
        <FormField name="dest" label="目的地坐标" hint={origin ? '起点为所选车辆当前位置' : vehicleId ? (canSeeLive ? '所选车辆暂无位置，无法自动估算里程' : '无车辆位置权限，里程请手动填写') : '选择车辆后以车辆当前位置为起点估算里程'}>
          {({ id, invalid }) => (
            <Controller
              control={form.control}
              name="dest"
              render={({ field }) => (
                <MapPicker
                  id={id}
                  invalid={invalid}
                  value={field.value}
                  origin={origin}
                  originLabel="车辆位置"
                  pointLabel="目的地"
                  height={260}
                  onChange={(p) => {
                    field.onChange(p)
                    const current = form.getValues('destination')
                    if (p?.address && (current.trim() === '' || destAuto)) {
                      form.setValue('destination', p.address, { shouldValidate: true })
                      setDestAuto(true)
                    }
                  }}
                />
              )}
            />
          )}
        </FormField>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField
            name="planned_km"
            label="预计里程（km）"
            hint={
              planning ? (
                <span className="inline-flex items-center gap-1">
                  <Spinner size="sm" /> 正在规划路线…
                </span>
              ) : plan.route ? (
                `按驾车路线规划：${plan.route.distance_km?.toFixed(1)} km，约 ${plan.route.duration_min ?? '—'} 分钟${plan.route.summary ? `，${plan.route.summary}` : ''}`
              ) : plan.straightKm !== null ? (
                `按直线距离 ${origin && destPoint ? formatDistance(haversineMeters(origin, destPoint)) : ''} × ${STRAIGHT_FACTOR} 估算${planError ? `（${planError}）` : ''}，可修改`
              ) : (
                '可手动填写；有起点与目的地坐标时自动估算'
              )
            }
          >
            <Input
              type="number"
              inputMode="decimal"
              min={0}
              step="0.1"
              placeholder="如 38.5"
              {...form.register('planned_km', { onChange: () => setKmTouched(true) })}
            />
          </FormField>
        </div>
      </section>

      {/* 4. 其他 */}
      <section className="space-y-4 border-t border-line pt-4">
        <h4 className="text-sm font-semibold text-ink">4. 随行与附件</h4>
        <FormField name="passengers" label="随行人员" hint={canSearchUsers ? '可多选' : '需要「用户管理-查看」权限才能选择随行人员'}>
          {({ id, invalid }) => (
            <Controller
              control={form.control}
              name="passengers"
              render={({ field }) => <UserPicker multiple id={id} invalid={invalid} value={field.value} onChange={field.onChange} placeholder="搜索并添加随行人员" excludeIds={new Set([applicant?.id ?? profile?.id ?? ''])} />}
            />
          )}
        </FormField>
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-ink">附件</span>
            <Button variant="ghost" size="sm" icon={Plus} onClick={() => attachments.append({ name: '', url: '' })} disabled={attachments.fields.length >= 10}>
              添加附件
            </Button>
          </div>
          {attachments.fields.length === 0 ? (
            <p className="text-xs text-ink-faint">本阶段不支持上传，仅登记附件名称与链接地址</p>
          ) : (
            <div className="space-y-2">
              {attachments.fields.map((f, i) => (
                <div key={f.id} className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_2fr_auto]">
                  <FormField name={`attachments.${i}.name`}>
                    <Input placeholder="附件名称" {...form.register(`attachments.${i}.name` as const)} />
                  </FormField>
                  <FormField name={`attachments.${i}.url`}>
                    <Input placeholder="https://…" {...form.register(`attachments.${i}.url` as const)} />
                  </FormField>
                  <Button variant="ghost" size="md" icon={Trash2} className="text-danger-200 hover:bg-danger-500/10" aria-label="删除附件" onClick={() => attachments.remove(i)} />
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* 预检结果 */}
      {precheck && <PrecheckCard r={precheck} />}

      <div className="sticky bottom-0 -mx-5 -mb-4 flex flex-wrap items-center justify-between gap-2 border-t border-line bg-surface-2 px-5 py-3">
        <span className="text-xs text-ink-faint">申请人：{applicantName}</span>
        <div className="flex items-center gap-2">
          <Button variant="secondary" onClick={onCancel} disabled={busy}>
            取消
          </Button>
          <Button variant="secondary" icon={ShieldCheck} onClick={() => void runPrecheck()} loading={precheckMutation.isPending} disabled={createMutation.isPending}>
            预检
          </Button>
          <Button type="submit" loading={createMutation.isPending} disabled={!canSubmit || precheckMutation.isPending} title={!canSubmit ? '请先预检并通过' : undefined}>
            提交申请
          </Button>
        </div>
      </div>
    </Form>
  )
}

/** 发起用车申请抽屉：基本信息 → 时段与车辆 → 目的地与路线 → 随行与附件 → 预检 → 提交 */
export default function CreateApprovalDrawer({ open, onClose, onCreated }: CreateApprovalDrawerProps) {
  return (
    <Drawer open={open} onClose={onClose} title="发起用车申请" description="提交前需先预检：校验时段、车辆冲突并确定审批人" width="lg">
      {open && <CreateForm onCancel={onClose} onCreated={onCreated} />}
    </Drawer>
  )
}
