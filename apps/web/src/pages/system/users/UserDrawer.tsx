import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import type { User, UserUpdate } from '../../../api/types'
import { assignUserRoles, createUser, updateUser } from '../../../api/users'
import Button from '../../../components/ui/Button'
import CheckboxGroup from '../../../components/ui/CheckboxGroup'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select, { type SelectOption } from '../../../components/ui/Select'
import { useToast } from '../../../components/ui/toast-context'
import type { TreeNode } from '../../../components/ui/tree-utils'
import TreeSelect from '../../../components/ui/TreeSelect'
import { usePermission } from '../../../hooks/usePermission'
import { sameIdSet, userFormSchema, userStatusOptions, type UserFormValues } from './schemas'

export type UserDrawerState = { mode: 'create' } | { mode: 'edit'; user: User } | null

interface SharedProps {
  deptNodes: TreeNode[]
  deptLoading: boolean
  deptUnavailable: boolean
  roleOptions: SelectOption[]
  rolesUnavailable: boolean
}

interface UserDrawerProps extends SharedProps {
  state: UserDrawerState
  onClose: () => void
  onSaved: () => void
}

interface UserFormProps extends SharedProps {
  user?: User
  onCancel: () => void
  onSaved: () => void
}

const STATUS_FORM_OPTIONS = userStatusOptions.filter((o) => o.value !== 'locked')

function defaultsFor(user: User | undefined): UserFormValues {
  if (!user) {
    return { username: '', name: '', password: '', phone: '', email: '', employee_no: '', dept_id: null, role_ids: [], status: 'active' }
  }
  return {
    username: user.username,
    name: user.name,
    password: '',
    phone: user.phone ?? '',
    email: user.email ?? '',
    employee_no: user.employee_no ?? '',
    dept_id: user.dept_id ?? null,
    role_ids: user.roles.map((r) => r.id),
    status: user.status,
  }
}

function UserForm({ user, deptNodes, deptLoading, deptUnavailable, roleOptions, rolesUnavailable, onCancel, onSaved }: UserFormProps) {
  const toast = useToast()
  const { can } = usePermission()
  const isEdit = user !== undefined
  const canAssign = can('system:user:assign-roles')
  const showRoles = !isEdit || canAssign

  const form = useForm<UserFormValues>({
    resolver: zodResolver(userFormSchema),
    defaultValues: defaultsFor(user),
  })

  // 编辑已锁定用户时才提供"锁定"选项（锁定由后端产生）
  const statusOptions = isEdit && user.status === 'locked' ? userStatusOptions : STATUS_FORM_OPTIONS

  // 角色列表不可用时把用户已有角色补进选项，保证已勾选项可见
  const roleChoices: SelectOption[] = (() => {
    if (!user) return roleOptions
    const known = new Set(roleOptions.map((o) => o.value))
    const extra = user.roles.filter((r) => !known.has(r.id)).map((r) => ({ value: r.id, label: r.name }))
    return [...roleOptions, ...extra]
  })()

  const save = useMutation({
    mutationFn: async (v: UserFormValues): Promise<User> => {
      if (!user) {
        return createUser({
          username: v.username,
          name: v.name,
          password: v.password || undefined,
          phone: v.phone || undefined,
          email: v.email || undefined,
          employee_no: v.employee_no || undefined,
          dept_id: v.dept_id ?? undefined,
          role_ids: v.role_ids,
          status: v.status === 'disabled' ? 'disabled' : 'active',
        })
      }
      // 只发送变化的字段；由有值改为空时发送空串以清除
      const body: UserUpdate = {}
      if (v.name !== user.name) body.name = v.name
      if (v.status !== user.status) body.status = v.status
      const textFields = ['phone', 'email', 'employee_no'] as const
      for (const key of textFields) {
        const prev = user[key] ?? ''
        if (v[key] !== prev) body[key] = v[key]
      }
      if (v.dept_id !== (user.dept_id ?? null)) {
        if (v.dept_id) body.dept_id = v.dept_id
        else body.clear_dept = true
      }
      let updated = user
      if (Object.keys(body).length > 0) updated = await updateUser(user.id, body)
      const currentRoleIds = user.roles.map((r) => r.id)
      if (canAssign && !sameIdSet(v.role_ids, currentRoleIds)) {
        updated = await assignUserRoles(user.id, v.role_ids)
      }
      return updated
    },
    onSuccess: () => {
      toast.success(isEdit ? '用户已更新' : '用户已创建')
      onSaved()
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '创建失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="username" label="用户名" required hint={isEdit ? '用户名不可修改' : '2–64 个字符，租户内唯一'}>
          <Input autoComplete="off" disabled={isEdit} placeholder="登录账号" {...form.register('username')} />
        </FormField>
        <FormField name="name" label="姓名" required>
          <Input placeholder="真实姓名" {...form.register('name')} />
        </FormField>
      </div>

      {!isEdit && (
        <FormField name="password" label="初始密码" hint="留空则使用系统缺省密码；否则至少 8 位，含字母和数字">
          <Input type="password" autoComplete="new-password" placeholder="留空使用系统缺省密码" {...form.register('password')} />
        </FormField>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="phone" label="手机号">
          <Input inputMode="numeric" maxLength={11} placeholder="11 位数字" {...form.register('phone')} />
        </FormField>
        <FormField name="email" label="邮箱">
          <Input type="email" placeholder="name@example.com" {...form.register('email')} />
        </FormField>
        <FormField name="employee_no" label="工号">
          <Input placeholder="可选" {...form.register('employee_no')} />
        </FormField>
        <FormField name="status" label="状态" required>
          <Select options={statusOptions} {...form.register('status')} />
        </FormField>
      </div>

      <FormField name="dept_id" label="部门" hint={deptUnavailable ? '部门数据暂不可用，可稍后再分配' : undefined}>
        {({ id, invalid }) => (
          <Controller
            control={form.control}
            name="dept_id"
            render={({ field }) => (
              <TreeSelect
                id={id}
                invalid={invalid}
                nodes={deptNodes}
                value={field.value}
                onChange={field.onChange}
                loading={deptLoading}
                placeholder="未分配部门"
                emptyText={deptUnavailable ? '部门数据暂不可用' : '暂无部门'}
              />
            )}
          />
        )}
      </FormField>

      {showRoles && (
        <FormField name="role_ids" label="角色" hint={rolesUnavailable && roleChoices.length === 0 ? '角色数据暂不可用，可稍后再分配' : undefined}>
          {({ invalid }) => (
            <Controller
              control={form.control}
              name="role_ids"
              render={({ field }) => (
                <CheckboxGroup options={roleChoices} value={field.value} onChange={field.onChange} invalid={invalid} emptyText="暂无可选角色" />
              )}
            />
          )}
        </FormField>
      )}

      <div className="flex justify-end gap-2 pt-2 border-t border-slate-100">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '创建用户'}
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑用户抽屉；按用户 id 作为 key 重建表单，避免残留状态 */
export default function UserDrawer({ state, onClose, onSaved, ...shared }: UserDrawerProps) {
  const user = state?.mode === 'edit' ? state.user : undefined
  return (
    <Drawer
      open={state !== null}
      onClose={onClose}
      title={user ? `编辑用户 · ${user.username}` : '新建用户'}
      description={user ? '只提交有变化的字段' : '新用户缺省状态为正常'}
      width="md"
    >
      {state && <UserForm key={user?.id ?? 'create'} user={user} onCancel={onClose} onSaved={onSaved} {...shared} />}
    </Drawer>
  )
}
