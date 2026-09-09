import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import type { User } from '../../../api/types'
import { assignUserRoles } from '../../../api/users'
import Button from '../../../components/ui/Button'
import CheckboxGroup from '../../../components/ui/CheckboxGroup'
import Modal from '../../../components/ui/Modal'
import type { SelectOption } from '../../../components/ui/Select'
import { useToast } from '../../../components/ui/toast-context'
import { sameIdSet } from './schemas'

interface Props {
  user: User | null
  roleOptions: SelectOption[]
  rolesUnavailable: boolean
  onClose: () => void
  onSaved: () => void
}

function AssignRolesBody({ user, roleOptions, rolesUnavailable, onClose, onSaved }: Props & { user: User }) {
  const toast = useToast()
  const initial = user.roles.map((r) => r.id)
  const [selected, setSelected] = useState<string[]>(initial)

  // 角色列表不可用时补入用户已有角色，保证已勾选项可见
  const known = new Set(roleOptions.map((o) => o.value))
  const options: SelectOption[] = [...roleOptions, ...user.roles.filter((r) => !known.has(r.id)).map((r) => ({ value: r.id, label: r.name }))]

  const mutation = useMutation({
    mutationFn: () => assignUserRoles(user.id, selected),
    onSuccess: () => {
      toast.success('角色已更新')
      onSaved()
    },
    onError: (e) => toast.error('分配失败', errorMessage(e)),
  })

  const unchanged = sameIdSet(selected, initial)

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-600">
        为 <span className="font-medium text-slate-800">{user.name}</span>（{user.username}）分配角色，将整体替换现有角色。
      </p>
      {rolesUnavailable && options.length === 0 ? (
        <p className="text-sm text-slate-400 rounded-lg bg-slate-50 px-3 py-2">角色数据暂不可用，请稍后再试。</p>
      ) : (
        <CheckboxGroup options={options} value={selected} onChange={setSelected} emptyText="本租户尚无角色" />
      )}
      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
          取消
        </Button>
        <Button onClick={() => mutation.mutate()} loading={mutation.isPending} disabled={unchanged}>
          保存
        </Button>
      </div>
    </div>
  )
}

/** 分配角色弹窗（整体替换） */
export default function AssignRolesModal(props: Props) {
  const { user, onClose } = props
  return (
    <Modal open={user !== null} onClose={onClose} title="分配角色" size="sm">
      {user && <AssignRolesBody key={user.id} {...props} user={user} />}
    </Modal>
  )
}
