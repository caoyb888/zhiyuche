import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import type { Profile, TokenPair } from '../api/types'

/** 认证状态：令牌对 + 当前用户资料（含租户、角色、权限码、菜单树），持久化到 localStorage */
export interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  /** access token 过期时间（RFC3339） */
  expiresAt: string | null
  profile: Profile | null

  /** 登录成功：写入令牌与资料 */
  setSession: (tokens: TokenPair, profile: Profile) => void
  /** 刷新成功：只替换令牌对 */
  setTokens: (tokens: TokenPair) => void
  /** 重新拉取 /auth/me 后更新资料 */
  setProfile: (profile: Profile) => void
  /** 退出 / 刷新失败：清空全部 */
  clear: () => void
}

export const AUTH_STORAGE_KEY = 'zy-auth'

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      expiresAt: null,
      profile: null,

      setSession: (tokens, profile) =>
        set({
          accessToken: tokens.access_token,
          refreshToken: tokens.refresh_token,
          expiresAt: tokens.expires_at,
          profile,
        }),
      setTokens: (tokens) =>
        set({
          accessToken: tokens.access_token,
          refreshToken: tokens.refresh_token,
          expiresAt: tokens.expires_at,
        }),
      setProfile: (profile) => set({ profile }),
      clear: () => set({ accessToken: null, refreshToken: null, expiresAt: null, profile: null }),
    }),
    {
      name: AUTH_STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({
        accessToken: s.accessToken,
        refreshToken: s.refreshToken,
        expiresAt: s.expiresAt,
        profile: s.profile,
      }),
    },
  ),
)

/** 非 React 环境（axios 拦截器等）读取当前 access token */
export const getAccessToken = (): string | null => useAuthStore.getState().accessToken
