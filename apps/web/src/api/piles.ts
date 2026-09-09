import { compactParams, del, get, post, put } from './client'
import type { Page, PageQuery, Pile, PileCreate, PileStatus, PileType, PileUpdate } from './types'

/** GET /assets/charge-piles 的筛选条件（不含分页） */
export interface PileFilter {
  /** 桩编号 / 名称 / 位置 */
  keyword?: string
  type?: PileType | ''
  status?: PileStatus | ''
}

export type PileListParams = PileFilter & PageQuery

export const pileKeys = {
  all: ['piles'] as const,
  list: (params: PileListParams) => ['piles', 'list', params] as const,
  /** 列表顶部地图：一次拉全（契约 pageSize 上限 200） */
  map: ['piles', 'map'] as const,
  detail: (id: string) => ['piles', 'detail', id] as const,
}

export const listPiles = (params: PileListParams): Promise<Page<Pile>> =>
  get<Page<Pile>>('/assets/charge-piles', { params: compactParams(params) })

/** 地图用：全部桩（最多 200） */
export const listAllPiles = (): Promise<Pile[]> => listPiles({ pageSize: 200, sort: 'pile_code' }).then((p) => p.items)

export const getPile = (id: string): Promise<Pile> => get<Pile>(`/assets/charge-piles/${id}`)

/** 编号重复 409 */
export const createPile = (body: PileCreate): Promise<Pile> => post<Pile, PileCreate>('/assets/charge-piles', body)

/** status 手动只能设 disabled 或恢复 offline，其余由 OCPP 上报 */
export const updatePile = (id: string, body: PileUpdate): Promise<Pile> => put<Pile, PileUpdate>(`/assets/charge-piles/${id}`, body)

export const deletePile = (id: string): Promise<unknown> => del<unknown>(`/assets/charge-piles/${id}`)
