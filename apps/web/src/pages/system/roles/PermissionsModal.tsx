import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { setRolePermissions } from '../../../api/roles'
import type { Role } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import CheckTree from '../../../components/ui/CheckTree'
import ErrorState from '../../../components/ui/ErrorState'
import Modal from '../../../components/ui/Modal'
import Spinner from '../../../components/ui/Spinner'
import { useToast } from '../../../components/ui/toast-context'
import { usePermissionTree } from '../../../hooks/usePermissionTree'
import { sameStringSet } from '../../../utils/array'

interface Props {
  role: Role | null
  onClose: () => void
  onSaved: (role: Role) => void
}

function PermissionsBody({ role, onClose, onSaved }: Props & { role: Role }) {
  const toast = useToast()
  const { query, nodes, actionCodes } = usePermissionTree()
  const [selected, setSelected] = useState<string[]>(role.permissions)

  const mutation = useMutation({
    mutationFn: () => setRolePermissions(role.id, selected),
    onSuccess: (saved) => {
      toast.success('权限已更新', '持有该角色的用户下次请求即生效')
      onSaved(saved)
    },
    onError: (e) => toast.error('保存失败', errorMessage(e)),
  })

  const unchanged = sameStringSet(selected, role.permissions)
  // 后端返回但当前树里没有的码（如平台专属权限在非平台租户不可见）：保留但提示
  const known = new Set(actionCodes)
  const unknown = selected.filter((c) => !known.has(c))

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-ink">
        <div>
          为 <span className="font-medium text-ink-strong">{role.name}</span>
          <span className="ml-1 font-mono text-xs text-ink-faint">{role.code}</span> 设置权限，将整体替换现有权限。
        </div>
        <div className="flex items-center gap-2">
          <Badge color="blue">已选 {selected.length} 项</Badge>
          <Button size="sm" variant="ghost" disabled={query.isPending || actionCodes.length === 0} onClick={() => setSelected([...actionCodes, ...unknown])}>
            全选
          </Button>
          <Button size="sm" variant="ghost" disabled={selected.length === 0} onClick={() => setSelected([])}>
            清空
          </Button>
        </div>
      </div>

      <div className="rounded-lg border border-line-strong p-2 max-h-[55vh] overflow-y-auto">
        {query.isPending ? (
          <div className="flex justify-center py-8">
            <Spinner label="加载权限树" />
          </div>
        ) : query.isError ? (
          <ErrorState size="sm" message={errorMessage(query.error)} onRetry={() => void query.refetch()} />
        ) : (
          <CheckTree nodes={nodes} value={selected} onChange={setSelected} emptyText="暂无可分配的权限点" />
        )}
      </div>

      {unknown.length > 0 && (
        <p className="text-xs text-warn-200 bg-warn-500/10 rounded-lg px-3 py-2">
          该角色持有 {unknown.length} 个当前权限树未列出的权限码（{unknown.join('、')}），保存时将原样保留。
        </p>
      )}

      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
          取消
        </Button>
        <Button onClick={() => mutation.mutate()} loading={mutation.isPending} disabled={unchanged || query.isError}>
          保存
        </Button>
      </div>
    </div>
  )
}

/** 分配权限弹窗：可勾选的权限树（勾父全选子、半选态） */
export default function PermissionsModal(props: Props) {
  const { role, onClose } = props
  return (
    <Modal open={role !== null} onClose={onClose} title="分配权限" size="lg">
      {role && <PermissionsBody key={role.id} {...props} role={role} />}
    </Modal>
  )
}
