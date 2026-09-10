import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import clsx from 'clsx'
import { Copy, Plus, Save, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Controller, useFieldArray, useForm, useWatch, type UseFormReturn } from 'react-hook-form'
import { createBillingRule, updateBillingRule } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { BillingPenaltyType, BillingRule, BillingRuleDoc, BillingRuleUpdate } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import JsonView from '../../../components/ui/JsonView'
import Select from '../../../components/ui/Select'
import Switch from '../../../components/ui/Switch'
import Tabs from '../../../components/ui/Tabs'
import { useToast } from '../../../components/ui/toast-context'
import { copyText } from '../../../utils/download'
import { formatDateTime, stringifyJson } from '../../../utils/format'
import { PENALTY_TYPE_LABEL, PENALTY_TYPE_OPTIONS } from '../style'
import { ACCOUNT_LEVEL_SELECT, PENALTY_FIELDS, PENALTY_FIELD_LABEL, docToForm, emptyMultiplier, emptyPenalty, formToDoc, ruleFormSchema, type PenaltyFormValues, type RuleFormValues } from './schemas'
import SimulatePanel from './SimulatePanel'

export type EditorState = { mode: 'create'; doc: BillingRuleDoc; name: string; source?: string } | { mode: 'edit'; rule: BillingRule }

type EditorTab = 'form' | 'json' | 'simulate'

interface RuleEditorProps {
  state: EditorState
  readOnly: boolean
  onSaved: (rule: BillingRule) => void
  onCancelCreate: () => void
}

function Section({ title, description, extra, children }: { title: string; description?: string; extra?: ReactNode; children: ReactNode }) {
  return (
    <section className="space-y-4 rounded-xl border border-line p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-ink">{title}</h4>
          {description && <p className="mt-0.5 text-xs text-ink-faint">{description}</p>}
        </div>
        {extra}
      </div>
      {children}
    </section>
  )
}

