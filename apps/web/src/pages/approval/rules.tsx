import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Save } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { approvalKeys, getApprovalRules, setApprovalRules } from '../../api/approvals'
import { errorMessage } from '../../api/client'
import type { ApprovalRules, ApprovalRulesUpdate, TripType } from '../../api/types'
import Button from '../../components/ui/Button'
import CheckboxGroup from '../../components/ui/CheckboxGroup'
import ErrorState from '../../components/ui/ErrorState'
import { Form, FormField } from '../../components/ui/Form'
import Input from '../../components/ui/Input'
import PageHeader from '../../components/ui/PageHeader'
import Select, { type SelectOption } from '../../components/ui/Select'
import Spinner from '../../components/ui/Spinner'
import Switch from '../../components/ui/Switch'
import { useToast } from '../../components/ui/toast-context'
import UserPicker from '../../components/UserPicker'
import { usePermission } from '../../hooks/usePermission'
import { useRoleOptions } from '../../hooks/useRoleOptions'
import { formatDateTime } from '../../utils/format'
import { TRIP_TYPE_LABEL } from './style'

const TIME_RULE = /^([01]\d|2[0-3]):[0-5]\d$/

const rulesSchema = z
  .object({
    enabled: z.boolean(),
    level2_km: z.string().trim().refine((v) => v === '' || (Number.isFinite(Number(v)) && Number(v) > 0), '里程阈值须为正数（km），留空表示不按里程触发'),
    level2_night: z.boolean(),
    night_start: z.string().regex(TIME_RULE, '时间格式为 HH:mm'),
    night_end: z.string().regex(TIME_RULE, '时间格式为 HH:mm'),
    level2_cross_dept: z.boolean(),
    level2_trip_types: z.array(z.enum(['official', 'daily'])),
    level2_approver: z.object({ id: z.string(), name: z.string(), username: z.string().optional(), dept_name: z.string().nullable().optional(), phone: z.string().nullable().optional() }).nullable(),
    fallback_approver_role: z.string().trim().min(1, '请选择回退审批角色').max(64, '角色编码最多 64 个字符'),
    overdue_alert_minutes: z.string().trim().refine((v) => /^\d+$/.test(v) && Number(v) >= 5, '超时提醒须为 ≥ 5 的整数（分钟）'),
  })
  .superRefine((v, ctx) => {
    const hasTrigger = v.level2_km !== '' || v.level2_night || v.level2_cross_dept || v.level2_trip_types.length > 0
    if (hasTrigger && !v.level2_approver) {
      ctx.addIssue({ code: 'custom', path: ['level2_approver'], message: '已配置二级审批触发条件，需指定二级审批人' })
    }
  })

type RulesFormValues = z.infer<typeof rulesSchema>

function toForm(r: ApprovalRules): RulesFormValues {
  return {
    enabled: r.enabled,
    level2_km: r.level2_km === null || r.level2_km === undefined ? '' : String(r.level2_km),
    level2_night: r.level2_night,
    night_start: r.night_start,
    night_end: r.night_end,
    level2_cross_dept: r.level2_cross_dept,
    level2_trip_types: r.level2_trip_types,
    level2_approver: r.level2_approver ?? null,
    fallback_approver_role: r.fallback_approver_role,
    overdue_alert_minutes: String(r.overdue_alert_minutes),
  }
}

function toUpdate(v: RulesFormValues): ApprovalRulesUpdate {
  return {
    enabled: v.enabled,
    level2_km: v.level2_km === '' ? null : Number(v.level2_km),
    level2_night: v.level2_night,
    night_start: v.night_start,
    night_end: v.night_end,
    level2_cross_dept: v.level2_cross_dept,
    level2_trip_types: v.level2_trip_types,
    level2_approver_id: v.level2_approver?.id ?? null,
    fallback_approver_role: v.fallback_approver_role,
    overdue_alert_minutes: Number(v.overdue_alert_minutes),
  }
}

