import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import type { Profile, TenantBrief, TokenPair } from '../api/types'

/** 认证状态：令牌对 + 当前用户资料（含租户、角色、权限码、菜单树），持久化到 localStorage */
export interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  /** access token 过期时间（RFC3339） */
  expiresAt: string | null
  profile: Profile | null
  /**
   * 超级管理员"切换查看租户"：非空时所有业务请求附带 `X-Tenant-ID`，
   * 后端据此把读写范围切到该租户。null = 平台视角（本租户 / 全局）。
   */
  viewTenantId: string | null
  /** 与 viewTenantId 配套的展示信息（顶栏显示） */
  viewTenant: TenantBrief | null

  /** 登录成功：写入令牌与资料 */
  setSession: (tokens: TokenPair, profile: Profile) => void
  /** 刷新成功：只替换令牌对 */
  setTokens: (tokens: TokenPair) => void
  /** 重新拉取 /auth/me 后更新资料 */
  setProfile: (profile: Profile) => void
  /** 超级管理员切换查看租户；传 null 回到平台视角 */
  setViewTenant: (tenant: TenantBrief | null) => void
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
      viewTenantId: null,
      viewTenant: null,

      setSession: (tokens, profile) =>
        set({
          accessToken: tokens.access_token,
          refreshToken: tokens.refresh_token,
          expiresAt: tokens.expires_at,
          profile,
          // 换了账号：查看租户不跨会话保留
          viewTenantId: null,
          viewTenant: null,
        }),
      setTokens: (tokens) =>
        set({
          accessToken: tokens.access_token,
          refreshToken: tokens.refresh_token,
          expiresAt: tokens.expires_at,
        }),
      setProfile: (profile) => set({ profile }),
      setViewTenant: (tenant) => set({ viewTenantId: tenant?.id ?? null, viewTenant: tenant }),
      clear: () => set({ accessToken: null, refreshToken: null, expiresAt: null, profile: null, viewTenantId: null, viewTenant: null }),
    }),
    {
      name: AUTH_STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({
        accessToken: s.accessToken,
        refreshToken: s.refreshToken,
        expiresAt: s.expiresAt,
        profile: s.profile,
        viewTenantId: s.viewTenantId,
        viewTenant: s.viewTenant,
      }),
    },
  ),
)

/** 非 React 环境（axios 拦截器等）读取当前 access token */
export const getAccessToken = (): string | null => useAuthStore.getState().accessToken

/** 非 React 环境读取当前查看的租户 id（仅超级管理员有意义） */
export const getViewTenantId = (): string | null => {
  const { viewTenantId, profile } = useAuthStore.getState()
  return profile?.is_super ? viewTenantId : null
}
