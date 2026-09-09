import { useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { Building2, ChevronDown } from 'lucide-react'
import { listTenantOptions, tenantKeys } from '../api/tenants'
import { useAuthStore } from '../store/auth'

/**
 * 顶栏"切换查看租户"（仅超级管理员）：选中后所有业务请求附带 X-Tenant-ID，
 * 并重置全部查询缓存让各页面按新租户重新加载。
 */
export default function TenantSwitcher() {
  const queryClient = useQueryClient()
  const profile = useAuthStore((s) => s.profile)
  const viewTenantId = useAuthStore((s) => s.viewTenantId)
  const viewTenant = useAuthStore((s) => s.viewTenant)
  const setViewTenant = useAuthStore((s) => s.setViewTenant)

  const tenants = useQuery({
    queryKey: tenantKeys.options,
    queryFn: listTenantOptions,
    enabled: profile?.is_super === true,
    staleTime: 60_000,
  })

  if (!profile?.is_super) return null

  const options = (tenants.data ?? []).filter((t) => t.status === 'active' && t.id !== profile.tenant.id)
  // 持久化的查看租户已不在列表（被停用等）时仍显示，避免下拉值与状态不一致
  const stale = viewTenant && !options.some((t) => t.id === viewTenant.id) ? viewTenant : null
  const viewing = viewTenantId !== null

  const onChange = (id: string) => {
    if (!id) setViewTenant(null)
    else {
      const t = options.find((o) => o.id === id) ?? (stale?.id === id ? stale : undefined)
      if (!t) return
      setViewTenant({ id: t.id, code: t.code, name: t.name })
    }
    void queryClient.resetQueries()
  }

  return (
    <div
      className={clsx(
        'relative hidden sm:flex items-center gap-1.5 rounded-full border pl-2.5 pr-7 h-7 text-xs transition-colors',
        viewing ? 'border-purple-200 bg-purple-50 text-purple-800' : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300',
      )}
      title={viewing ? `正在以平台管理员身份查看租户「${viewTenant?.name ?? ''}」` : '切换查看其他租户的数据'}
    >
      <Building2 size={13} className="shrink-0" />
      <span className="max-w-[10rem] truncate">{viewing ? `查看：${viewTenant?.name ?? viewTenantId}` : '平台视角'}</span>
      <select
        aria-label="切换查看租户"
        value={viewTenantId ?? ''}
        onChange={(e) => onChange(e.target.value)}
        className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
        disabled={tenants.isPending}
      >
        <option value="">平台视角（{profile.tenant.name}）</option>
        {options.map((t) => (
          <option key={t.id} value={t.id}>
            {t.name}（{t.code}）
          </option>
        ))}
        {stale && (
          <option value={stale.id}>
            {stale.name}（{stale.code}，已不可用）
          </option>
        )}
        {tenants.isError && (
          <option value="" disabled>
            租户列表加载失败
          </option>
        )}
      </select>
      <ChevronDown size={13} className="absolute right-2 top-1/2 -translate-y-1/2 pointer-events-none" />
    </div>
  )
}
