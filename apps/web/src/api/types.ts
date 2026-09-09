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

// ── 车辆 ─────────────────────────────────────────────
export type VehicleStatus = Schemas['VehicleStatusEnum']
export type Vehicle = Schemas['Vehicle']
export type VehicleBrief = Schemas['VehicleBrief']
export type VehicleLive = Schemas['VehicleLive']
export type VehicleCreate = Schemas['VehicleCreate']
export type VehicleUpdate = Schemas['VehicleUpdate']
/** 手动可切换的车辆状态（VehicleUpdate.status） */
export type VehicleManualStatus = NonNullable<VehicleUpdate['status']>
export type TelemetryPoint = Schemas['TelemetryPoint']

// ── 网关设备 ──────────────────────────────────────────
export type Device = Schemas['Device']
export type DeviceStatus = Device['status']
export type DeviceCreate = Schemas['DeviceCreate']
export type DeviceUpdate = Schemas['DeviceUpdate']
/** 新建 / 换密钥返回：设备 + 一次性 api_key */
export type DeviceWithKey = Device & { api_key: string }

// ── NFC 卡 ───────────────────────────────────────────
export type Card = Schemas['Card']
export type CardStatus = Card['status']
export type CardCreate = Schemas['CardCreate']
export type CardUpdate = Schemas['CardUpdate']

// ── 充电桩 ────────────────────────────────────────────
export type Pile = Schemas['Pile']
export type PileStatus = Schemas['PileStatusEnum']
export type PileType = Pile['type']
export type PileCreate = Schemas['PileCreate']
export type PileUpdate = Schemas['PileUpdate']
/** 手动可设置的桩状态（PileUpdate.status） */
export type PileManualStatus = NonNullable<PileUpdate['status']>

// ── 公务审批 ──────────────────────────────────────────
export type UserBrief = Schemas['UserBrief']
export type TripType = Schemas['TripType']
export type ApprovalStatus = Schemas['ApprovalStatus']
export type ApprovalStep = Schemas['ApprovalStep']
export type ApprovalStepAction = ApprovalStep['action']
export type Approval = Schemas['Approval']
export type ApprovalUrgency = Approval['urgency']
export type ApprovalAttachment = NonNullable<Approval['attachments']>[number]
export type ApprovalCreate = Schemas['ApprovalCreate']
export type PrecheckResult = Schemas['PrecheckResult']
export type PrecheckConflict = PrecheckResult['conflicts'][number]
export type ApprovalRules = Schemas['ApprovalRules']
export type ApprovalRulesUpdate = Schemas['ApprovalRulesUpdate']

// ── 行程（轨迹 / 事件，供地图组件与总览复用）────────────
export type PlannedRoute = Schemas['PlannedRoute']
export type TripTrack = Schemas['TripTrack']
export type TrackPoint = TripTrack['points'][number]
export type TripEvent = Schemas['TripEvent']
export type TripEventType = TripEvent['type']
export type TripSummary = Schemas['TripSummary']
export type TripStatus = Schemas['TripStatus']
export type TripBrief = Schemas['TripBrief']
export type Trip = Schemas['Trip']
export type TripSource = Trip['source']
export type RoofSignStatus = Trip['roof_sign_status']
/** 行程详情里的关联申请摘要 */
export type TripApprovalBrief = NonNullable<Trip['approval']>

// ── 总览 ─────────────────────────────────────────────
export type DashboardOverview = Schemas['DashboardOverview']
export type DashboardEvent = DashboardOverview['recent_events'][number]

// ── 通知 ─────────────────────────────────────────────
export type Notification = Schemas['Notification']

// ── 充电（阶段 3）───────────────────────────────────────
export type ChargeStatus = Schemas['ChargeStatus']
export type ConnectorStatus = Schemas['ConnectorStatus']
export type PileLive = Schemas['PileLive']
export type PileLiveConnector = PileLive['connectors'][number]
export type ChargeTransaction = Schemas['ChargeTransaction']
export type ChargeReviewStatus = ChargeTransaction['review_status']
export type ChargeBindMethod = NonNullable<ChargeTransaction['bind_method']>
export type MeterValue = Schemas['MeterValue']
export type ChargingSummary = Schemas['ChargingSummary']
export type ChargingPileStat = NonNullable<ChargingSummary['by_pile']>[number]
export type AccountLevel = Schemas['AccountLevel']

// ── 计费明细（trips.cost_detail：billing/engine.Result 的 JSON）──
export type CostLineKind = 'base' | 'multiplier' | 'cap' | 'surcharge' | 'electricity' | 'penalty'
export interface CostLine {
  /** 里程费 / 时长费 / 电费 / 时段系数 / 低电附加 / 超速罚金 ... */
  item: string
  kind: CostLineKind
  /** km / h / kWh / 次 */
  qty: number
  unit: string
  /** 元/单位（系数行为 factor） */
  unit_price: number
  /** 元；罚金为正数单列，减免为负 */
  amount: number
  note?: string
}
export interface CostDetail {
  rule_name: string
  lines: CostLine[]
  /** 里程费 + 时长费（未乘系数） */
  base: number
  /** 命中的时段系数（无则 1） */
  multiplier: number
  cap_applied: boolean
  surcharge: number
  electricity: number
  penalty: number
  total: number
  /** 扣费账户级别 */
  attribution: AccountLevel | string
}
/** 契约暂未收录、后端 trips 表已有的计费字段（migration 00004），返回时展示 */
export interface TripBillingExt {
  billing_status?: 'pending' | 'charged' | 'skipped' | 'failed' | string | null
  billing_error?: string | null
  account_id?: string | null
  account_txn_id?: number | null
  billed_at?: string | null
}
