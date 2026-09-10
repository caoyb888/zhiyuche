import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createTenant, updateTenant } from '../../../api/tenants'
import type { Tenant, TenantUpdate } from '../../../api/types'
import Button from '../../../components/ui/Button'
import DateTimeInput from '../../../components/ui/DateTimeInput'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import { useToast } from '../../../components/ui/toast-context'
import { localToRFC3339, rfc3339ToLocal } from '../../../utils/format'
import { tenantCreateSchema, tenantEditSchema, type TenantCreateValues, type TenantEditValues } from './schemas'

export type TenantDrawerState = { mode: 'create' } | { mode: 'edit'; tenant: Tenant } | null

interface Props {
  state: TenantDrawerState
  onClose: () => void
  onSaved: (tenant: Tenant) => void
}

function CreateForm({ onCancel, onSaved }: { onCancel: () => void; onSaved: (t: Tenant) => void }) {
  const toast = useToast()
  const form = useForm<TenantCreateValues>({
    resolver: zodResolver(tenantCreateSchema),
    defaultValues: { code: '', name: '', license_no: '', contact_name: '', contact_phone: '', expires_at: '', admin_username: '', admin_password: '', admin_name: '' },
  })

  const save = useMutation({
    mutationFn: (v: TenantCreateValues) =>
      createTenant({
        code: v.code,
        name: v.name,
        license_no: v.license_no || undefined,
        contact_name: v.contact_name || undefined,
        contact_phone: v.contact_phone || undefined,
        expires_at: localToRFC3339(v.expires_at),
        admin_username: v.admin_username,
        admin_password: v.admin_password || undefined,
        admin_name: v.admin_name || '管理员',
      }),
    onSuccess: (t) => {
      toast.success('租户已创建', '已自动创建内置角色与租户管理员账号')
      onSaved(t)
    },
    onError: (e) => toast.error('创建失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="code" label="租户代码" required hint="2–32 位小写字母、数字或短横线，全局唯一；登录时用于区分同名账号">
          <Input autoComplete="off" placeholder="如 chenhua" spellCheck={false} {...form.register('code')} />
        </FormField>
        <FormField name="name" label="租户名称" required>
          <Input placeholder="如 山东宸华" {...form.register('name')} />
        </FormField>
        <FormField name="license_no" label="营业执照号">
          <Input placeholder="可选" {...form.register('license_no')} />
        </FormField>
        <FormField name="expires_at" label="到期时间" hint="留空为长期有效">
          {({ id, invalid }) => <Controller control={form.control} name="expires_at" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
        </FormField>
        <FormField name="contact_name" label="联系人">
          <Input placeholder="可选" {...form.register('contact_name')} />
        </FormField>
        <FormField name="contact_phone" label="联系电话">
          <Input inputMode="tel" placeholder="可选" {...form.register('contact_phone')} />
        </FormField>
      </div>

      <div className="rounded-lg border border-line bg-surface-3/60 p-4 space-y-4">
        <div className="text-sm font-medium text-ink">租户管理员账号</div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField name="admin_username" label="用户名" required>
            <Input autoComplete="off" placeholder="如 chenhua_admin" spellCheck={false} {...form.register('admin_username')} />
          </FormField>
          <FormField name="admin_name" label="姓名" hint="留空为「管理员」">
            <Input placeholder="管理员" {...form.register('admin_name')} />
          </FormField>
        </div>
        <FormField name="admin_password" label="初始密码" hint="留空使用系统缺省密码；否则至少 8 位，含字母和数字">
          <Input type="password" autoComplete="new-password" placeholder="留空使用系统缺省密码" {...form.register('admin_password')} />
        </FormField>
      </div>

      <div className="flex justify-end gap-2 pt-2 border-t border-line">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          创建租户
        </Button>
      </div>
    </Form>
  )
}

function EditForm({ tenant, onCancel, onSaved }: { tenant: Tenant; onCancel: () => void; onSaved: (t: Tenant) => void }) {
  const toast = useToast()
  const form = useForm<TenantEditValues>({
    resolver: zodResolver(tenantEditSchema),
    defaultValues: {
      name: tenant.name,
      license_no: tenant.license_no ?? '',
      contact_name: tenant.contact_name ?? '',
      contact_phone: tenant.contact_phone ?? '',
      expires_at: rfc3339ToLocal(tenant.expires_at),
    },
  })

  const save = useMutation({
    mutationFn: (v: TenantEditValues): Promise<Tenant> => {
      const body: TenantUpdate = {}
      if (v.name !== tenant.name) body.name = v.name
      const textFields = ['license_no', 'contact_name', 'contact_phone'] as const
      for (const key of textFields) {
        if (v[key] !== (tenant[key] ?? '')) body[key] = v[key]
      }
      if (v.expires_at !== rfc3339ToLocal(tenant.expires_at)) {
        const iso = localToRFC3339(v.expires_at)
        if (iso) body.expires_at = iso
        else body.clear_expires = true
      }
      if (Object.keys(body).length === 0) return Promise.resolve(tenant)
      return updateTenant(tenant.id, body)
    },
    onSuccess: (t) => {
      toast.success('租户已更新')
      onSaved(t)
    },
    onError: (e) => toast.error('更新失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="name" label="租户名称" required>
          <Input {...form.register('name')} />
        </FormField>
        <FormField name="license_no" label="营业执照号">
          <Input placeholder="可选" {...form.register('license_no')} />
        </FormField>
        <FormField name="contact_name" label="联系人">
          <Input placeholder="可选" {...form.register('contact_name')} />
        </FormField>
        <FormField name="contact_phone" label="联系电话">
          <Input inputMode="tel" placeholder="可选" {...form.register('contact_phone')} />
        </FormField>
        <FormField name="expires_at" label="到期时间" hint="清空则长期有效" className="sm:col-span-2">
          {({ id, invalid }) => <Controller control={form.control} name="expires_at" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
        </FormField>
      </div>
      <div className="flex justify-end gap-2 pt-2 border-t border-line">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          保存修改
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑租户抽屉；状态变更走列表页的停用 / 启用 */
export default function TenantDrawer({ state, onClose, onSaved }: Props) {
  const tenant = state?.mode === 'edit' ? state.tenant : undefined
  return (
    <Drawer
      open={state !== null}
      onClose={onClose}
      title={tenant ? `编辑租户 · ${tenant.code}` : '新建租户'}
      description={tenant ? '代码不可修改；停用 / 启用请在列表操作' : '创建后自动生成内置角色与租户管理员账号'}
      width="md"
    >
      {state && (tenant ? <EditForm key={tenant.id} tenant={tenant} onCancel={onClose} onSaved={onSaved} /> : <CreateForm onCancel={onClose} onSaved={onSaved} />)}
    </Drawer>
  )
}
