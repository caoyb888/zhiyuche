import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useMemo } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'
import { errorMessage } from '../../../api/client'
import { setParam } from '../../../api/params'
import type { Param, ParamValueType } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Modal from '../../../components/ui/Modal'
import Switch from '../../../components/ui/Switch'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { paramTypeLabel } from './schemas'

interface Props {
  param: Param | null
  /** 当前写入的作用域说明（全局缺省 / 某租户覆盖） */
  scopeHint: string
  onClose: () => void
  onSaved: (param: Param) => void
}

interface FormValues {
  value: string
  description: string
}

/** 与后端 param.ValidateValue 一致的前端校验 */
function valueError(type: ParamValueType, v: string): string | undefined {
  switch (type) {
    case 'int':
      return /^-?\d+$/.test(v) ? undefined : '值必须是整数'
    case 'float':
      return v !== '' && Number.isFinite(Number(v)) ? undefined : '值必须是数字'
    case 'bool':
      return v === 'true' || v === 'false' ? undefined : '值必须是 true 或 false'
    case 'json':
      try {
        JSON.parse(v)
        return undefined
      } catch {
        return '值必须是合法 JSON'
      }
    default:
      return undefined
  }
}

function schemaFor(type: ParamValueType) {
  return z.object({
    value: z.string().superRefine((v, ctx) => {
      const msg = valueError(type, v)
      if (msg) ctx.addIssue({ code: 'custom', message: msg })
    }),
    description: z.string().trim().max(255, '说明最多 255 个字符'),
  })
}

function ParamForm({ param, scopeHint, onClose, onSaved }: Props & { param: Param }) {
  const toast = useToast()
  const schema = useMemo(() => schemaFor(param.value_type), [param.value_type])
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { value: param.value, description: param.description ?? '' },
  })

  const save = useMutation({
    mutationFn: (v: FormValues) => {
      // json 规范化为紧凑格式再提交，避免仅因空白差异写入
      const value = param.value_type === 'json' ? JSON.stringify(JSON.parse(v.value)) : v.value.trim()
      const description = v.description !== (param.description ?? '') ? v.description : undefined
      return setParam(param.key, { value, description })
    },
    onSuccess: (saved) => {
      toast.success('参数已保存', saved.source === 'global' ? '已写入全局缺省值' : '已写入本租户覆盖值')
      onSaved(saved)
    },
    onError: (e) => toast.error('保存失败', errorMessage(e)),
  })

  const type = param.value_type
  const control = (() => {
    if (type === 'bool') {
      return (
        <Controller
          control={form.control}
          name="value"
          render={({ field }) => (
            <Switch checked={field.value === 'true'} onChange={(c) => field.onChange(c ? 'true' : 'false')} label={field.value === 'true' ? '开启（true）' : '关闭（false）'} />
          )}
        />
      )
    }
    if (type === 'json') {
      return <Textarea rows={6} spellCheck={false} className="font-mono text-xs" placeholder='{"key": "value"}' {...form.register('value')} />
    }
    if (type === 'int' || type === 'float') {
      return <Input type="number" step={type === 'int' ? 1 : 'any'} inputMode={type === 'int' ? 'numeric' : 'decimal'} className="font-mono" {...form.register('value')} />
    }
    return <Input autoComplete="off" spellCheck={false} {...form.register('value')} />
  })()

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="rounded-lg bg-surface-3 px-3 py-2.5 text-xs text-ink-muted space-y-1">
        <div className="flex items-center gap-2">
          <code className="font-mono text-ink">{param.key}</code>
          <Badge color="gray">{paramTypeLabel[type]}</Badge>
        </div>
        <div>{scopeHint}</div>
        {param.source === 'tenant' && param.global_value !== null && param.global_value !== undefined && (
          <div>
            全局缺省值：<code className="font-mono text-ink">{param.global_value}</code>
          </div>
        )}
      </div>
      <FormField name="value" label="参数值" required>
        {control}
      </FormField>
      <FormField name="description" label="说明">
        <Textarea rows={2} placeholder="可选" {...form.register('description')} />
      </FormField>
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          保存
        </Button>
      </div>
    </Form>
  )
}

/** 修改参数值：按 value_type 渲染不同输入并做前端校验 */
export default function ParamEditModal(props: Props) {
  const { param, onClose } = props
  return (
    <Modal open={param !== null} onClose={onClose} title="修改参数" size="md">
      {param && <ParamForm key={`${param.key}:${param.updated_at}`} {...props} param={param} />}
    </Modal>
  )
}
