import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createCard, updateCard } from '../../../api/cards'
import type { Card, CardUpdate } from '../../../api/types'
import Button from '../../../components/ui/Button'
import DateTimeInput from '../../../components/ui/DateTimeInput'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select from '../../../components/ui/Select'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { localToRFC3339, rfc3339ToLocal } from '../../../utils/format'
import { cardEditSchema, cardIssueSchema, cardStatusOptions, type CardEditValues, type CardIssueValues } from './schemas'
import UserPicker from './UserPicker'

export type CardDrawerState = { mode: 'create' } | { mode: 'edit'; card: Card } | null

interface CardDrawerProps {
  state: CardDrawerState
  onClose: () => void
  onSaved: (card: Card) => void
}

/** 发卡：UID + 可选持卡人 + 发卡时间 + 备注 */
function IssueForm({ onCancel, onSaved }: { onCancel: () => void; onSaved: (c: Card) => void }) {
  const toast = useToast()
  const form = useForm<CardIssueValues>({
    resolver: zodResolver(cardIssueSchema),
    defaultValues: { card_uid: '', holder: null, issued_at: rfc3339ToLocal(new Date().toISOString()), remark: '' },
  })

  const save = useMutation({
    mutationFn: (v: CardIssueValues) =>
      createCard({
        card_uid: v.card_uid.toUpperCase(),
        user_id: v.holder?.id ?? undefined,
        issued_at: localToRFC3339(v.issued_at),
        remark: v.remark || undefined,
      }),
    onSuccess: (c) => {
      toast.success('发卡成功', c.user_name ? `已绑定持卡人 ${c.user_name}` : undefined)
      onSaved(c)
    },
    onError: (e) => toast.error('发卡失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <FormField name="card_uid" label="卡片 UID" required hint="8–32 位十六进制；读卡器读出的物理 UID，租户内唯一">
        <Input placeholder="如 04A1B2C3D4E5F6" autoComplete="off" className="font-mono uppercase" maxLength={32} {...form.register('card_uid')} />
      </FormField>
      <FormField name="holder" label="持卡人" hint="可选；一个用户可持多张卡，也可发卡后再绑定">
        {({ id, invalid }) => <Controller control={form.control} name="holder" render={({ field }) => <UserPicker id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
      </FormField>
      <FormField name="issued_at" label="发卡时间" hint="留空则以服务器当前时间为准">
        {({ id, invalid }) => <Controller control={form.control} name="issued_at" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />}
      </FormField>
      <FormField name="remark" label="备注">
        <Textarea placeholder="可选" {...form.register('remark')} />
      </FormField>
      <div className="flex justify-end gap-2 border-t border-slate-100 pt-2">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          发卡
        </Button>
      </div>
    </Form>
  )
}

/** 编辑：状态 + 备注（status=lost 等同挂失） */
function EditForm({ card, onCancel, onSaved }: { card: Card; onCancel: () => void; onSaved: (c: Card) => void }) {
  const toast = useToast()
  const form = useForm<CardEditValues>({
    resolver: zodResolver(cardEditSchema),
    defaultValues: { status: card.status, remark: card.remark ?? '' },
  })

  const save = useMutation({
    mutationFn: async (v: CardEditValues): Promise<Card> => {
      const body: CardUpdate = {}
      if (v.status !== card.status) body.status = v.status
      if (v.remark !== (card.remark ?? '')) body.remark = v.remark
      if (Object.keys(body).length === 0) return card
      return updateCard(card.id, body)
    },
    onSuccess: (c) => {
      toast.success('卡片已更新')
      onSaved(c)
    },
    onError: (e) => toast.error('更新失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="rounded-lg bg-slate-50 px-3 py-2 text-sm">
        <span className="text-slate-400">UID </span>
        <span className="font-mono text-slate-800">{card.card_uid}</span>
        {card.user_name && (
          <>
            <span className="ml-3 text-slate-400">持卡人 </span>
            <span className="text-slate-800">{card.user_name}</span>
          </>
        )}
      </div>
      <FormField name="status" label="状态" required hint="挂失 / 停用后刷卡取车将被拒绝">
        <Select options={cardStatusOptions} {...form.register('status')} />
      </FormField>
      <FormField name="remark" label="备注">
        <Textarea {...form.register('remark')} />
      </FormField>
      <div className="flex justify-end gap-2 border-t border-slate-100 pt-2">
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

/** 发卡 / 编辑卡抽屉 */
export default function CardDrawer({ state, onClose, onSaved }: CardDrawerProps) {
  const card = state?.mode === 'edit' ? state.card : undefined
  return (
    <Drawer open={state !== null} onClose={onClose} title={card ? `编辑卡片 · ${card.card_uid}` : '发卡'} description={card ? 'UID 不可修改；绑定 / 解绑持卡人请使用列表操作' : '录入 NFC 卡并可同时绑定持卡人'} width="sm">
      {state?.mode === 'create' && <IssueForm key="create" onCancel={onClose} onSaved={onSaved} />}
      {card && <EditForm key={card.id} card={card} onCancel={onClose} onSaved={onSaved} />}
    </Drawer>
  )
}
