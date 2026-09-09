import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Globe, Pencil, RotateCcw, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { errorMessage } from '../../../api/client'
import { listParams, paramKeys, resetParam } from '../../../api/params'
import type { Param } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import Input from '../../../components/ui/Input'
import PageHeader from '../../../components/ui/PageHeader'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { usePermission } from '../../../hooks/usePermission'
import { useAuthStore } from '../../../store/auth'
import { formatDateTime, text } from '../../../utils/format'
import ParamEditModal from './ParamEditModal'
import { paramSourceColor, paramSourceLabel, paramTypeLabel } from './schemas'

function ValueCell({ p, value }: { p: Param; value: string | null | undefined }) {
  if (value === null || value === undefined) return <span className="text-slate-400">—</span>
  if (p.value_type === 'bool') return <Badge color={value === 'true' ? 'green' : 'gray'}>{value}</Badge>
  return (
    <code className="block max-w-[18rem] truncate font-mono text-xs text-slate-700" title={value}>
      {value === '' ? <span className="text-slate-400">（空）</span> : value}
    </code>
  )
}

export default function ParamsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can, isSuper } = usePermission()
  const viewTenant = useAuthStore((s) => s.viewTenant)
  const [keyword, setKeyword] = useState('')
  const [editTarget, setEditTarget] = useState<Param | null>(null)
  const [resetTarget, setResetTarget] = useState<Param | null>(null)

  // 列表不分页且通常只有几十个键：一次拉全，关键词在前端过滤
  const list = useQuery({ queryKey: paramKeys.list({}), queryFn: () => listParams() })

  const kw = keyword.trim().toLowerCase()
  const rows = useMemo(() => {
    const all = list.data ?? []
    if (!kw) return all
    return all.filter((p) => p.key.toLowerCase().includes(kw) || (p.description ?? '').toLowerCase().includes(kw) || p.value.toLowerCase().includes(kw))
  }, [list.data, kw])

  const invalidate = () => queryClient.invalidateQueries({ queryKey: paramKeys.all })

  const reset = useMutation({
    mutationFn: (key: string) => resetParam(key),
    onSuccess: () => {
      toast.success('已恢复全局缺省值')
      setResetTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('恢复失败', errorMessage(e)),
  })

  const canUpdate = can('system:param:update')
  // 超级管理员：未切换查看租户时写全局缺省，否则写目标租户的覆盖值
  const globalScope = isSuper && !viewTenant
  const scopeHint = globalScope
    ? '当前为平台视角：保存将修改全局缺省值，对所有未覆盖该参数的租户生效。'
    : viewTenant
      ? `当前正在查看租户「${viewTenant.name}」：保存将写入该租户的覆盖值。`
      : '保存将写入本租户的覆盖值，不影响其他租户；可随时恢复全局缺省。'

  const columns: Column<Param>[] = [
    { key: 'key', title: '参数键', render: (p) => <code className="font-mono text-xs text-slate-800">{p.key}</code> },
    { key: 'value', title: '当前值', render: (p) => <ValueCell p={p} value={p.value} /> },
    { key: 'value_type', title: '类型', width: 80, render: (p) => <Badge color="gray">{paramTypeLabel[p.value_type]}</Badge> },
    { key: 'source', title: '来源', width: 80, render: (p) => <Badge color={paramSourceColor[p.source]}>{paramSourceLabel[p.source]}</Badge> },
    {
      key: 'global_value',
      title: '全局缺省值',
      render: (p) => (p.source === 'tenant' ? <ValueCell p={p} value={p.global_value} /> : <span className="text-xs text-slate-400">（同当前值）</span>),
    },
    { key: 'description', title: '说明', render: (p) => <span className="text-slate-500">{text(p.description)}</span> },
    { key: 'updated_at', title: '更新时间', render: (p) => <span className="text-xs text-slate-500">{formatDateTime(p.updated_at)}</span> },
  ]

  const renderActions = (p: Param) => (
    <div className="flex items-center justify-end gap-0.5">
      <Button variant="ghost" size="sm" icon={Pencil} className="!px-2" title="修改" aria-label="修改" onClick={() => setEditTarget(p)} />
      {p.source === 'tenant' && !globalScope && (
        <Button variant="ghost" size="sm" icon={RotateCcw} className="!px-2" title="恢复全局缺省" aria-label="恢复全局缺省" onClick={() => setResetTarget(p)} />
      )}
    </div>
  )

  return (
    <div className="space-y-4 slide-up">
      <PageHeader title="参数设置" description="系统参数：租户覆盖值优先，否则采用全局缺省值" />

      {isSuper && (
        <div className="flex items-start gap-2 rounded-xl border border-purple-100 bg-purple-50 px-4 py-3 text-sm text-purple-800">
          <Globe size={16} className="mt-0.5 shrink-0" />
          <div>
            {globalScope ? (
              <>
                <span className="font-medium">当前修改的是全局缺省值。</span>
                <span className="ml-1 text-purple-700">如需为某个租户单独设置，请在顶栏切换查看租户。</span>
              </>
            ) : (
              <>
                当前正在查看租户 <span className="font-medium">{viewTenant?.name}</span>：修改将写入该租户的覆盖值。
              </>
            )}
          </div>
        </div>
      )}

      <div className="card p-4 flex flex-wrap items-center gap-2">
        <div className="w-full sm:w-72">
          <Input icon={Search} value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="按键名 / 说明 / 值过滤" aria-label="过滤参数" />
        </div>
        <span className="text-xs text-slate-400">
          共 {list.data?.length ?? 0} 项{kw ? `，匹配 ${rows.length} 项` : ''}
        </span>
      </div>

      <div className="card p-4">
        {list.isError ? (
          <ErrorState message={errorMessage(list.error)} onRetry={() => void list.refetch()} />
        ) : (
          <Table
            columns={columns}
            data={rows}
            rowKey="key"
            loading={list.isFetching}
            actions={canUpdate ? { render: renderActions, width: 90 } : undefined}
            empty={<Empty title={kw ? '没有匹配的参数' : '暂无参数'} description={kw ? '换个关键词试试' : '参数键由系统预置'} />}
          />
        )}
      </div>

      <ParamEditModal
        param={editTarget}
        scopeHint={scopeHint}
        onClose={() => setEditTarget(null)}
        onSaved={() => {
          setEditTarget(null)
          void invalidate()
        }}
      />
      <ConfirmDialog
        open={resetTarget !== null}
        title={`恢复「${resetTarget?.key ?? ''}」的全局缺省值？`}
        description={
          <>
            将删除租户覆盖值 <code className="font-mono">{resetTarget?.value}</code>，恢复为全局缺省值 <code className="font-mono">{text(resetTarget?.global_value)}</code>。
          </>
        }
        confirmText="恢复缺省"
        loading={reset.isPending}
        onConfirm={() => {
          if (resetTarget) reset.mutate(resetTarget.key)
        }}
        onCancel={() => setResetTarget(null)}
      />
    </div>
  )
}
