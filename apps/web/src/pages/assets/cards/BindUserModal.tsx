import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { bindCard } from '../../../api/cards'
import { errorMessage } from '../../../api/client'
import type { Card } from '../../../api/types'
import Button from '../../../components/ui/Button'
import Modal from '../../../components/ui/Modal'
import { useToast } from '../../../components/ui/toast-context'
import UserPicker, { type PickedUser } from './UserPicker'

interface BindUserModalProps {
  card: Card | null
  onClose: () => void
  onSaved: (card: Card) => void
}

function BindBody({ card, onClose, onSaved }: { card: Card; onClose: () => void; onSaved: (c: Card) => void }) {
  const toast = useToast()
  const [user, setUser] = useState<PickedUser | null>(null)

  const bind = useMutation({
    mutationFn: () => bindCard(card.id, user?.id ?? ''),
    onSuccess: (c) => {
      toast.success('已绑定持卡人', c.user_name ?? undefined)
      onSaved(c)
    },
    onError: (e) => toast.error('绑定失败', errorMessage(e)),
  })

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-600">
        为卡片 <span className="font-mono font-medium text-slate-800">{card.card_uid}</span> {card.user_id ? '更换' : '绑定'}持卡人。
        {card.user_name && <span className="text-slate-400">（当前：{card.user_name}）</span>}
      </p>
      <div className="space-y-1.5">
        <label htmlFor="bind-user" className="block text-sm font-medium text-slate-700">
          持卡人
        </label>
        <UserPicker id="bind-user" value={user} onChange={setUser} />
      </div>
      <div className="flex justify-end gap-2 pt-2">
        <Button variant="secondary" onClick={onClose} disabled={bind.isPending}>
          取消
        </Button>
        <Button onClick={() => bind.mutate()} loading={bind.isPending} disabled={!user}>
          绑定
        </Button>
      </div>
    </div>
  )
}

/** 卡片绑定持卡人 */
export default function BindUserModal({ card, onClose, onSaved }: BindUserModalProps) {
  return (
    <Modal open={card !== null} onClose={onClose} title="绑定持卡人" size="sm">
      {card && <BindBody key={card.id} card={card} onClose={onClose} onSaved={onSaved} />}
    </Modal>
  )
}
