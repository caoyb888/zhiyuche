import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { useFieldArray, useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createDictType, updateDictType } from '../../../api/dicts'
import type { DictType, DictTypeUpdate } from '../../../api/types'
import Button from '../../../components/ui/Button'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { dictTypeFormSchema, type DictTypeFormValues } from './schemas'

export type DictTypeDrawerState = { mode: 'create' } | { mode: 'edit'; dictType: DictType } | null

interface Props {
  state: DictTypeDrawerState
  /** 超级管理员未切换查看租户：创建的是全局字典 */
  createsGlobal: boolean
  onClose: () => void
  onSaved: (dictType: DictType) => void
}

function DictTypeForm({ dictType, onCancel, onSaved }: { dictType?: DictType; onCancel: () => void; onSaved: (t: DictType) => void }) {
  const toast = useToast()
  const isEdit = dictType !== undefined

  const form = useForm<DictTypeFormValues>({
    resolver: zodResolver(dictTypeFormSchema),
    defaultValues: dictType
      ? { code: dictType.code, name: dictType.name, description: dictType.description ?? '', items: [] }
      : { code: '', name: '', description: '', items: [] },
  })
  const items = useFieldArray({ control: form.control, name: 'items' })

  const save = useMutation({
    mutationFn: (v: DictTypeFormValues): Promise<DictType> => {
      if (!dictType) {
        return createDictType({
          code: v.code,
          name: v.name,
          description: v.description || undefined,
          items: v.items.map((it, i) => ({ label: it.label, value: it.value, sort: it.sort === '' ? i : Number(it.sort), status: 'active' })),
        })
      }
      const body: DictTypeUpdate = {}
      if (v.name !== dictType.name) body.name = v.name
      if (v.description !== (dictType.description ?? '')) body.description = v.description
      if (Object.keys(body).length === 0) return Promise.resolve(dictType)
      return updateDictType(dictType.id, body)
    },
    onSuccess: (saved) => {
      toast.success(isEdit ? '字典已更新' : '字典已创建')
      onSaved(saved)
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '创建失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="code" label="字典代码" required hint={isEdit ? '代码不可修改' : '2–64 位小写字母、数字或下划线；业务代码按此取值'}>
          <Input autoComplete="off" disabled={isEdit} placeholder="如 vehicle_type" spellCheck={false} {...form.register('code')} />
        </FormField>
        <FormField name="name" label="字典名称" required>
          <Input placeholder="如 车辆类型" {...form.register('name')} />
        </FormField>
      </div>
      <FormField name="description" label="描述">
        <Textarea rows={2} placeholder="可选" {...form.register('description')} />
      </FormField>

      {!isEdit && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-ink">
              初始条目 <span className="font-normal text-ink-faint">（可选，创建后可继续维护）</span>
            </span>
            <Button size="sm" variant="secondary" icon={Plus} onClick={() => items.append({ label: '', value: '', sort: String(items.fields.length) })}>
              添加条目
            </Button>
          </div>
          {items.fields.length === 0 ? (
            <div className="rounded-lg border border-dashed border-line-strong py-4 text-center text-xs text-ink-faint">尚未添加条目</div>
          ) : (
            <div className="space-y-2">
              <div className="grid grid-cols-[1fr_1fr_5rem_2rem] gap-2 px-1 text-xs text-ink-faint">
                <span>显示名</span>
                <span>值</span>
                <span>排序</span>
                <span />
              </div>
              {items.fields.map((f, i) => (
                <div key={f.id} className="grid grid-cols-[1fr_1fr_5rem_2rem] gap-2 items-start">
                  <FormField name={`items.${i}.label`}>
                    <Input placeholder="轿车" {...form.register(`items.${i}.label`)} />
                  </FormField>
                  <FormField name={`items.${i}.value`}>
                    <Input placeholder="sedan" spellCheck={false} {...form.register(`items.${i}.value`)} />
                  </FormField>
                  <FormField name={`items.${i}.sort`}>
                    <Input inputMode="numeric" {...form.register(`items.${i}.sort`)} />
                  </FormField>
                  <Button variant="ghost" size="sm" icon={Trash2} className="!px-2 h-9 text-danger-200 hover:bg-danger-500/10" aria-label="移除" onClick={() => items.remove(i)} />
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <div className="flex justify-end gap-2 pt-2 border-t border-line">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '创建字典'}
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑字典类型抽屉；编辑只开放名称与描述 */
export default function DictTypeDrawer({ state, createsGlobal, onClose, onSaved }: Props) {
  const dictType = state?.mode === 'edit' ? state.dictType : undefined
  return (
    <Drawer
      open={state !== null}
      onClose={onClose}
      title={dictType ? `编辑字典 · ${dictType.code}` : '新建字典'}
      description={
        dictType
          ? dictType.is_system
            ? '系统字典只能修改名称与描述'
            : '代码不可修改'
          : createsGlobal
            ? '当前为平台视角：创建的是全局字典，对所有租户可见'
            : '租户字典与同代码的全局字典并存时，本租户以租户字典为准'
      }
      width="md"
    >
      {state && <DictTypeForm key={dictType?.id ?? 'create'} dictType={dictType} onCancel={onClose} onSaved={onSaved} />}
    </Drawer>
  )
}
