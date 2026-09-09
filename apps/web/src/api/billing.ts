import { compactParams, del, download, get, post, put, type DownloadedFile } from './client'
import type {
  Account,
  AccountLevel,
  AccountNode,
  AccountStatus,
  AccountTransaction,
  BillingResult,
  BillingRule,
  BillingRuleCreate,
  BillingRuleDoc,
  BillingRuleUpdate,
  BillingTripInput,
  MyAccount,
  Page,
  PageQuery,
  Settlement,
  SettlementStatus,
  TransactionType,
} from './types'

// ── 查询参数 ──────────────────────────────────────────

export interface AccountFilter {
  level?: AccountLevel | ''
  /** 部门名 / 用户名 / 姓名 */
  keyword?: string
  /** 仅余额为负 */
  negative?: boolean
}
export type AccountListParams = AccountFilter & PageQuery

export interface AccountTransactionFilter {
  type?: TransactionType | ''
  from?: string
  to?: string
}
export type AccountTransactionParams = AccountTransactionFilter & PageQuery

export interface TransactionFilter extends AccountTransactionFilter {
  level?: AccountLevel | ''
  /** 账户名 / 备注 / ref_id */
  keyword?: string
}
export type TransactionListParams = TransactionFilter & PageQuery

export interface SettlementListParams {
  /** YYYY-MM；缺省最近一期 */
  period?: string
  status?: SettlementStatus | ''
}

// ── 请求体 ────────────────────────────────────────────

export interface SimulateBody {
  /** 与 rule 二选一；都不传用当前生效规则 */
  rule_id?: string
  rule?: BillingRuleDoc
  trip: BillingTripInput
}

export interface RechargeBody {
  amount: number
  remark?: string
}

export interface AccountUpdateBody {
  credit_limit?: number
  monthly_budget?: number
  status?: AccountStatus
}

export interface AllocateBody {
  /** 与 to_level + to_owner_id 二选一 */
  to_account_id?: string
  to_level?: AccountLevel
  /** 部门 id 或用户 id */
  to_owner_id?: string
  amount: number
  remark?: string
}

export interface AllocateResult {
  from?: AccountTransaction
  to?: AccountTransaction
}

export interface AdjustBody {
  /** 非 0，有符号 */
  amount: number
  type?: 'adjust' | 'refund'
  remark: string
  ref_type?: string
  ref_id?: string
}

export interface SettlementPeriod {
  period: string
  status: 'none' | 'draft' | 'confirmed'
  total?: number
}

/** query key 统一以 ['billing'] 开头：WebSocket account.updated 到达时整体失效 */
export const billingKeys = {
  all: ['billing'] as const,
  rules: ['billing', 'rules', 'list'] as const,
  rule: (id: string) => ['billing', 'rules', 'detail', id] as const,
  ruleTemplate: ['billing', 'rules', 'template'] as const,
  accountTree: ['billing', 'accounts', 'tree'] as const,
  accounts: (params: AccountListParams) => ['billing', 'accounts', 'list', params] as const,
  account: (id: string) => ['billing', 'accounts', 'detail', id] as const,
  myAccount: ['billing', 'accounts', 'me'] as const,
  accountTransactions: (id: string, params: AccountTransactionParams) => ['billing', 'accounts', 'transactions', id, params] as const,
  transactions: (params: TransactionListParams) => ['billing', 'transactions', params] as const,
  settlements: (params: SettlementListParams) => ['billing', 'settlements', 'list', params] as const,
  settlementPeriods: ['billing', 'settlements', 'periods'] as const,
  settlement: (id: string) => ['billing', 'settlements', 'detail', id] as const,
}

// ── 计费规则 ──────────────────────────────────────────

/** 规则列表（本租户；is_default 为当前生效规则） */
export const listBillingRules = (): Promise<BillingRule[]> => get<BillingRule[]>('/billing/rules')

/** 缺省规则模板（纯电车队标准套餐） */
export const getBillingRuleTemplate = (): Promise<BillingRuleDoc> => get<BillingRuleDoc>('/billing/rules/template')

export const getBillingRule = (id: string): Promise<BillingRule> => get<BillingRule>(`/billing/rules/${id}`)

/** 新建规则；首条规则自动设为生效，activate=true 时立即生效 */
export const createBillingRule = (body: BillingRuleCreate): Promise<BillingRule> => post<BillingRule, BillingRuleCreate>('/billing/rules', body)

