import { compactParams, del, get, post, put } from './client'
import type { NotifyChannel, Page, PageQuery, Template, TemplateCreate, TemplateUpdate } from './types'

export interface TemplateFilter {
  keyword?: string
  channel?: NotifyChannel | ''
}

export type TemplateListParams = TemplateFilter & PageQuery

export const templateKeys = {
  all: ['templates'] as const,
  list: (params: TemplateListParams) => ['templates', 'list', params] as const,
  detail: (id: string) => ['templates', 'detail', id] as const,
}

/** GET /system/notification-templates —— 全局 + 本租户，分页 */
export const listTemplates = (params: TemplateListParams): Promise<Page<Template>> =>
  get<Page<Template>>('/system/notification-templates', { params: compactParams(params) })

export const getTemplate = (id: string): Promise<Template> => get<Template>(`/system/notification-templates/${id}`)

/** 超级管理员未切换查看租户时创建全局模板；(作用域, code, channel) 重复返回 409 */
export const createTemplate = (body: TemplateCreate): Promise<Template> =>
  post<Template, TemplateCreate>('/system/notification-templates', body)

/** 只更新出现的字段；全局模板仅超级管理员可改（否则 403） */
export const updateTemplate = (id: string, body: TemplateUpdate): Promise<Template> =>
  put<Template, TemplateUpdate>(`/system/notification-templates/${id}`, body)

export const deleteTemplate = (id: string): Promise<unknown> => del<unknown>(`/system/notification-templates/${id}`)
