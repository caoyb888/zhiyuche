import { compactParams, get, post, put } from './client'
import type { Approval, ApprovalCreate, ApprovalRules, ApprovalRulesUpdate, ApprovalStatus, Page, PageQuery, PrecheckResult, TripType, VehicleBrief } from './types'

/** 列表视角：我发起的 / 待我审批（approval:approve）/ 全部（approval:manage） */
export type ApprovalScope = 'mine' | 'todo' | 'all'

export const APPROVAL_SCOPES: readonly ApprovalScope[] = ['mine', 'todo', 'all']

export function isApprovalScope(v: string | null | undefined): v is ApprovalScope {
  return v === 'mine' || v === 'todo' || v === 'all'
}

/** GET /approvals 的筛选条件（不含分页） */
export interface ApprovalFilter {
  scope?: ApprovalScope
  status?: ApprovalStatus | ''
  trip_type?: TripType | ''
  /** 申请人部门（含子部门） */
  dept_id?: string
  vehicle_id?: string
  applicant_id?: string
  /** planned_start ≥ */
  from?: string
  /** planned_start ≤ */
  to?: string
  /** 单号 / 事由 / 目的地 / 申请人 */
  keyword?: string
}

export type ApprovalListParams = ApprovalFilter & PageQuery

export interface AvailableVehiclesQuery {
  start: string
  end: string
  /** 编辑 / 审批指派时排除自身 */
  exclude_approval_id?: string
}

export interface ApproveBody {
  remark?: string
  /** 申请未指定车辆时必填；approval:manage 可改派 */
  vehicle_id?: string
}

export interface RejectBody {
  reason: string
}

export interface CancelBody {
  reason?: string
}

/** query key 统一以 ['approvals'] 开头：WebSocket approval.updated 事件到达时整体失效 */
export const approvalKeys = {
  all: ['approvals'] as const,
  list: (params: ApprovalListParams) => ['approvals', 'list', params] as const,
  detail: (id: string) => ['approvals', 'detail', id] as const,
  todoCount: ['approvals', 'todo-count'] as const,
  availableVehicles: (q: AvailableVehiclesQuery) => ['approvals', 'available-vehicles', q] as const,
  rules: ['approvals', 'rules'] as const,
}

export const listApprovals = (params: ApprovalListParams): Promise<Page<Approval>> =>
  get<Page<Approval>>('/approvals', { params: compactParams(params) })

/** 详情：含审批步骤、车辆、申请人、关联行程摘要；可见范围外返回 403 */
export const getApproval = (id: string): Promise<Approval> => get<Approval>(`/approvals/${id}`)

export const createApproval = (body: ApprovalCreate): Promise<Approval> => post<Approval, ApprovalCreate>('/approvals', body)

/** 预检（不落库）；即使不通过也返回 200，看 ok / problems */
export const precheckApproval = (body: ApprovalCreate): Promise<PrecheckResult> => post<PrecheckResult, ApprovalCreate>('/approvals/precheck', body)

export const getApprovalTodoCount = async (): Promise<number> => {
  const res = await get<{ todo?: number }>('/approvals/todo-count')
  return typeof res?.todo === 'number' ? res.todo : 0
}

/** 时段内空闲且无冲突申请的车辆（含 SOC、续航） */
export const listAvailableVehicles = (q: AvailableVehiclesQuery): Promise<VehicleBrief[]> =>
  get<VehicleBrief[]>('/approvals/available-vehicles', { params: compactParams(q) })

export const getApprovalRules = (): Promise<ApprovalRules> => get<ApprovalRules>('/approvals/rules')

export const setApprovalRules = (body: ApprovalRulesUpdate): Promise<ApprovalRules> => put<ApprovalRules, ApprovalRulesUpdate>('/approvals/rules', body)

/** 通过当前步骤；一级通过且需二级 → pending_l2，否则 → approved */
export const approveApproval = (id: string, body: ApproveBody = {}): Promise<Approval> => post<Approval, ApproveBody>(`/approvals/${id}/approve`, body)

/** 驳回：任一级驳回即终止 */
export const rejectApproval = (id: string, body: RejectBody): Promise<Approval> => post<Approval, RejectBody>(`/approvals/${id}/reject`, body)

/** 撤销：申请人本人（pending / approved 且未开始行程）或 approval:manage */
export const cancelApproval = (id: string, body: CancelBody = {}): Promise<Approval> => post<Approval, CancelBody>(`/approvals/${id}/cancel`, body)
