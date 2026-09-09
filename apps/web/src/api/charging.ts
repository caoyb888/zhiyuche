import { compactParams, download, get, post, type DownloadedFile } from './client'
import type { ChargeReviewStatus, ChargeStatus, ChargeTransaction, ChargingSummary, MeterValue, Page, PageQuery, PileLive } from './types'

/** GET /charging/transactions 的筛选条件（不含分页） */
export interface ChargeTxFilter {
  status?: ChargeStatus | ''
  review_status?: ChargeReviewStatus | ''
  pile_id?: string
  vehicle_id?: string
  user_id?: string
  /** 用户部门（含子部门） */
  dept_id?: string
  /** start_at ≥ */
  from?: string
  /** start_at ≤ */
  to?: string
  /** 事务号 / 桩 / 车牌 / 用户 */
  keyword?: string
}

export type ChargeTxListParams = ChargeTxFilter & PageQuery

export interface RemoteStartBody {
  /** 缺省 1 */
  connector_id?: number
  /** 缺省当前用户的第一张有效卡 */
  id_tag?: string
  /** 预先指定归属车辆 */
  vehicle_id?: string
}

export interface RemoteStopBody {
  transaction_id: string
}

/** 桩对远程指令的应答 */
export interface RemoteCommandResult {
  status?: 'Accepted' | 'Rejected'
}

export type ReviewAction = 'approve' | 'reject'

export interface ReviewBody {
  action: ReviewAction
  note?: string
  /** approve 时可修正归属 */
  vehicle_id?: string
  user_id?: string
}

/** query key 统一以 ['charging'] 开头：WebSocket charging.updated 到达时整体失效 */
export const chargingKeys = {
  all: ['charging'] as const,
  /** 桩实时视图（30s 轮询 + WS 失效） */
  pilesLive: ['charging', 'piles', 'live'] as const,
  txList: (params: ChargeTxListParams) => ['charging', 'transactions', 'list', params] as const,
  txDetail: (id: string) => ['charging', 'transactions', 'detail', id] as const,
  meterValues: (id: string) => ['charging', 'transactions', 'meter-values', id] as const,
  summary: (date?: string) => ['charging', 'summary', date ?? 'today'] as const,
}

/** 桩 + 各连接器状态 + 进行中的事务（含实时 kWh / 功率） */
export const listPilesLive = (): Promise<PileLive[]> => get<PileLive[]>('/charging/piles/live')

/** 远程启动充电：桩须在线且连接器可用；409 表示桩离线 / 连接器占用 */
export const remoteStartCharging = (pileId: string, body: RemoteStartBody): Promise<RemoteCommandResult> =>
  post<RemoteCommandResult, RemoteStartBody>(`/charging/piles/${pileId}/remote-start`, body, { timeout: 30_000 })

/** 远程停止充电 */
export const remoteStopCharging = (pileId: string, transactionId: string): Promise<RemoteCommandResult> =>
  post<RemoteCommandResult, RemoteStopBody>(`/charging/piles/${pileId}/remote-stop`, { transaction_id: transactionId }, { timeout: 30_000 })

export const listChargeTransactions = (params: ChargeTxListParams): Promise<Page<ChargeTransaction>> =>
  get<Page<ChargeTransaction>>('/charging/transactions', { params: compactParams(params) })

/** 详情：含电表曲线抽样（≤ 200 点）、归属信息、复核信息 */
export const getChargeTransaction = (id: string): Promise<ChargeTransaction> => get<ChargeTransaction>(`/charging/transactions/${id}`)

/** 电表曲线（全部点，ts 升序） */
export const getChargeMeterValues = (id: string): Promise<MeterValue[]> =>
  get<MeterValue[]>(`/charging/transactions/${id}/meter-values`, { timeout: 30_000 })

/** 复核：approve 确认归属并按当前规则计价扣费（status→settled）；reject 不计费 */
export const reviewChargeTransaction = (id: string, body: ReviewBody): Promise<ChargeTransaction> =>
  post<ChargeTransaction, ReviewBody>(`/charging/transactions/${id}/review`, body)

/** 汇总：当日 / 当月次数、电量、费用；进行中 / 待复核数；桩在线 / 充电中 / 故障；本月按桩统计。date 缺省今天 */
export const getChargingSummary = (date?: string): Promise<ChargingSummary> =>
  get<ChargingSummary>('/charging/summary', { params: compactParams({ date }) })

/** 导出 xlsx（同列表筛选） */
export const exportChargeTransactions = (filter: ChargeTxFilter): Promise<DownloadedFile> =>
  download('/charging/transactions/export', '充电记录.xlsx', { params: compactParams(filter) })
