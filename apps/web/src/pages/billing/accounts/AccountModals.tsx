import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { z } from 'zod'
import { adjustAccount, allocateAccount, rechargeEnterprise, updateAccount, type AccountUpdateBody, type AllocateBody } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { Account, AccountNode, UserBrief } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Modal from '../../../components/ui/Modal'
import Select, { type SelectOption } from '../../../components/ui/Select'
import Switch from '../../../components/ui/Switch'
import Tabs from '../../../components/ui/Tabs'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import TreeSelect from '../../../components/ui/TreeSelect'
import UserPicker from '../../../components/UserPicker'
import { useDeptTree } from '../../../hooks/useDeptTree'
import { formatMoney } from '../../../utils/format'
import { ACCOUNT_LEVEL_BADGE, ACCOUNT_LEVEL_LABEL, childLevel } from '../style'
import { nodeExists, nodeKey } from './utils'

export type AccountModalState =
  | { kind: 'recharge' }
  | { kind: 'allocate'; source: AccountNode; target?: AccountNode }
  | { kind: 'adjust'; account: Account }
  | { kind: 'edit'; account: Account }
  | null

interface AccountModalsProps {
  state: AccountModalState
  onClose: () => void
  /** 操作成功后（刷新账户树 / 流水） */
  onDone: () => void
}

const REMARK_MAX = 200

const positiveMoney = z.string().trim().refine((v) => v !== '' && Number.isFinite(Number(v)) && Number(v) > 0, '金额须为大于 0 的数字')
const nonNegativeMoney = z.string().trim().refine((v) => v !== '' && Number.isFinite(Number(v)) && Number(v) >= 0, '须为 ≥ 0 的数字')
const remark = z.string().trim().max(REMARK_MAX, `备注最多 ${REMARK_MAX} 个字符`)

function AccountLine({ a }: { a: Pick<Account, 'owner_name' | 'level' | 'balance' | 'credit_limit'> }) {
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      <Badge color={ACCOUNT_LEVEL_BADGE[a.level]}>{ACCOUNT_LEVEL_LABEL[a.level]}</Badge>
      <span className="font-medium text-ink-strong">{a.owner_name}</span>
      <span className="text-xs text-ink-faint">
        余额 <span className="font-mono">{formatMoney(a.balance)}</span> · 透支额度 <span className="font-mono">{formatMoney(a.credit_limit)}</span>
      </span>
    </div>
  )
}

// ── 充值 ─────────────────────────────────────────────

const rechargeSchema = z.object({ amount: positiveMoney, remark })
type RechargeValues = z.infer<typeof rechargeSchema>

function RechargeForm({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const toast = useToast()
  const form = useForm<RechargeValues>({ resolver: zodResolver(rechargeSchema), defaultValues: { amount: '', remark: '' } })
  const save = useMutation({
    mutationFn: (v: RechargeValues) => rechargeEnterprise({ amount: Number(v.amount), remark: v.remark || undefined }),
    onSuccess: (tx) => {
      toast.success('充值已登记', `企业账户余额 ${formatMoney(tx.balance_after)} 元`)
      onDone()
    },
    onError: (e) => toast.error('充值失败', errorMessage(e)),
  })
  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="rounded-lg bg-surface-3 px-3 py-2.5 text-xs text-ink-muted">线下到账后在此登记，金额计入企业账户余额（流水类型：充值）</div>
      <FormField name="amount" label="充值金额（元）" required>
        <Input type="number" inputMode="decimal" min={0} step="0.01" placeholder="如 50000" autoFocus {...form.register('amount')} />
      </FormField>
      <FormField name="remark" label="备注" hint="如 转账流水号、付款方">
        <Textarea rows={2} maxLength={REMARK_MAX} {...form.register('remark')} />
      </FormField>
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          确认充值
        </Button>
      </div>
    </Form>
  )
}

// ── 划拨 ─────────────────────────────────────────────

const allocateSchema = z.object({ amount: positiveMoney, remark })
type AllocateValues = z.infer<typeof allocateSchema>
type TargetMode = 'child' | 'pick'

interface AllocateFormProps {
  source: AccountNode
  target?: AccountNode
  onClose: () => void
  onDone: () => void
}

