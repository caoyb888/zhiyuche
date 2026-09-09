import { compactParams, del, get, post, put } from './client'
import type { Card, CardCreate, CardStatus, CardUpdate, Page, PageQuery } from './types'

/** GET /assets/nfc-cards 的筛选条件（不含分页） */
export interface CardFilter {
  /** 卡号 / 持卡人姓名 / 用户名 */
  keyword?: string
  status?: CardStatus | ''
  /** 是否已绑定持卡人 */
  bound?: boolean
}

export type CardListParams = CardFilter & PageQuery

export const cardKeys = {
  all: ['cards'] as const,
  list: (params: CardListParams) => ['cards', 'list', params] as const,
  detail: (id: string) => ['cards', 'detail', id] as const,
}

export const listCards = (params: CardListParams): Promise<Page<Card>> =>
  get<Page<Card>>('/assets/nfc-cards', { params: compactParams(params) })

export const getCard = (id: string): Promise<Card> => get<Card>(`/assets/nfc-cards/${id}`)

/** 发卡（可同时绑定用户；一个用户可持多张卡；UID 重复 409） */
export const createCard = (body: CardCreate): Promise<Card> => post<Card, CardCreate>('/assets/nfc-cards', body)

/** 状态 / 备注；status=lost 等同挂失 */
export const updateCard = (id: string, body: CardUpdate): Promise<Card> => put<Card, CardUpdate>(`/assets/nfc-cards/${id}`, body)

export const deleteCard = (id: string): Promise<unknown> => del<unknown>(`/assets/nfc-cards/${id}`)

export const bindCard = (id: string, user_id: string): Promise<Card> => post<Card>(`/assets/nfc-cards/${id}/bind`, { user_id })

export const unbindCard = (id: string): Promise<Card> => post<Card>(`/assets/nfc-cards/${id}/unbind`)

/** 挂失：status=lost，刷卡取车被拒 */
export const reportCardLoss = (id: string): Promise<Card> => post<Card>(`/assets/nfc-cards/${id}/report-loss`)
