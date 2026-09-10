import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createTemplate, updateTemplate } from '../../../api/templates'
import type { Template, TemplateUpdate } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select from '../../../components/ui/Select'
import Switch from '../../../components/ui/Switch'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { channelOptions, extractVariables, templateFormSchema, type TemplateFormValues } from './schemas'

export type TemplateDrawerState = { mode: 'create' } | { mode: 'edit'; template: Template } | null

interface Props {
  state: TemplateDrawerState
  createsGlobal: boolean
  onClose: () => void
  onSaved: (template: Template) => void
}

function TemplateForm({ template, onCancel, onSaved }: { template?: Template; onCancel: () => void; onSaved: (t: Template) => void }) {
  const toast = useToast()
  const isEdit = template !== undefined
  const form = useForm<TemplateFormValues>({
    resolver: zodResolver(templateFormSchema),
    defaultValues: template
      ? { code: template.code, channel: template.channel, title: template.title, content: template.content, enabled: template.enabled }
      : { code: '', channel: 'inapp', title: '', content: '', enabled: true },
  })

  const save = useMutation({
    mutationFn: (v: TemplateFormValues): Promise<Template> => {
      if (!template) return createTemplate({ code: v.code, channel: v.channel, title: v.title, content: v.content, enabled: v.enabled })
      const body: TemplateUpdate = {}
      if (v.title !== template.title) body.title = v.title
      if (v.content !== template.content) body.content = v.content
      if (v.enabled !== template.enabled) body.enabled = v.enabled
      if (Object.keys(body).length === 0) return Promise.resolve(template)
      return updateTemplate(template.id, body)
    },
    onSuccess: (saved) => {
      toast.success(isEdit ? '模板已更新' : '模板已创建')
      onSaved(saved)
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '创建失败', errorMessage(e)),
  })

  const variables = extractVariables(useWatch({ control: form.control, name: 'content' }))

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_10rem]">
        <FormField name="code" label="模板代码" required hint={isEdit ? '代码不可修改' : '事件代码，如 approval.submitted；同一作用域内 (代码, 渠道) 唯一'}>
          <Input autoComplete="off" disabled={isEdit} placeholder="approval.submitted" spellCheck={false} {...form.register('code')} />
        </FormField>
        <FormField name="channel" label="渠道" required hint={isEdit ? '渠道不可修改' : undefined}>
          <Select options={channelOptions} disabled={isEdit} {...form.register('channel')} />
        </FormField>
      </div>
      <FormField name="title" label="标题" required>
        <Input placeholder="如 您的用车申请已提交" {...form.register('title')} />
      </FormField>
      <FormField
        name="content"
        label="内容"
        required
        hint={
          <span>
            用 <code className="font-mono text-ink">{'{{变量名}}'}</code> 引用变量，发送时替换为实际值，如 <code className="font-mono text-ink">{'{{applicant}}'}</code>。
          </span>
        }
      >
        <Textarea rows={6} placeholder={'{{applicant}} 提交了用车申请，目的地 {{destination}}，请及时审批。'} {...form.register('content')} />
      </FormField>
      {variables.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 text-xs text-ink-muted">
          <span>检测到变量：</span>
          {variables.map((v) => (
            <Badge key={v} color="gray">
              <code className="font-mono">{v}</code>
            </Badge>
          ))}
        </div>
      )}
      <FormField name="enabled" label="启用">
        <Controller control={form.control} name="enabled" render={({ field }) => <Switch checked={field.value} onChange={field.onChange} label={field.value ? '启用' : '停用'} />} />
      </FormField>

      <div className="flex justify-end gap-2 pt-2 border-t border-line">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '创建模板'}
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑通知模板抽屉；编辑时代码与渠道不可改 */
export default function TemplateDrawer({ state, createsGlobal, onClose, onSaved }: Props) {
  const template = state?.mode === 'edit' ? state.template : undefined
  return (
    <Drawer
      open={state !== null}
      onClose={onClose}
      title={template ? `编辑模板 · ${template.code}` : '新建通知模板'}
      description={
        template
          ? template.is_global
            ? '全局模板，对所有未覆盖的租户生效'
            : '租户模板，仅本租户生效'
          : createsGlobal
            ? '当前为平台视角：创建的是全局模板'
            : '与同代码同渠道的全局模板并存时，本租户以租户模板为准'
      }
      width="md"
    >
      {state && <TemplateForm key={template?.id ?? 'create'} template={template} onCancel={onClose} onSaved={onSaved} />}
    </Drawer>
  )
}