function MultiplierRows({ form, readOnly }: { form: UseFormReturn<RuleFormValues>; readOnly: boolean }) {
  const { fields, append, remove } = useFieldArray({ control: form.control, name: 'time_multipliers' })
  return (
    <Section
      title="时段系数"
      description="按行程开始时间（Asia/Shanghai）匹配，命中第一条；可跨午夜（如 23:00–06:00）。系数 1.3 表示加价 30%，0.7 表示打七折"
      extra={
        !readOnly && (
          <Button type="button" variant="secondary" size="sm" icon={Plus} onClick={() => append(emptyMultiplier())}>
            添加时段
          </Button>
        )
      }
    >
      {fields.length === 0 ? (
        <div className="text-xs text-ink-faint">未配置时段系数，全天按基础费率计费</div>
      ) : (
        <div className="space-y-3">
          {fields.map((f, i) => (
            <div key={f.id} className="grid grid-cols-2 gap-3 rounded-lg bg-surface-3 p-3 sm:grid-cols-[1fr_1fr_1fr_6rem_auto] sm:items-start">
              <FormField name={`time_multipliers.${i}.name`} label={i === 0 ? '名称' : undefined}>
                <Input placeholder="如 早高峰" maxLength={32} {...form.register(`time_multipliers.${i}.name`)} />
              </FormField>
              <FormField name={`time_multipliers.${i}.start`} label={i === 0 ? '开始' : undefined}>
                <Input type="time" {...form.register(`time_multipliers.${i}.start`)} />
              </FormField>
              <FormField name={`time_multipliers.${i}.end`} label={i === 0 ? '结束' : undefined}>
                <Input type="time" {...form.register(`time_multipliers.${i}.end`)} />
              </FormField>
              <FormField name={`time_multipliers.${i}.factor`} label={i === 0 ? '系数' : undefined}>
                <Input type="number" inputMode="decimal" min={0} step="0.1" {...form.register(`time_multipliers.${i}.factor`)} />
              </FormField>
              <div className={clsx('flex justify-end', i === 0 && 'sm:pt-7')}>
                {!readOnly && (
                  <Button type="button" variant="ghost" size="sm" icon={Trash2} className="!px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200" title="删除" aria-label="删除时段" onClick={() => remove(i)} />
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </Section>
  )
}

function PenaltyRows({ form, readOnly }: { form: UseFormReturn<RuleFormValues>; readOnly: boolean }) {
  const { fields, append, remove } = useFieldArray({ control: form.control, name: 'penalty_rules' })
  const rows = useWatch({ control: form.control, name: 'penalty_rules' }) ?? []
  const usedTypes = new Set(rows.map((r) => r?.type))
  const nextType = (Object.keys(PENALTY_TYPE_LABEL) as BillingPenaltyType[]).find((t) => !usedTypes.has(t))

  return (
    <Section
      title="罚金规则"
      description="每种类型最多一条；超速 / 急加减速按事件次数计，低电未充电按还车 SOC 判定，超时还车按超出计划的整小时计。上限为单次行程该项罚金封顶"
      extra={
        !readOnly && (
          <Button type="button" variant="secondary" size="sm" icon={Plus} disabled={!nextType} title={nextType ? undefined : '四种罚金类型均已配置'} onClick={() => nextType && append(emptyPenalty(nextType))}>
            添加罚金
          </Button>
        )
      }
    >
      {fields.length === 0 ? (
        <div className="text-xs text-ink-faint">未配置罚金规则</div>
      ) : (
        <div className="space-y-3">
          {fields.map((f, i) => {
            const type: BillingPenaltyType = rows[i]?.type ?? f.type
            const keys = PENALTY_FIELDS[type]
            return (
              <div key={f.id} className="space-y-3 rounded-lg bg-surface-3 p-3">
                <div className="flex items-end gap-3">
                  <FormField name={`penalty_rules.${i}.type`} label="类型" className="w-full sm:w-56">
                    <Select options={PENALTY_TYPE_OPTIONS} {...form.register(`penalty_rules.${i}.type`)} />
                  </FormField>
                  <div className="flex-1" />
                  {!readOnly && (
                    <Button type="button" variant="ghost" size="sm" icon={Trash2} className="!px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200" title="删除" aria-label="删除罚金规则" onClick={() => remove(i)} />
                  )}
                </div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                  {keys.map((k: keyof Omit<PenaltyFormValues, 'type'>) => (
                    <FormField key={k} name={`penalty_rules.${i}.${k}`} label={PENALTY_FIELD_LABEL[k]}>
                      <Input type="number" inputMode="decimal" min={0} step={k === 'min_soc_required' || k === 'threshold_kmh' ? '1' : '0.01'} {...form.register(`penalty_rules.${i}.${k}`)} />
                    </FormField>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </Section>
  )
}

function RuleForm({ state, readOnly, onSaved, onCancelCreate }: RuleEditorProps) {
  const toast = useToast()
  const isEdit = state.mode === 'edit'
  const rule = isEdit ? state.rule : undefined
  const defaults = useMemo<RuleFormValues>(
    () => (state.mode === 'edit' ? docToForm(state.rule.rule, state.rule) : { ...docToForm(state.doc), name: state.name }),
    [state],
  )
  const form = useForm<RuleFormValues>({ resolver: zodResolver(ruleFormSchema), defaultValues: defaults })
  const [tab, setTab] = useState<EditorTab>('form')

  // 当前编辑中的规则文档：JSON 视图与模拟计算都用它（未保存的修改也生效）
  const values = useWatch({ control: form.control })
  // useWatch 返回 DeepPartial；defaultValues 覆盖了全部字段，合并后即完整表单值
  const draft = useMemo(() => ({ ...defaults, ...values }) as RuleFormValues, [defaults, values])
  const draftDoc = useMemo(() => formToDoc(draft), [draft])
  const doc = useMemo(() => {
    const parsed = ruleFormSchema.safeParse(draft)
    return parsed.success ? formToDoc(parsed.data) : null
  }, [draft])

  // 接口数据刷新（保存后 / WebSocket 失效重取）时同步到未修改的表单
  useEffect(() => {
    if (!form.formState.isDirty) form.reset(defaults)
  }, [defaults, form])

  const save = useMutation({
    mutationFn: async (v: RuleFormValues): Promise<BillingRule> => {
      const nextDoc = formToDoc(v)
      if (!rule) {
        return createBillingRule({
          name: v.name.trim(),
          rule: nextDoc,
          effective_from: v.effective_from || undefined,
          effective_to: v.effective_to || undefined,
          activate: false,
        })
      }
      const body: BillingRuleUpdate = {
        name: v.name.trim(),
        rule: nextDoc,
        enabled: v.enabled,
        effective_from: v.effective_from || null,
        effective_to: v.effective_to || null,
      }
      return updateBillingRule(rule.id, body)
    },
    onSuccess: (saved) => {
      toast.success(isEdit ? '规则已保存' : '规则已创建', isEdit ? undefined : '新规则尚未生效，可在左侧列表"设为生效"')
      form.reset(docToForm(saved.rule, saved))
      onSaved(saved)
    },
    // 后端 400 会带 engine.Validate 的具体信息（如 time_multipliers[0].range）
    onError: (e) => toast.error(isEdit ? '保存失败' : '创建失败', errorMessage(e)),
  })

  const copyJson = async () => {
    const ok = await copyText(stringifyJson(draftDoc))
    if (ok) toast.success('已复制规则 JSON')
    else toast.error('复制失败', '请手动选择文本复制')
  }

  const enabled = useWatch({ control: form.control, name: 'enabled' })

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-base font-semibold text-ink-strong truncate">{rule ? rule.name : '新建规则'}</h3>
            {rule?.is_default && <Badge color="green">生效中</Badge>}
            {rule && !rule.enabled && <Badge color="gray">已停用</Badge>}
            {state.mode === 'create' && state.source && <Badge color="blue">{state.source}</Badge>}
          </div>
          <div className="mt-0.5 text-xs text-ink-faint">
            {rule ? `更新于 ${formatDateTime(rule.updated_at)} · 创建于 ${formatDateTime(rule.created_at)}` : '保存后出现在左侧列表；首条规则将自动设为生效'}
          </div>
        </div>
        <Tabs<EditorTab>
          size="sm"
          className="!border-b-0"
          items={[
            { key: 'form', label: '表单编辑' },
            { key: 'json', label: 'JSON 视图' },
            { key: 'simulate', label: '模拟计算' },
          ]}
          value={tab}
          onChange={setTab}
        />
      </div>

      {/* 表单始终挂载，切换页签不丢失未保存的修改 */}
      <div className={tab === 'form' ? undefined : 'hidden'}>
        <Form form={form} onSubmit={(v) => save.mutate(v)}>
          <fieldset disabled={readOnly} className="min-w-0 space-y-4">
            <Section title="基本信息" description="规则名称用于列表展示；生效期为空表示不限；停用的规则不会被设为生效">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField name="name" label="规则名称" required>
                  <Input placeholder="如 纯电车队标准套餐" maxLength={64} {...form.register('name')} />
                </FormField>
                <FormField name="rule_name" label="rule_name（账单上显示）" required hint="写入行程账单明细的规则名">
                  <Input maxLength={64} {...form.register('rule_name')} />
                </FormField>
                <FormField name="effective_from" label="生效开始">
                  <Input type="date" {...form.register('effective_from')} />
                </FormField>
                <FormField name="effective_to" label="生效结束">
                  <Input type="date" {...form.register('effective_to')} />
                </FormField>
              </div>
              {isEdit && (
                <Controller control={form.control} name="enabled" render={({ field }) => <Switch checked={field.value} onChange={field.onChange} disabled={readOnly} label={enabled ? '已启用' : '已停用'} size="sm" />} />
              )}
            </Section>

            <Section title="基础费率" description="里程费 = 里程 × 每公里；时长费 = 小时 × 每小时；两者之和乘以时段系数后按每 24 小时封顶（0 不封顶）">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <FormField name="per_km" label="每公里（元/km）" required>
                  <Input type="number" inputMode="decimal" min={0} step="0.01" {...form.register('per_km')} />
                </FormField>
                <FormField name="per_hour" label="每小时（元/小时）" required>
                  <Input type="number" inputMode="decimal" min={0} step="0.01" {...form.register('per_hour')} />
                </FormField>
                <FormField name="daily_cap" label="日封顶（元/24 小时）" hint="0 表示不封顶">
                  <Input type="number" inputMode="decimal" min={0} step="0.01" {...form.register('daily_cap')} />
                </FormField>
              </div>
            </Section>

            <Section title="纯电专项" description="电价同时用于充电事务计价；低电附加按封顶后的基础金额的百分比计">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField name="electricity_cost_per_kwh" label="电价（元/kWh）">
                  <Input type="number" inputMode="decimal" min={0} step="0.01" {...form.register('electricity_cost_per_kwh')} />
                </FormField>
                <FormField name="charging_attribution" label="充电费归属账户" hint="充电事务结束后扣哪一级账户">
                  <Select options={ACCOUNT_LEVEL_SELECT} {...form.register('charging_attribution')} />
                </FormField>
                <FormField name="threshold_soc" label="低电附加阈值（SOC %）" hint="还车 SOC 低于阈值时收取附加费">
                  <Input type="number" inputMode="numeric" min={0} max={100} step="1" {...form.register('threshold_soc')} />
                </FormField>
                <FormField name="surcharge_pct" label="低电附加（%）" hint="0 表示不收取">
                  <Input type="number" inputMode="decimal" min={0} step="1" {...form.register('surcharge_pct')} />
                </FormField>
              </div>
              <Controller
                control={form.control}
                name="include_electricity_in_trip"
                render={({ field }) => <Switch checked={field.value} onChange={field.onChange} disabled={readOnly} label="行程账单包含电费行（能耗 × 电价）" size="sm" />}
              />
            </Section>

            <MultiplierRows form={form} readOnly={readOnly} />
            <PenaltyRows form={form} readOnly={readOnly} />

            <Section title="费用归属" description="行程结束后按用车类型扣哪一级账户；部门 / 员工账户不存在时自动创建">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField name="official" label="公务用车">
                  <Select options={ACCOUNT_LEVEL_SELECT} {...form.register('official')} />
                </FormField>
                <FormField name="daily" label="日常用车">
                  <Select options={ACCOUNT_LEVEL_SELECT} {...form.register('daily')} />
                </FormField>
              </div>
            </Section>
          </fieldset>

          {!readOnly && (
            <div className="flex items-center justify-between gap-3 border-t border-line pt-3">
              <span className="text-xs text-ink-faint">{form.formState.isDirty ? '有未保存的修改' : ''}</span>
              <div className="flex items-center gap-2">
                {isEdit ? (
                  <Button variant="secondary" onClick={() => form.reset(defaults)} disabled={!form.formState.isDirty || save.isPending}>
                    还原
                  </Button>
                ) : (
                  <Button variant="secondary" onClick={onCancelCreate} disabled={save.isPending}>
                    取消
                  </Button>
                )}
                <Button type="submit" icon={Save} loading={save.isPending}>
                  {isEdit ? '保存规则' : '创建规则'}
                </Button>
              </div>
            </div>
          )}
        </Form>
      </div>

      {tab === 'json' && (
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <p className="text-xs text-ink-faint">{doc ? '当前编辑中的规则文档（与提交 PUT/POST 的 rule 字段一致）' : '表单存在校验错误，以下为按当前输入生成的草稿'}</p>
            <Button variant="secondary" size="sm" icon={Copy} onClick={() => void copyJson()}>
              复制 JSON
            </Button>
          </div>
          <JsonView value={draftDoc} maxHeight="60vh" />
        </div>
      )}

      {tab === 'simulate' && <SimulatePanel doc={doc} ruleId={rule?.id} dirty={form.formState.isDirty} />}
    </div>
  )
}

/** 规则编辑器：按规则 id / 新建来源作为 key 重建表单 */
export default function RuleEditor(props: RuleEditorProps) {
  const key = props.state.mode === 'edit' ? props.state.rule.id : `create:${props.state.source ?? ''}:${props.state.name}`
  return <RuleForm key={key} {...props} />
}
