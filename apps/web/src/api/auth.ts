import { get, post, put } from './client'
import type { LoginRequest, LoginResponse, MenuNode, Profile, TokenPair } from './types'

export const authKeys = {
  me: ['auth', 'me'] as const,
}

/** POST /auth/login —— 公开接口，401 不触发刷新 */
export const login = (body: LoginRequest): Promise<LoginResponse> =>
  post<LoginResponse, LoginRequest>('/auth/login', body, { skipAuth: true, skipAuthRefresh: true })

/** POST /auth/refresh —— refresh token 一次性轮换 */
export const refreshToken = (refresh_token: string): Promise<TokenPair> =>
  post<TokenPair>('/auth/refresh', { refresh_token }, { skipAuth: true, skipAuthRefresh: true })

/** POST /auth/logout —— 吊销当前 access token，可选吊销 refresh token */
export const logout = (refresh_token?: string | null): Promise<unknown> =>
  post<unknown>('/auth/logout', refresh_token ? { refresh_token } : {}, { skipAuthRefresh: true })

/** GET /auth/me —— 当前用户资料、权限码与菜单树 */
export const getMe = (): Promise<Profile> => get<Profile>('/auth/me')

/** GET /auth/menus */
export const getMenus = (): Promise<MenuNode[]> => get<MenuNode[]>('/auth/menus')

export interface ChangePasswordBody {
  old_password: string
  new_password: string
}

/** PUT /auth/password —— 修改本人密码 */
export const changePassword = (body: ChangePasswordBody): Promise<unknown> =>
  put<unknown, ChangePasswordBody>('/auth/password', body)
