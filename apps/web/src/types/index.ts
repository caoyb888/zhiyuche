// ── 通用 ─────────────────────────────────────────────
/** 经纬度坐标 [lng, lat]（高德/WGS84 均为经度在前） */
export type LngLat = [lng: number, lat: number]

/** index.css 中定义的状态徽标样式类 */
export type BadgeClass =
  | 'badge-green'
  | 'badge-blue'
  | 'badge-amber'
  | 'badge-red'
  | 'badge-gray'
  | 'badge-purple'

export type Department = '销售部' | '工程部' | '行政部' | '财务部'

// ── 车辆 ─────────────────────────────────────────────
export type VehicleStatus = '在途' | '空闲' | '充电中' | '维保中' | '预约中'

/** 充电/电量状态摘要 */
export type ChargeState = '正常' | '慢充' | '快充' | '注意' | '低电警告'

export interface Vehicle {
  id: string
  plate: string
  model: string
  status: VehicleStatus
  /** 当前驾驶员，无人时为 '—' */
  driver: string
  /** 电量百分比 0-100 */
  soc: number
  /** 电池健康度百分比 0-100 */
  soh: number
  /** 总里程 km */
  odometer: number
  charge: ChargeState
  location: LngLat
  /** 所属/当前用车部门，空闲时为 '—' */
  dept: string
  /** 当前用车事由，空闲时为 '—' */
  trip: string
}

// ── 审批 ─────────────────────────────────────────────
export type ApprovalStatus = '待审批' | '已通过' | '已驳回'
export type ApprovalUrgency = '普通' | '紧急'
export type ApprovalPurpose = '公务出行' | '客户拜访' | '接送领导' | '物资采购' | '业务出行'

export interface Approval {
  id: string
  applicant: string
  dept: Department
  purpose: ApprovalPurpose
  detail: string
  /** 指派车辆车牌 */
  vehicle: string
  startTime: string
  endTime: string
  dest: string
  route: string
  /** 预计里程 km */
  distance: number
  status: ApprovalStatus
  submitTime: string
  approver: string
  /** 审批层级 */
  level: 1 | 2
  urgency: ApprovalUrgency
  rejectReason?: string
}

// ── 行程 ─────────────────────────────────────────────
export type TripStatus = '已完成' | '进行中' | '已取消'

export interface Trip {
  id: string
  plate: string
  driver: string
  dept: Department
  purpose: string
  /** HH:mm */
  startTime: string
  /** HH:mm */
  endTime: string
  /** YYYY-MM-DD */
  date: string
  /** km */
  distance: number
  /** 可读时长文本，如 "2小时43分" */
  duration: string
  /** 耗电 kWh */
  energy: number
  /** 费用 元 */
  cost: number
  avgSpeed: number
  maxSpeed: number
  /** 急加速次数 */
  accel: number
  /** 急刹车次数 */
  brake: number
  /** 是否亮起公务灯牌 */
  sign: boolean
  status: TripStatus
  route: string
  routePoints: LngLat[]
}

// ── 报表 / 图表 ──────────────────────────────────────
/** 月度各部门费用（Recharts dataKey 直接使用部门名） */
export type MonthlyTrendPoint = { month: string } & Record<Department, number>

export interface DeptPieSlice {
  name: Department
  value: number
  color: string
}

export interface DailyUsagePoint {
  day: string
  trips: number
  km: number
}

export interface DeptStat {
  dept: Department
  trips: number
  km: number
  cost: number
  energy: number
  /** 环比变化百分比，正数为增加 */
  change: number
}

export interface TopUser {
  name: string
  dept: Department
  trips: number
  km: number
  cost: number
}

// ── 车辆健康 ─────────────────────────────────────────
export type BatteryGrade = '优秀' | '良好' | '关注' | '预警'
export type ScoreLevel = '优秀' | '良好' | '较差'

export interface HealthItem {
  id: string
  plate: string
  soh: number
  /** 综合评分 0-100 */
  score: number
  battery: BatteryGrade
  brake: number
  accel: number
  /** 百公里耗电 kWh */
  energy: number
  issues: string[]
  /** YYYY-MM-DD */
  nextMaint: string
  lastCharge: string
}

export type AiItemStatus = 'good' | 'warn' | 'bad'

export interface AiReportItem {
  icon: string
  label: string
  value: string
  status: AiItemStatus
  tip: string
}

export interface AiReport {
  plate: string
  score: number
  level: ScoreLevel
  summary: string
  items: AiReportItem[]
}

// ── 充电 ─────────────────────────────────────────────
export type ChargerStatus = '使用中' | '空闲' | '故障'

export interface Charger {
  id: string
  name: string
  /** 如 "快充 60kW" */
  type: string
  status: ChargerStatus
  /** 正在充电的车牌，空闲时为 '—' */
  vehicle: string
  /** 当前车辆电量，未接入车辆时为 null */
  soc: number | null
  startTime: string
  /** 本次已充电量 kWh */
  energy: number
  /** 本次费用 元 */
  cost: number
}

export interface ChargeRecord {
  date: string
  plate: string
  driver: string
  pile: string
  kwh: number
  cost: number
  duration: string
  dept: string
}

// ── 告警 ─────────────────────────────────────────────
export type AlertLevel = 'red' | 'amber'

export interface Alert {
  level: AlertLevel
  msg: string
  time: string
}
