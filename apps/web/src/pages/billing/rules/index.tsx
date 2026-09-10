import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { BadgeCheck, Calculator, Copy, FilePlus2, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { activateBillingRule, billingKeys, deleteBillingRule, getBillingRuleTemplate, listBillingRules } from '../../../api/billing'
import { errorMessage } from '../../../api/client'
import type { BillingRule } from '../../../api/types'
import Badge from '../../../components/ui/Badge'
import Button from '../../../components/ui/Button'
import ConfirmDialog from '../../../components/ui/ConfirmDialog'
import Empty from '../../../components/ui/Empty'
import ErrorState from '../../../components/ui/ErrorState'
import PageHeader from '../../../components/ui/PageHeader'
import Spinner from '../../../components/ui/Spinner'
import { useToast } from '../../../components/ui/toast-context'
import { usePermission } from '../../../hooks/usePermission'
import { formatDate, formatDateTime } from '../../../utils/format'
import RuleEditor, { type EditorState } from './RuleEditor'
import { DEFAULT_RULE_DOC } from './schemas'

type Selection = { kind: 'rule'; id: string } | { kind: 'create'; state: Extract<EditorState, { mode: 'create' }> } | null

function effectiveText(r: BillingRule): string {
  if (!r.effective_from && !r.effective_to) return '长期有效'
  return `${r.effective_from ? formatDate(r.effective_from) : '…'} ～ ${r.effective_to ? formatDate(r.effective_to) : '…'}`
}

/** 计费规则（/billing/rules）：左侧规则列表，右侧分区表单编辑器 + JSON 视图 + 模拟计算 */
export default function BillingRulesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const canUpdate = can('billing:rule:update')

  const rules = useQuery({ queryKey: billingKeys.rules, queryFn: listBillingRules })
  const list = useMemo(() => rules.data ?? [], [rules.data])

  // null = 未手动选择：缺省选中生效规则（无则第一条）
  const [chosen, setChosen] = useState<Selection>(null)
  const [activateTarget, setActivateTarget] = useState<BillingRule | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<BillingRule | null>(null)
  const [templateLoading, setTemplateLoading] = useState(false)

  const defaultSelection: Selection = list.length > 0 ? { kind: 'rule', id: (list.find((r) => r.is_default) ?? list[0]).id } : null
  const selection: Selection = chosen ?? defaultSelection
  const setSelection = (s: Selection) => setChosen(s)

  const selectedRule = selection?.kind === 'rule' ? list.find((r) => r.id === selection.id) : undefined
  const invalidate = () => queryClient.invalidateQueries({ queryKey: billingKeys.all })

  const activate = useMutation({
    mutationFn: (id: string) => activateBillingRule(id),
    onSuccess: (saved) => {
      toast.success(`「${saved.name}」已设为生效规则`, '对之后结束的行程 / 充电生效')
      setActivateTarget(null)
      void invalidate()
    },
    onError: (e) => toast.error('设置失败', errorMessage(e)),
  })

  const remove = useMutation({
    mutationFn: (id: string) => deleteBillingRule(id),
    onSuccess: (_, id) => {
      toast.success('规则已删除')
      setDeleteTarget(null)
      if (selection?.kind === 'rule' && selection.id === id) setSelection(null)
      void invalidate()
    },
    // 409：生效中的规则不能删除
    onError: (e) => toast.error('删除失败', errorMessage(e)),
  })

  /** 从模板新建：接口不可用时回退到内置缺省模板 */
  const createFromTemplate = async () => {
    setTemplateLoading(true)
    try {
      const doc = await queryClient.fetchQuery({ queryKey: billingKeys.ruleTemplate, queryFn: getBillingRuleTemplate, staleTime: 5 * 60_000 })
      setSelection({ kind: 'create', state: { mode: 'create', doc, name: doc.rule_name, source: '来自模板' } })
    } catch (e) {
      toast.warning('模板接口暂不可用，已使用内置缺省模板', errorMessage(e))
      setSelection({ kind: 'create', state: { mode: 'create', doc: DEFAULT_RULE_DOC, name: DEFAULT_RULE_DOC.rule_name, source: '内置模板' } })
    } finally {
      setTemplateLoading(false)
    }
  }

  const copyRule = (r: BillingRule) => {
    setSelection({ kind: 'create', state: { mode: 'create', doc: structuredClone(r.rule), name: `${r.name}（副本）`, source: `复制自 ${r.name}` } })
  }

  const editorState: EditorState | null = selection?.kind === 'create' ? selection.state : selectedRule ? { mode: 'edit', rule: selectedRule } : null

  return (
    <div className="space-y-4 slide-up">
      <PageHeader
        title="计费规则"
        description="可编程计费引擎：基础费率、纯电专项、时段系数、罚金与费用归属；同一时刻只有一条规则生效"
        extra={
          canUpdate && (
            <>
              {selectedRule && (
                <Button variant="secondary" icon={Copy} onClick={() => copyRule(selectedRule)}>
                  复制当前规则
                </Button>
              )}
              <Button icon={FilePlus2} loading={templateLoading} onClick={() => void createFromTemplate()}>
                从模板新建
              </Button>
            </>
          )
        }
      />

      <div className="grid gap-4 lg:grid-cols-[320px_minmax(0,1fr)] items-start">
        {/* ── 左：规则列表 ── */}
        <div className="card p-3">
          <div className="flex items-center justify-between px-1 pb-2 border-b border-line">
            <div className="flex items-center gap-1.5 text-sm font-medium text-ink">
              <Calculator size={15} className="text-ink-faint" />
              规则列表
            </div>
            <span className="text-xs text-ink-faint">{list.length > 0 ? `${list.length} 条` : ''}</span>
          </div>
          <div className="pt-2 max-h-[70vh] overflow-y-auto">
            {rules.isPending ? (
              <div className="flex justify-center py-10">
                <Spinner label="加载中" />
              </div>
            ) : rules.isError ? (
              <ErrorState size="sm" message={errorMessage(rules.error)} onRetry={() => void rules.refetch()} />
            ) : list.length === 0 ? (
              <Empty
                size="sm"
                title="暂无计费规则"
                description="从模板新建一条规则，首条规则自动生效"
                action={
                  canUpdate && (
                    <Button size="sm" variant="secondary" icon={FilePlus2} loading={templateLoading} onClick={() => void createFromTemplate()}>
                      从模板新建
                    </Button>
                  )
                }
              />
            ) : (
              <ul className="space-y-1">
                {list.map((r) => {
                  const active = selection?.kind === 'rule' && selection.id === r.id
                  return (
                    <li key={r.id}>
                      <div
                        role="button"
                        tabIndex={0}
                        onClick={() => setSelection({ kind: 'rule', id: r.id })}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.preventDefault()
                            setSelection({ kind: 'rule', id: r.id })
                          }
                        }}
                        className={clsx('group cursor-pointer rounded-lg px-3 py-2.5 transition-colors', active ? 'bg-brand-600/10' : 'hover:bg-surface-3')}
                      >
                        <div className="flex items-center gap-2">
                          <span className={clsx('min-w-0 flex-1 truncate text-sm font-medium', active ? 'text-brand-300' : 'text-ink-strong')}>{r.name}</span>
                          {r.is_default && <Badge color="green">生效中</Badge>}
                          {!r.enabled && <Badge color="gray">停用</Badge>}
                        </div>
                        <div className="mt-1 flex items-center justify-between gap-2 text-[11px] text-ink-faint">
                          <span className="truncate">
                            {effectiveText(r)} · 更新 {formatDateTime(r.updated_at)}
                          </span>
                        </div>
                        {canUpdate && (
                          <div className="mt-1.5 flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                            {!r.is_default && (
                              <Button variant="ghost" size="sm" icon={BadgeCheck} className="!h-7 !px-2 text-ev-200 hover:bg-ev-500/10" disabled={!r.enabled} title={r.enabled ? '设为生效' : '停用的规则不能设为生效'} onClick={() => setActivateTarget(r)}>
                                设为生效
                              </Button>
                            )}
                            <Button variant="ghost" size="sm" icon={Copy} className="!h-7 !px-2" onClick={() => copyRule(r)}>
                              复制
                            </Button>
                            {!r.is_default && (
                              <Button variant="ghost" size="sm" icon={Trash2} className="!h-7 !px-2 text-danger-200 hover:bg-danger-500/10 hover:text-danger-200" onClick={() => setDeleteTarget(r)}>
                                删除
                              </Button>
                            )}
                          </div>
                        )}
                      </div>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        </div>

        {/* ── 右：编辑器 ── */}
        <div className="card p-5 min-h-[24rem]">
          {editorState ? (
            <RuleEditor
              state={editorState}
              readOnly={!canUpdate}
              onCancelCreate={() => setSelection(null)}
              onSaved={(saved) => {
                queryClient.setQueryData<BillingRule[]>(billingKeys.rules, (old) => {
                  if (!old) return old
                  return old.some((r) => r.id === saved.id) ? old.map((r) => (r.id === saved.id ? saved : r)) : [...old, saved]
                })
                setSelection({ kind: 'rule', id: saved.id })
                void invalidate()
              }}
            />
          ) : (
            <Empty
              icon={Calculator}
              title={rules.isError ? '规则列表暂不可用' : '选择左侧规则'}
              description={rules.isError ? '仍可从内置模板新建规则并做表单校验与模拟计算' : '查看、编辑规则，或从模板 / 现有规则新建'}
              action={
                canUpdate && (
                  <Button variant="secondary" icon={FilePlus2} loading={templateLoading} onClick={() => void createFromTemplate()}>
                    从模板新建
                  </Button>
                )
              }
            />
          )}
        </div>
      </div>

      <ConfirmDialog
        open={activateTarget !== null}
        title={`将「${activateTarget?.name ?? ''}」设为生效规则？`}
        description="其余规则将取消生效；对之后结束的行程与充电事务生效，已结算的不受影响。"
        confirmText="设为生效"
        loading={activate.isPending}
        onConfirm={() => {
          if (activateTarget) activate.mutate(activateTarget.id)
        }}
        onCancel={() => setActivateTarget(null)}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        danger
        title={`删除规则「${deleteTarget?.name ?? ''}」？`}
        description="生效中的规则无法删除；删除后不可恢复。"
        confirmText="删除"
        loading={remove.isPending}
        onConfirm={() => {
          if (deleteTarget) remove.mutate(deleteTarget.id)
        }}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
