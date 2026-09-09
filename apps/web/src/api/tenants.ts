import { compactParams, del, get, post, put } from './client'
import type { Page, PageQuery, Tenant, TenantCreate, TenantStatus, TenantUpdate } from './types'

export interface TenantFilter {
  keyword?: string
  status?: TenantStatus | ''
}

export type TenantListParams = TenantFilter & PageQuery

export const tenantKeys = {
  all: ['tenants'] as const,
  list: (params: TenantListParams) => ['tenants', 'list', params] as const,
  /** 顶栏"切换查看租户"下拉 */
  options: ['tenants', 'options'] as const,
  detail: (id: string) => ['tenants', 'detail', id] as const,
}

/**
 * 租户接口是平台级的，与当前"查看租户"无关，统一不带 X-Tenant-ID，
 * 避免切换查看租户后列表 / 编辑受影响。
 */
const PLATFORM = { skipTenantHeader: true } as const

/** GET /system/tenants —— 仅平台租户可见（system:tenant:view 只发给平台租户） */
export const listTenants = (params: TenantListParams): Promise<Page<Tenant>> =>
  get<Page<Tenant>>('/system/tenants', { ...PLATFORM, params: compactParams(params) })

/** 顶栏下拉：一次拉全（契约 pageSize 上限 200） */
export const listTenantOptions = (): Promise<Tenant[]> =>
  listTenants({ pageSize: 200, sort: 'code' }).then((p) => p.items)

/** 含用户数 / 部门数 */
export const getTenant = (id: string): Promise<Tenant> => get<Tenant>(`/system/tenants/${id}`, PLATFORM)

/** 自动创建内置角色与租户管理员账号 */
export const createTenant = (body: TenantCreate): Promise<Tenant> => post<Tenant, TenantCreate>('/system/tenants', body, PLATFORM)

/** 只更新出现的字段；status→disabled 时该租户所有会话失效；平台租户不可停用（400） */
export const updateTenant = (id: string, body: TenantUpdate): Promise<Tenant> =>
  put<Tenant, TenantUpdate>(`/system/tenants/${id}`, body, PLATFORM)

/** 停用 / 启用（PUT status） */
export const setTenantStatus = (id: string, status: TenantStatus): Promise<Tenant> => updateTenant(id, { status })

/** DELETE /system/tenants/{id} —— 等价于置为 disabled，不物理删除 */
export const disableTenant = (id: string): Promise<unknown> => del<unknown>(`/system/tenants/${id}`, PLATFORM)