/** 编辑规则（rule 需通过后端校验，失败 400） */
export const updateBillingRule = (id: string, body: BillingRuleUpdate): Promise<BillingRule> =>
  put<BillingRule, BillingRuleUpdate>(`/billing/rules/${id}`, body)

/** 删除规则（生效中的规则 409） */
export const deleteBillingRule = (id: string): Promise<unknown> => del<unknown>(`/billing/rules/${id}`)

/** 设为生效规则（其余取消生效） */
export const activateBillingRule = (id: string): Promise<BillingRule> => post<BillingRule>(`/billing/rules/${id}/activate`)

/** 模拟计算（不落库） */
export const simulateBillingRule = (body: SimulateBody): Promise<BillingResult> => post<BillingResult, SimulateBody>('/billing/rules/simulate', body)

// ── 账户 ─────────────────────────────────────────────

/** 账户树：企业 → 部门 → 员工（未创建的节点 exists=false） */
export const getAccountTree = (): Promise<AccountNode> => get<AccountNode>('/billing/accounts/tree')

export const listAccounts = (params: AccountListParams): Promise<Page<Account>> =>
  get<Page<Account>>('/billing/accounts', { params: compactParams(params) })

/** 我的员工账户（不存在则返回余额 0 的虚拟账户）与最近 20 条流水 */
export const getMyAccount = (): Promise<MyAccount> => get<MyAccount>('/billing/accounts/me')

export const getAccount = (id: string): Promise<Account> => get<Account>(`/billing/accounts/${id}`)

/** 企业账户充值（线下到账登记） */
export const rechargeEnterprise = (body: RechargeBody): Promise<AccountTransaction> =>
  post<AccountTransaction, RechargeBody>('/billing/accounts/recharge', body)

/** 编辑账户：透支额度、月度预算、冻结/解冻 */
export const updateAccount = (id: string, body: AccountUpdateBody): Promise<Account> =>
  put<Account, AccountUpdateBody>(`/billing/accounts/${id}`, body)

/** 向下级账户划拨（目标不存在则自动创建；余额不足 409） */
export const allocateAccount = (id: string, body: AllocateBody): Promise<AllocateResult> =>
  post<AllocateResult, AllocateBody>(`/billing/accounts/${id}/allocate`, body)

/** 余额调整（正数入账 / 退款，负数扣减） */
export const adjustAccount = (id: string, body: AdjustBody): Promise<AccountTransaction> =>
  post<AccountTransaction, AdjustBody>(`/billing/accounts/${id}/adjust`, body)

export const listAccountTransactions = (id: string, params: AccountTransactionParams): Promise<Page<AccountTransaction>> =>
  get<Page<AccountTransaction>>(`/billing/accounts/${id}/transactions`, { params: compactParams(params) })

/** 租户全部流水 */
export const listTransactions = (params: TransactionListParams): Promise<Page<AccountTransaction>> =>
  get<Page<AccountTransaction>>('/billing/transactions', { params: compactParams(params) })

// ── 月度结算 ──────────────────────────────────────────

/** 某期结算单：一条企业汇总（dept_id 为 null）+ 每部门一条 */
export const listSettlements = (params: SettlementListParams): Promise<Settlement[]> =>
  get<Settlement[]>('/billing/settlements', { params: compactParams(params) })

/** 可结算的期（倒序）及各期状态 */
export const listSettlementPeriods = (): Promise<SettlementPeriod[]> => get<SettlementPeriod[]>('/billing/settlements/periods')

/** 生成 / 重算某期（已确认 409） */
export const generateSettlement = (period: string): Promise<Settlement[]> =>
  post<Settlement[]>('/billing/settlements/generate', { period }, { timeout: 60_000 })

/** 结算单详情（含明细行） */
export const getSettlement = (id: string): Promise<Settlement> => get<Settlement>(`/billing/settlements/${id}`)

/** 确认结算单（企业汇总行确认即全部确认） */
export const confirmSettlement = (id: string): Promise<Settlement> => post<Settlement>(`/billing/settlements/${id}/confirm`)

/** 导出某期结算单 xlsx；dept_id 只导出某部门 */
export const exportSettlements = (period: string, dept_id?: string): Promise<DownloadedFile> =>
  download('/billing/settlements/export', `结算单-${period}.xlsx`, { params: compactParams({ period, dept_id }) })