function AllocateForm({ source, target, onClose, onDone }: AllocateFormProps) {
  const toast = useToast()
  const toLevel = childLevel(source.level)
  const children = source.children ?? []
  const [mode, setMode] = useState<TargetMode>(children.length > 0 ? 'child' : 'pick')
  const [childKey, setChildKey] = useState<string>(children[0] ? nodeKey(children[0]) : '')
  const [deptId, setDeptId] = useState<string | null>(null)
  const [user, setUser] = useState<UserBrief | null>(null)
  const [targetError, setTargetError] = useState<string | null>(null)
  const { nodes: deptNodes, query: deptQuery } = useDeptTree(!target && source.level === 'enterprise')

  const form = useForm<AllocateValues>({ resolver: zodResolver(allocateSchema), defaultValues: { amount: '', remark: '' } })
  const available = source.balance + source.credit_limit

  const childOptions: SelectOption[] = children.map((c) => ({ value: nodeKey(c), label: `${c.owner_name}${nodeExists(c) ? `（余额 ${formatMoney(c.balance)}）` : '（未创建）'}` }))

  /** 目标 → 请求体（已存在用 to_account_id，否则 to_level + to_owner_id 自动创建） */
  const resolveTarget = (): Pick<AllocateBody, 'to_account_id' | 'to_level' | 'to_owner_id'> | null => {
    if (!toLevel) return null
    const fromNode = (n: AccountNode) => (nodeExists(n) ? { to_account_id: n.id } : { to_level: n.level, to_owner_id: n.owner_id })
    if (target) return fromNode(target)
    if (mode === 'child') {
      const n = children.find((c) => nodeKey(c) === childKey)
      return n ? fromNode(n) : null
    }
    if (source.level === 'enterprise') return deptId ? { to_level: 'department', to_owner_id: deptId } : null
    return user ? { to_level: 'employee', to_owner_id: user.id } : null
  }

  const save = useMutation({
    mutationFn: (v: AllocateValues) => {
      const to = resolveTarget()
      if (!to) throw new Error('请选择划拨目标')
      return allocateAccount(source.id, { ...to, amount: Number(v.amount), remark: v.remark || undefined })
    },
    onSuccess: (r) => {
      toast.success('划拨成功', r.from ? `「${source.owner_name}」余额 ${formatMoney(r.from.balance_after)} 元` : undefined)
      onDone()
    },
    // 409：余额不足（含透支额度）
    onError: (e) => toast.error('划拨失败', errorMessage(e)),
  })

  const submit = (v: AllocateValues) => {
    if (!resolveTarget()) {
      setTargetError('请选择划拨目标')
      return
    }
    setTargetError(null)
    save.mutate(v)
  }

  if (!toLevel) return <div className="text-sm text-ink-muted">员工账户没有下级，无法向下划拨。</div>

  return (
    <Form form={form} onSubmit={submit}>
      <div className="space-y-2 rounded-lg bg-surface-3 px-3 py-2.5">
        <div className="text-[11px] text-ink-faint">划出账户</div>
        <AccountLine a={source} />
        <div className="text-[11px] text-ink-faint">
          可用额度（余额 + 透支）<span className="font-mono text-ink">{formatMoney(available)}</span> 元；超出将被拒绝
        </div>
      </div>

      <div className="space-y-2">
        <div className="text-sm font-medium text-ink">划入{ACCOUNT_LEVEL_LABEL[toLevel]}账户</div>
        {target ? (
          <div className="rounded-lg border border-line-strong px-3 py-2.5">
            <AccountLine a={target} />
            {!nodeExists(target) && <div className="mt-1 text-[11px] text-brand-600">该账户尚未创建，划拨成功后自动创建</div>}
          </div>
        ) : (
          <>
            <Tabs<TargetMode>
              size="sm"
              items={[
                { key: 'child', label: '从下级列表选择', count: children.length, disabled: children.length === 0 },
                { key: 'pick', label: source.level === 'enterprise' ? '选择部门' : '搜索员工' },
              ]}
              value={mode}
              onChange={setMode}
            />
            {mode === 'child' ? (
              <Select options={childOptions} value={childKey} onChange={(e) => setChildKey(e.target.value)} placeholder="请选择" invalid={Boolean(targetError)} aria-label="划入账户" />
            ) : source.level === 'enterprise' ? (
              <TreeSelect nodes={deptNodes} value={deptId} onChange={setDeptId} placeholder="选择部门（不存在的账户将自动创建）" loading={deptQuery.isLoading} invalid={Boolean(targetError)} emptyText={deptQuery.isError ? '部门数据暂不可用' : '暂无部门'} />
            ) : (
              <UserPicker value={user} onChange={setUser} placeholder="搜索员工姓名 / 用户名" invalid={Boolean(targetError)} />
            )}
            {targetError && (
              <p className="text-xs text-danger-200" role="alert">
                {targetError}
              </p>
            )}
          </>
        )}
      </div>

      <FormField name="amount" label="划拨金额（元）" required>
        <Input type="number" inputMode="decimal" min={0} step="0.01" placeholder="如 5000" {...form.register('amount')} />
      </FormField>
      <FormField name="remark" label="备注">
        <Textarea rows={2} maxLength={REMARK_MAX} placeholder="可选" {...form.register('remark')} />
      </FormField>
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          确认划拨
        </Button>
      </div>
    </Form>
  )
}

// ── 调整 ─────────────────────────────────────────────

const adjustSchema = z.object({
  amount: z.string().trim().refine((v) => v !== '' && Number.isFinite(Number(v)) && Number(v) !== 0, '金额须为非 0 数字：正数入账，负数扣减'),
  type: z.enum(['adjust', 'refund']),
  remark: z.string().trim().min(1, '调整必须填写原因').max(REMARK_MAX, `备注最多 ${REMARK_MAX} 个字符`),
})
type AdjustValues = z.infer<typeof adjustSchema>

