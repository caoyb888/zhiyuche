import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createRole, updateRole } from '../../../api/roles'
import type { Role, RoleUpdate } from '../../../api/types'
import Button from '../../../components/ui/Button'
import CheckTree from '../../../components/ui/CheckTree'
import Drawer from '../../../components/ui/Drawer'
import ErrorState from '../../../components/ui/ErrorState'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Spinner from '../../../components/ui/Spinner'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { usePermissionTree } from '../../../hooks/usePermissionTree'
import { roleFormSchema, type RoleFormValues } from './schemas'

export type RoleDrawerState = { mode: 'create' } | { mode: 'edit'; role: Role } | null

interface RoleDrawerProps {
  state: RoleDrawerState
  onClose: () => void
  onSaved: (role: Role) => void
}

function RoleForm({ role, onCancel, onSaved }: { role?: Role; onCancel: () => void; onSaved: (role: Role) => void }) {
  const toast = useToast()
  const isEdit = role !== undefined
  // 编辑时不需要权限树（权限走"分配权限"）
  const perms = usePermissionTree(!isEdit)

  const form = useForm<RoleFormValues>({
    resolver: zodResolver(roleFormSchema),
    defaultValues: role
      ? { code: role.code, name: role.name, description: role.description ?? '', permissions: role.permissions }
      : { code: '', name: '', description: '', permissions: [] },
  })

  const save = useMutation({
    mutationFn: (v: RoleFormValues): Promise<Role> => {
      if (!role) {
        return createRole({ code: v.code, name: v.name, description: v.description || undefined, permissions: v.permissions })
      }
      const body: RoleUpdate = {}
      if (v.name !== role.name) body.name = v.name
      if (v.description !== (role.description ?? '')) body.description = v.description
      if (Object.keys(body).length === 0) return Promise.resolve(role)
      return updateRole(role.id, body)
    },
    onSuccess: (saved) => {
      toast.success(isEdit ? '角色已更新' : '角色已创建')
      onSaved(saved)
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '创建失败', errorMessage(e)),
  })

  const selectedCount = useWatch({ control: form.control, name: 'permissions' }).length

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="code" label="角色代码" required hint={isEdit ? '代码不可修改' : '2–32 位小写字母、数字或下划线，租户内唯一'}>
          <Input autoComplete="off" disabled={isEdit} placeholder="如 fleet_manager" spellCheck={false} {...form.register('code')} />
        </FormField>
        <FormField name="name" label="角色名称" required>
          <Input placeholder="如 车队管理员" {...form.register('name')} />
        </FormField>
      </div>
      <FormField name="description" label="描述">
        <Textarea rows={2} placeholder="可选，说明该角色的职责范围" {...form.register('description')} />
      </FormField>

      {!isEdit && (
        <FormField
          name="permissions"
          label={
            <span>
              初始权限 <span className="font-normal text-ink-faint">（可选，已选 {selectedCount} 项；创建后也可在"分配权限"中调整）</span>
            </span>
          }
        >
          <div className="rounded-lg border border-line-strong p-2 max-h-72 overflow-y-auto">
            {perms.query.isPending ? (
              <div className="flex justify-center py-6">
                <Spinner size="sm" label="加载权限树" />
              </div>
            ) : perms.query.isError ? (
              <ErrorState size="sm" message={errorMessage(perms.query.error)} onRetry={() => void perms.query.refetch()} />
            ) : (
              <Controller
                control={form.control}
                name="permissions"
                render={({ field }) => <CheckTree nodes={perms.nodes} value={field.value} onChange={field.onChange} size="sm" defaultExpandAll={false} />}
              />
            )}
          </div>
        </FormField>
      )}

      <div className="flex justify-end gap-2 pt-2 border-t border-line">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '创建角色'}
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑角色抽屉；编辑只开放名称与描述（内置角色亦然） */
export default function RoleDrawer({ state, onClose, onSaved }: RoleDrawerProps) {
  const role = state?.mode === 'edit' ? state.role : undefined
  return (
    <Drawer
      open={state !== null}
      onClose={onClose}
      title={role ? `编辑角色 · ${role.code}` : '新建角色'}
      description={role ? (role.is_system ? '内置角色只能修改名称与描述' : '代码不可修改；权限请使用"分配权限"') : '角色只持有动作权限码，菜单可见性由权限推导'}
      width="md"
    >
      {state && <RoleForm key={role?.id ?? 'create'} role={role} onCancel={onClose} onSaved={onSaved} />}
    </Drawer>
  )
}
