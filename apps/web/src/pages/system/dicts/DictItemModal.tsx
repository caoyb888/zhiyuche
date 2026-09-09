import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createDictItem, updateDictItem } from '../../../api/dicts'
import type { DictItem, DictItemUpdate } from '../../../api/types'
import Button from '../../../components/ui/Button'
import ColorInput from '../../../components/ui/ColorInput'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Modal from '../../../components/ui/Modal'
import Select from '../../../components/ui/Select'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { stringifyJson } from '../../../utils/format'
import { dictItemFormSchema, dictItemStatusOptions, parseExtra, type DictItemFormValues } from './schemas'

export type DictItemModalState = { mode: 'create'; typeId: string; nextSort: number } | { mode: 'edit'; item: DictItem } | null

interface Props {
  state: DictItemModalState
  onClose: () => void
  onSaved: (item: DictItem) => void
}

function defaultsFor(state: NonNullable<DictItemModalState>): DictItemFormValues {
  if (state.mode === 'create') return { label: '', value: '', sort: String(state.nextSort), color: '', status: 'active', extra: '' }
  const it = state.item
  const hasExtra = Object.keys(it.extra ?? {}).length > 0
  return { label: it.label, value: it.value, sort: String(it.sort), color: it.color ?? '', status: it.status, extra: hasExtra ? stringifyJson(it.extra) : '' }
}

function DictItemForm({ state, onClose, onSaved }: { state: NonNullable<DictItemModalState>; onClose: () => void; onSaved: (item: DictItem) => void }) {
  const toast = useToast()
  const isEdit = state.mode === 'edit'
  const form = useForm<DictItemFormValues>({ resolver: zodResolver(dictItemFormSchema), defaultValues: defaultsFor(state) })

  const save = useMutation({
    mutationFn: (v: DictItemFormValues): Promise<DictItem> => {
      const extra = parseExtra(v.extra) ?? {}
      if (state.mode === 'create') {
        return createDictItem(state.typeId, { label: v.label, value: v.value, sort: Number(v.sort), color: v.color || undefined, status: v.status, extra })
      }
      const it = state.item
      const body: DictItemUpdate = {}
      if (v.label !== it.label) body.label = v.label
      if (v.value !== it.value) body.value = v.value
      if (Number(v.sort) !== it.sort) body.sort = Number(v.sort)
      if (v.color !== (it.color ?? '')) body.color = v.color
      if (v.status !== it.status) body.status = v.status
      if (stringifyJson(extra) !== stringifyJson(it.extra ?? {})) body.extra = extra
      if (Object.keys(body).length === 0) return Promise.resolve(it)
      return updateDictItem(it.id, body)
    },
    onSuccess: (saved) => {
      toast.success(isEdit ? '条目已更新' : '条目已添加')
      onSaved(saved)
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '添加失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="label" label="显示名" required>
          <Input placeholder="如 轿车" {...form.register('label')} />
        </FormField>
        <FormField name="value" label="值" required hint="同一字典内唯一，业务代码按此判断">
          <Input placeholder="如 sedan" spellCheck={false} {...form.register('value')} />
        </FormField>
        <FormField name="sort" label="排序" hint="数字越小越靠前">
          <Input inputMode="numeric" {...form.register('sort')} />
        </FormField>
        <FormField name="status" label="状态" required>
          <Select options={dictItemStatusOptions} {...form.register('status')} />
        </FormField>
      </div>
      <FormField name="color" label="颜色" hint="可选，用于前端着色（如状态标签）">
        {({ id, invalid }) => (
          <Controller control={form.control} name="color" render={({ field }) => <ColorInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />
        )}
      </FormField>
      <FormField name="extra" label="附加数据" hint="可选，JSON 对象">
        <Textarea rows={3} placeholder='{"seats": 5}' spellCheck={false} className="font-mono text-xs" {...form.register('extra')} />
      </FormField>
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '添加条目'}
        </Button>
      </div>
    </Form>
  )
}

/** 新增 / 编辑字典条目 */
export default function DictItemModal({ state, onClose, onSaved }: Props) {
  return (
    <Modal open={state !== null} onClose={onClose} title={state?.mode === 'edit' ? `编辑条目 · ${state.item.value}` : '新增条目'} size="md">
      {state && <DictItemForm key={state.mode === 'edit' ? state.item.id : 'create'} state={state} onClose={onClose} onSaved={onSaved} />}
    </Modal>
  )
}
