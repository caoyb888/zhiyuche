import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'
import { errorMessage } from '../../../api/client'
import { createDept, updateDept } from '../../../api/depts'
import type { Dept, DeptNode, DeptUpdate } from '../../../api/types'
import { listUsers } from '../../../api/users'
import Button from '../../../components/ui/Button'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select, { type SelectOption } from '../../../components/ui/Select'
import { useToast } from '../../../components/ui/toast-context'
import type { TreeNode } from '../../../components/ui/tree-utils'
import TreeSelect from '../../../components/ui/TreeSelect'
import { useDebounce } from '../../../hooks/useDebounce'
import { collectDeptIds } from '../../../hooks/useDeptTree'

const schema = z.object({
  name: z.string().trim().min(1, '请输入部门名称').max(64, '名称最多 64 个字符'),
  code: z.string().trim().max(64, '编码最多 64 个字符'),
  sort: z.string().trim().regex(/^-?\d*$/, '排序须为整数'),
  leader_user_id: z.string(),
  monthly_budget: z
    .string()
    .trim()
    .regex(/^(\d+(\.\d{1,2})?)?$/, '预算须为非负数字，最多两位小数'),
  status: z.enum(['active', 'disabled']),
  parent_id: z.string().nullable(),
})

type DeptFormValues = z.infer<typeof schema>

const statusOptions: SelectOption[] = [
  { value: 'active', label: '启用' },
  { value: 'disabled', label: '停用' },
]

export type DeptFormProps =
  | { mode: 'create'; parentId: string | null; depts: DeptNode[]; nodes: TreeNode[]; onSaved: (dept: Dept) => void; onCancel: () => void }
  | { mode: 'edit'; dept: DeptNode; depts: DeptNode[]; nodes: TreeNode[]; onSaved: (dept: Dept) => void; onCancel?: () => void }

const toInt = (v: string): number => (v === '' ? 0 : Number(v))
const toNumber = (v: string): number => (v === '' ? 0 : Number(v))

/** 部门表单：新建（根 / 子）与编辑共用；编辑时上级部门不可选到自身子树 */
export default function DeptForm(props: DeptFormProps) {
  const toast = useToast()
  const dept = props.mode === 'edit' ? props.dept : undefined
  const isEdit = dept !== undefined

  const form = useForm<DeptFormValues>({
    resolver: zodResolver(schema),
    defaultValues: dept
      ? {
          name: dept.name,
          code: dept.code ?? '',
          sort: String(dept.sort),
          leader_user_id: dept.leader_user_id ?? '',
          monthly_budget: dept.monthly_budget === 0 ? '' : String(dept.monthly_budget),
          status: dept.status,
          parent_id: dept.parent_id ?? null,
        }
      : {
          name: '',
          code: '',
          sort: '0',
          leader_user_id: '',
          monthly_budget: '',
          status: 'active',
          parent_id: props.mode === 'create' ? props.parentId : null,
        },
  })

  // 编辑时禁止把部门移到自己的子树下
  const disabledKeys = useMemo(() => (dept ? collectDeptIds(dept) : undefined), [dept])

  // 负责人：缺省列出本部门（含子部门）用户，输入关键词时在全租户搜索
  const [leaderKeyword, setLeaderKeyword] = useState('')
  const kw = useDebounce(leaderKeyword.trim(), 300)
  const leaders = useQuery({
    queryKey: ['users', 'leader-options', { kw, dept: dept?.id ?? null }],
    queryFn: () => listUsers({ pageSize: 200, keyword: kw || undefined, dept_id: kw ? undefined : dept?.id, status: 'active' }),
    staleTime: 30_000,
  })
  const leaderOptions = useMemo<SelectOption[]>(() => {
    const opts = (leaders.data?.items ?? []).map((u) => ({ value: u.id, label: `${u.name}（${u.username}）` }))
    if (dept?.leader_user_id && !opts.some((o) => o.value === dept.leader_user_id)) {
      opts.unshift({ value: dept.leader_user_id, label: dept.leader_name ?? '当前负责人' })
    }
    return opts
  }, [leaders.data, dept])

  const save = useMutation({
    mutationFn: (v: DeptFormValues): Promise<Dept> => {
      if (!dept) {
        return createDept({
          name: v.name,
          code: v.code || undefined,
          sort: toInt(v.sort),
          leader_user_id: v.leader_user_id || undefined,
          monthly_budget: toNumber(v.monthly_budget),
          status: v.status,
          parent_id: v.parent_id ?? undefined,
        })
      }
      const body: DeptUpdate = {
        name: v.name,
        code: v.code,
        sort: toInt(v.sort),
        monthly_budget: toNumber(v.monthly_budget),
        status: v.status,
      }
      if (v.parent_id) body.parent_id = v.parent_id
      else if (dept.parent_id) body.clear_parent = true
      if (v.leader_user_id) body.leader_user_id = v.leader_user_id
      else if (dept.leader_user_id) body.clear_leader = true
      return updateDept(dept.id, body)
    },
    onSuccess: (saved) => {
      toast.success(isEdit ? '部门已更新' : '部门已创建')
      props.onSaved(saved)
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '创建失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="name" label="部门名称" required>
          <Input placeholder="如 销售部" {...form.register('name')} />
        </FormField>
        <FormField name="code" label="部门编码" hint="可选，便于对接外部系统">
          <Input placeholder="如 SALES" {...form.register('code')} />
        </FormField>
      </div>

      <FormField name="parent_id" label="上级部门" hint={isEdit ? '可移动到其他部门下；不能移到自身子树' : '留空则创建为根部门'}>
        {({ id, invalid }) => (
          <Controller
            control={form.control}
            name="parent_id"
            render={({ field }) => (
              <TreeSelect id={id} invalid={invalid} nodes={props.nodes} value={field.value} onChange={field.onChange} disabledKeys={disabledKeys} placeholder="（根部门）" />
            )}
          />
        )}
      </FormField>

      <FormField name="leader_user_id" label="负责人" hint={leaders.isError ? `用户列表加载失败：${errorMessage(leaders.error)}` : '可输入姓名 / 用户名搜索'}>
        {({ id, invalid }) => (
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input className="sm:w-44" placeholder="搜索用户" value={leaderKeyword} onChange={(e) => setLeaderKeyword(e.target.value)} aria-label="搜索负责人" />
            <Controller
              control={form.control}
              name="leader_user_id"
              render={({ field }) => (
                <Select
                  id={id}
                  invalid={invalid}
                  options={leaderOptions}
                  placeholder={leaders.isFetching ? '加载中…' : '未指定'}
                  value={field.value}
                  onChange={(e) => field.onChange(e.target.value)}
                />
              )}
            />
          </div>
        )}
      </FormField>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <FormField name="sort" label="排序" hint="数字越小越靠前">
          <Input inputMode="numeric" {...form.register('sort')} />
        </FormField>
        <FormField name="monthly_budget" label="月度预算（元）">
          <Input inputMode="decimal" placeholder="0.00" {...form.register('monthly_budget')} />
        </FormField>
        <FormField name="status" label="状态" required>
          <Select options={statusOptions} {...form.register('status')} />
        </FormField>
      </div>

      <div className="flex justify-end gap-2 pt-2 border-t border-slate-100">
        {props.onCancel && (
          <Button variant="secondary" onClick={props.onCancel} disabled={save.isPending}>
            取消
          </Button>
        )}
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '创建部门'}
        </Button>
      </div>
    </Form>
  )
}