const TRIP_TYPE_CHECKS = (Object.keys(TRIP_TYPE_LABEL) as TripType[]).map((t) => ({ value: t, label: TRIP_TYPE_LABEL[t] }))

function RulesForm({ rules }: { rules: ApprovalRules }) {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const canSearchUsers = can('system:user:view')
  const form = useForm<RulesFormValues>({ resolver: zodResolver(rulesSchema), defaultValues: toForm(rules) })
  const [enabled, level2Night, fallbackRole] = useWatch({ control: form.control, name: ['enabled', 'level2_night', 'fallback_approver_role'] })

  // 角色下拉（value = 角色编码）；无权限时退化为文本输入
  const { roles, query: roleQuery } = useRoleOptions(canSearchUsers)
  const roleOptions = useMemo<SelectOption[]>(() => {
    const list = roles.map((r) => ({ value: r.code, label: `${r.name}（${r.code}）` }))
    if (fallbackRole && !list.some((o) => o.value === fallbackRole)) list.unshift({ value: fallbackRole, label: `${fallbackRole}（当前值）` })
    return list
  }, [roles, fallbackRole])
  const roleAsSelect = canSearchUsers && !roleQuery.isError

  // 接口数据刷新（如 WebSocket 失效后重取）时同步到未修改的表单
  useEffect(() => {
    if (!form.formState.isDirty) form.reset(toForm(rules))
  }, [rules, form])

  const save = useMutation({
    mutationFn: (v: RulesFormValues) => setApprovalRules(toUpdate(v)),
    onSuccess: (saved) => {
      toast.success('审批规则已保存')
      queryClient.setQueryData(approvalKeys.rules, saved)
      form.reset(toForm(saved))
    },
    onError: (e) => toast.error('保存失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="card space-y-5 p-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h3 className="text-sm font-semibold text-slate-800">启用审批流</h3>
            <p className="mt-0.5 text-xs text-slate-400">关闭后新申请仍会创建，但不再按下列条件判定二级审批</p>
          </div>
          <Controller control={form.control} name="enabled" render={({ field }) => <Switch checked={field.value} onChange={field.onChange} label={field.value ? '已启用' : '已关闭'} />} />
        </div>
      </div>

      <div className="card space-y-5 p-5">
        <div>
          <h3 className="text-sm font-semibold text-slate-800">二级审批触发条件</h3>
          <p className="mt-0.5 text-xs text-slate-400">满足任一条件的申请在一级（部门负责人）通过后转交二级审批人；均不满足则一级通过即批准</p>
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField name="level2_km" label="预计里程阈值（km）" hint="预计里程 ≥ 阈值时需二级审批；留空则不按里程触发">
            <Input type="number" inputMode="decimal" min={0} step="1" placeholder="如 50，留空不启用" disabled={!enabled} {...form.register('level2_km')} />
          </FormField>
          <FormField name="level2_trip_types" label="指定用车类型" hint="所选类型一律需二级审批">
            {({ invalid }) => <Controller control={form.control} name="level2_trip_types" render={({ field }) => <CheckboxGroup options={TRIP_TYPE_CHECKS} value={field.value} onChange={(v) => field.onChange(v.filter((x): x is TripType => x === 'official' || x === 'daily'))} disabled={!enabled} invalid={invalid} columns={2} />} />}
          </FormField>
        </div>
        <div className="space-y-3 rounded-xl border border-slate-100 p-4">
          <Controller control={form.control} name="level2_night" render={({ field }) => <Switch checked={field.value} onChange={field.onChange} disabled={!enabled} label="夜间用车需二级审批" size="sm" />} />
          <div className="grid grid-cols-2 gap-4 sm:max-w-md">
            <FormField name="night_start" label="夜间开始">
              <Input type="time" disabled={!enabled || !level2Night} {...form.register('night_start')} />
            </FormField>
            <FormField name="night_end" label="夜间结束" hint="跨零点时结束时间小于开始时间，如 22:00–06:00">
              <Input type="time" disabled={!enabled || !level2Night} {...form.register('night_end')} />
            </FormField>
          </div>
        </div>
        <div className="rounded-xl border border-slate-100 p-4">
          <Controller control={form.control} name="level2_cross_dept" render={({ field }) => <Switch checked={field.value} onChange={field.onChange} disabled={!enabled} label="跨部门用车（车辆归属部门与申请人部门不同）需二级审批" size="sm" />} />
        </div>
      </div>

      <div className="card space-y-5 p-5">
        <div>
          <h3 className="text-sm font-semibold text-slate-800">审批人</h3>
          <p className="mt-0.5 text-xs text-slate-400">一级审批人为申请人所在部门负责人（逐级向上，跳过本人）；找不到时回退到持有指定角色的用户</p>
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField name="level2_approver" label="二级审批人" hint={canSearchUsers ? '可清空；配置了触发条件时必填' : '需要「用户管理-查看」权限才能搜索用户'}>
            {({ id, invalid }) => <Controller control={form.control} name="level2_approver" render={({ field }) => <UserPicker id={id} invalid={invalid} value={field.value} onChange={field.onChange} placeholder="搜索并指定二级审批人" />} />}
          </FormField>
          <FormField name="fallback_approver_role" label="回退审批角色" required hint={roleAsSelect ? '按角色编码匹配，取该角色下最早创建的在职用户' : '填写角色编码（如 approver）'}>
            {roleAsSelect ? <Select options={roleOptions} placeholder={roleQuery.isPending ? '加载角色…' : '请选择角色'} disabled={roleQuery.isPending} {...form.register('fallback_approver_role')} /> : <Input placeholder="角色编码，如 approver" maxLength={64} {...form.register('fallback_approver_role')} />}
          </FormField>
          <FormField name="overdue_alert_minutes" label="超时提醒（分钟）" required hint="审批步骤超过该时长未处理时提醒审批人，最小 5 分钟">
            <Input type="number" inputMode="numeric" min={5} step="1" {...form.register('overdue_alert_minutes')} />
          </FormField>
        </div>
      </div>

      <div className="flex items-center justify-between gap-3">
        <span className="text-xs text-slate-400">{rules.updated_at ? `最近更新 ${formatDateTime(rules.updated_at)}` : '尚未保存过，当前为系统缺省值'}</span>
        <div className="flex items-center gap-2">
          <Button variant="secondary" onClick={() => form.reset(toForm(rules))} disabled={!form.formState.isDirty || save.isPending}>
            还原
          </Button>
          <Button type="submit" icon={Save} loading={save.isPending}>
            保存规则
          </Button>
        </div>
      </div>
    </Form>
  )
}

/** 审批规则（/approval/rules，approval:rule）：二级审批触发条件、审批人与超时提醒 */
export default function ApprovalRulesPage() {
  const navigate = useNavigate()
  const rules = useQuery({ queryKey: approvalKeys.rules, queryFn: getApprovalRules })

  return (
    <div className="mx-auto max-w-4xl space-y-4 slide-up">
      <PageHeader
        title="审批规则"
        description="本租户的用车审批规则：何时需要二级审批、审批人与超时提醒"
        extra={
          <Button variant="secondary" icon={ArrowLeft} onClick={() => navigate('/approval')}>
            返回审批列表
          </Button>
        }
      />
      {rules.isPending ? (
        <div className="card flex justify-center p-10">
          <Spinner label="加载规则…" />
        </div>
      ) : rules.isError ? (
        <div className="card p-4">
          <ErrorState message={errorMessage(rules.error)} onRetry={() => void rules.refetch()} />
        </div>
      ) : (
        <RulesForm rules={rules.data} />
      )}
    </div>
  )
}
