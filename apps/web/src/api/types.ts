// 从 openapi.yaml 生成的 schema.d.ts（`npm run gen:api`）中导出常用别名。
// 业务代码只从这里引用契约类型，避免到处写 components['schemas'][...]。
import type { components } from './schema'

type Schemas = components['schemas']

// ── 信封与分页 ────────────────────────────────────────
export type Envelope = Schemas['Envelope']
/** 契约中 Page.items 为 unknown[]，这里按实际元素类型收窄 */
export type Page<T> = Omit<Schemas['Page'], 'items'> & { items: T[] }

/** 列表接口统一查询参数 */
export interface PageQuery {
  page?: number
  pageSize?: number
  /** 排序字段，`-field` 表示降序 */
  sort?: string
}

// ── 探针 ─────────────────────────────────────────────
export type Health = Schemas['Health']
export type Ready = Schemas['Ready']

// ── 认证 ─────────────────────────────────────────────
export type LoginRequest = Schemas['LoginRequest']
export type TokenPair = Schemas['TokenPair']
export type LoginResponse = Schemas['LoginResponse']
export type Profile = Schemas['Profile']
export type MenuNode = Schemas['MenuNode']
export type TenantBrief = Schemas['TenantBrief']
export type RoleBrief = Schemas['RoleBrief']

// ── 用户 ─────────────────────────────────────────────
export type User = Schemas['User']
export type UserStatus = Schemas['UserStatus']
export type UserCreate = Schemas['UserCreate']
export type UserUpdate = Schemas['UserUpdate']
export type ImportResult = Schemas['ImportResult']
export type ImportError = ImportResult['errors'][number]

// ── 部门 ─────────────────────────────────────────────
export type Dept = Schemas['Dept']
export type DeptNode = Schemas['DeptNode']
export type DeptCreate = Schemas['DeptCreate']
export type DeptUpdate = Schemas['DeptUpdate']
export type DeptStatus = Dept['status']

// ── 角色与权限 ────────────────────────────────────────
export type Role = Schemas['Role']
export type RoleCreate = Schemas['RoleCreate']
export type RoleUpdate = Schemas['RoleUpdate']
export type PermissionNode = Schemas['PermissionNode']
export type PermissionNodeType = PermissionNode['type']

// ── 租户 ─────────────────────────────────────────────
export type Tenant = Schemas['Tenant']
export type TenantStatus = Tenant['status']
export type TenantCreate = Schemas['TenantCreate']
export type TenantUpdate = Schemas['TenantUpdate']

// ── 字典 ─────────────────────────────────────────────
export type DictType = Schemas['DictType']
export type DictTypeCreate = Schemas['DictTypeCreate']
export type DictTypeUpdate = Schemas['DictTypeUpdate']
export type DictItem = Schemas['DictItem']
export type DictItemStatus = DictItem['status']
export type DictItemCreate = Schemas['DictItemCreate']
export type DictItemUpdate = Schemas['DictItemUpdate']

// ── 系统参数 ──────────────────────────────────────────
export type Param = Schemas['Param']
export type ParamValueType = Param['value_type']
export type ParamSource = Param['source']
export type ParamUpdate = Schemas['ParamUpdate']

// ── 审计日志 ──────────────────────────────────────────
export type AuditLog = Schemas['AuditLog']

// ── 通知模板 ──────────────────────────────────────────
export type NotifyChannel = Schemas['NotifyChannel']
export type Template = Schemas['Template']
export type TemplateCreate = Schemas['TemplateCreate']
export type TemplateUpdate = Schemas['TemplateUpdate']