const ADJUST_TYPE_OPTIONS: SelectOption[] = [
  { value: 'adjust', label: '调整（人工更正）' },
  { value: 'refund', label: '退款（返还扣费）' },
]

function AdjustForm({ account, onClose, onDone }: { account: Account; onClose: () => void; onDone: () => void }) {
  const toast = useToast()
  const form = useForm<AdjustValues>({ resolver: zodResolver(adjustSchema), defaultValues: { amount: '', type: 'adjust', remark: '' } })
  const save = useMutation({
    mutationFn: (v: AdjustValues) => adjustAccount(account.id, { amount: Number(v.amount), type: v.type, remark: v.remark }),
    onSuccess: (tx) => {
      toast.success('余额已调整', `「${account.owner_name}」余额 ${formatMoney(tx.balance_after)} 元`)
      onDone()
    },
    onError: (e) => toast.error('调整失败', errorMessage(e)),
  })
  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="rounded-lg bg-surface-3 px-3 py-2.5">
        <AccountLine a={account} />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="amount" label="金额（元，有符号）" required hint="正数入账，负数扣减">
          <Input type="number" inputMode="decimal" step="0.01" placeholder="如 -120.5" autoFocus {...form.register('amount')} />
        </FormField>
        <FormField name="type" label="类型" required>
          <Select options={ADJUST_TYPE_OPTIONS} {...form.register('type')} />
        </FormField>
      </div>
      <FormField name="remark" label="原因" required>
        <Textarea rows={3} maxLength={REMARK_MAX} placeholder="必填：如 行程 T-20260909-012 重复扣费退回" {...form.register('remark')} />
      </FormField>
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          确认调整
        </Button>
      </div>
    </Form>
  )
}

// ── 编辑 ─────────────────────────────────────────────

const editSchema = z.object({ credit_limit: nonNegativeMoney, monthly_budget: nonNegativeMoney, frozen: z.boolean() })
type EditValues = z.infer<typeof editSchema>

function EditForm({ account, onClose, onDone }: { account: Account; onClose: () => void; onDone: () => void }) {
  const toast = useToast()
  const form = useForm<EditValues>({
    resolver: zodResolver(editSchema),
    defaultValues: { credit_limit: String(account.credit_limit), monthly_budget: String(account.monthly_budget), frozen: account.status === 'frozen' },
  })
  const save = useMutation({
    mutationFn: (v: EditValues) => {
      const body: AccountUpdateBody = {}
      const credit = Number(v.credit_limit)
      const budget = Number(v.monthly_budget)
      if (credit !== account.credit_limit) body.credit_limit = credit
      if (budget !== account.monthly_budget) body.monthly_budget = budget
      const status = v.frozen ? 'frozen' : 'active'
      if (status !== account.status) body.status = status
      if (Object.keys(body).length === 0) return Promise.resolve(account)
      return updateAccount(account.id, body)
    },
    onSuccess: () => {
      toast.success('账户已更新')
      onDone()
    },
    onError: (e) => toast.error('更新失败', errorMessage(e)),
  })
  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="rounded-lg bg-surface-3 px-3 py-2.5">
        <AccountLine a={account} />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="credit_limit" label="透支额度（元）" hint="余额可透支到 -额度，0 表示不允许透支">
          <Input type="number" inputMode="decimal" min={0} step="0.01" {...form.register('credit_limit')} />
        </FormField>
        <FormField name="monthly_budget" label="月度预算（元）" hint="用于本月支出进度与结算单预算对比，0 表示不设">
          <Input type="number" inputMode="decimal" min={0} step="0.01" {...form.register('monthly_budget')} />
        </FormField>
      </div>
      <Controller control={form.control} name="frozen" render={({ field }) => <Switch checked={field.value} onChange={field.onChange} label={field.value ? '已冻结：不再扣费与划拨' : '正常：可扣费与划拨'} size="sm" />} />
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

/** 账户操作弹窗：充值 / 划拨 / 调整 / 编辑 */
export default function AccountModals({ state, onClose, onDone }: AccountModalsProps) {
  const title = state?.kind === 'recharge' ? '企业账户充值' : state?.kind === 'allocate' ? '额度划拨' : state?.kind === 'adjust' ? '余额调整' : state?.kind === 'edit' ? '编辑账户' : ''
  return (
    <Modal open={state !== null} onClose={onClose} title={title} size="md">
      {state?.kind === 'recharge' && <RechargeForm onClose={onClose} onDone={onDone} />}
      {state?.kind === 'allocate' && <AllocateForm key={`${state.source.id}:${state.target ? nodeKey(state.target) : ''}`} source={state.source} target={state.target} onClose={onClose} onDone={onDone} />}
      {state?.kind === 'adjust' && <AdjustForm key={state.account.id} account={state.account} onClose={onClose} onDone={onDone} />}
      {state?.kind === 'edit' && <EditForm key={`${state.account.id}:${state.account.updated_at}`} account={state.account} onClose={onClose} onDone={onDone} />}
    </Modal>
  )
}
