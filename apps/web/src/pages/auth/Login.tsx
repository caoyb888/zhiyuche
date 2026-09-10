import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import clsx from 'clsx'
import { AlertCircle, Building2, Car, ChevronDown, Lock, ShieldCheck, User, Zap } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import { login } from '../../api/auth'
import { ErrorCode, errorMessage, isApiError } from '../../api/client'
import Button from '../../components/ui/Button'
import { Form, FormField } from '../../components/ui/Form'
import Input from '../../components/ui/Input'
import { useAuthStore } from '../../store/auth'

const schema = z.object({
  username: z.string().trim().min(2, '用户名至少 2 个字符').max(64, '用户名最多 64 个字符'),
  password: z.string().min(1, '请输入密码'),
  tenant_code: z.string().trim().max(64, '租户代码最多 64 个字符'),
})

type LoginForm = z.infer<typeof schema>

const features = [
  { icon: Car, title: '车队实时监控', desc: '在途 / 空闲 / 充电状态一屏掌握' },
  { icon: ShieldCheck, title: '公务用车合规', desc: '申请、审批、行程全程留痕' },
  { icon: Zap, title: '能耗与费用分析', desc: '部门预算、充电成本自动核算' },
]

export default function Login() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const setSession = useAuthStore((s) => s.setSession)
  const [showTenant, setShowTenant] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const redirectParam = params.get('redirect')
  const redirectTo = redirectParam && redirectParam.startsWith('/') && !redirectParam.startsWith('//') ? redirectParam : '/'

  const form = useForm<LoginForm>({
    resolver: zodResolver(schema),
    defaultValues: { username: '', password: '', tenant_code: '' },
  })

  const mutation = useMutation({
    mutationFn: (v: LoginForm) =>
      login({ username: v.username, password: v.password, tenant_code: v.tenant_code || undefined }),
    onSuccess: (data) => {
      setError(null)
      setSession({ access_token: data.access_token, refresh_token: data.refresh_token, expires_at: data.expires_at }, data.user)
      navigate(redirectTo, { replace: true })
    },
    onError: (e) => {
      const msg = errorMessage(e, '登录失败，请稍后重试')
      setError(msg)
      // 后端 40000 "该用户名存在于多个租户，请指定租户代码" → 展开租户代码
      if (isApiError(e) && e.code === ErrorCode.BadRequest && msg.includes('租户')) {
        setShowTenant(true)
        form.setFocus('tenant_code')
      }
    },
  })

  return (
    <div className="min-h-screen flex flex-col md:flex-row bg-surface-0">
      {/* ── 品牌区 ── */}
      <aside
        className="relative overflow-hidden bg-[#071223] text-ink md:w-[46%] lg:w-[42%] flex flex-col justify-between px-8 py-8 md:px-12 md:py-12"
        style={{ backgroundImage: 'radial-gradient(rgba(60,207,224,.055) 1px, transparent 1px)', backgroundSize: '26px 26px' }}
      >
        <div className="pointer-events-none absolute -top-24 -right-24 w-80 h-80 rounded-full bg-brand-600/40 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-32 -left-20 w-96 h-96 rounded-full bg-tech-500/20 blur-3xl" />
        <span className="pointer-events-none absolute left-6 top-6 w-6 h-[1.5px] bg-tech-400/85" />
        <span className="pointer-events-none absolute left-6 top-6 w-[1.5px] h-6 bg-tech-400/85" />
        <div className="relative">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-[14px] bg-gradient-to-br from-brand-500 to-brand-700 ring-1 ring-tech-400/30 flex items-center justify-center text-white font-semibold text-xl shadow-[0_18px_40px_-16px_rgba(29,111,216,.95)]">智</div>
            <div>
              <div className="text-xl font-semibold tracking-[0.14em] text-ink-strong">智御系统</div>
              <div className="font-mono text-[10px] tracking-[0.24em] text-ink-faint mt-1">ZHIYUCHE FLEET PLATFORM</div>
            </div>
          </div>
          <p className="mt-6 md:mt-10 text-ink-muted text-sm leading-loose font-light max-w-sm">
            面向企事业单位的车辆智能租赁与全生命周期管理平台，连接车端、桩端与业务流程，让每一次出行可管、可控、可追溯。
          </p>
          <ul className="hidden md:block mt-10 space-y-5">
            {features.map(({ icon: Icon, title, desc }) => (
              <li key={title} className="flex items-start gap-3">
                <div className="mt-0.5 w-9 h-9 rounded-[10px] bg-tech-400/[0.09] border border-tech-400/25 text-tech-400 flex items-center justify-center shrink-0">
                  <Icon size={18} />
                </div>
                <div>
                  <div className="text-sm font-medium text-ink-strong">{title}</div>
                  <div className="text-xs text-ink-faint mt-1 leading-relaxed">{desc}</div>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <div className="relative hidden md:flex items-center gap-3 font-mono text-[11px] tracking-[0.12em] text-ink-disabled">
          <span>V2.0 · © 智御系统</span>
          <span className="w-px h-3 bg-line" />
          <span className="inline-flex items-center gap-1.5 tracking-normal font-sans">
            <span className="w-1.5 h-1.5 rounded-full bg-tech-400 shadow-glow inline-block" />
            等保三级 · 数据本地化部署
          </span>
        </div>
      </aside>

      {/* ── 表单区 ── */}
      <main className="flex-1 flex items-center justify-center px-5 py-10 md:p-12">
        <div className="w-full max-w-md">
          <div className="card relative overflow-hidden p-6 sm:p-8 slide-up">
            <span className="absolute left-0 top-0 h-0.5 w-14 bg-gradient-to-r from-tech-400 to-brand-600" />
            <div className="flex items-baseline gap-2.5">
              <h1 className="text-xl font-semibold text-ink-strong tracking-[0.06em]">账号登录</h1>
              <span className="font-mono text-[10px] tracking-[0.18em] text-ink-disabled">SIGN IN</span>
            </div>
            <p className="mt-2 text-sm text-ink-faint">使用管理员分配的账号登录管理端</p>

            <Form form={form} onSubmit={(v) => mutation.mutate(v)} className="mt-6">
              <FormField name="username" label="用户名" required>
                <Input icon={User} autoComplete="username" autoFocus placeholder="请输入用户名" {...form.register('username')} />
              </FormField>
              <FormField name="password" label="密码" required>
                <Input icon={Lock} type="password" autoComplete="current-password" placeholder="请输入密码" {...form.register('password')} />
              </FormField>

              <div>
                <button
                  type="button"
                  onClick={() => setShowTenant((v) => !v)}
                  className="inline-flex items-center gap-1 text-xs text-ink-muted hover:text-brand-300"
                  aria-expanded={showTenant}
                >
                  <ChevronDown size={14} className={clsx('transition-transform', showTenant && 'rotate-180')} />
                  {showTenant ? '收起租户代码' : '指定租户代码（可选）'}
                </button>
                {showTenant && (
                  <div className="mt-3">
                    <FormField name="tenant_code" label="租户代码" hint="同一用户名存在于多个租户时需要填写">
                      <Input icon={Building2} autoComplete="organization" placeholder="如 chenhua" {...form.register('tenant_code')} />
                    </FormField>
                  </div>
                )}
              </div>

              {error && (
                <div role="alert" className="flex items-start gap-2 rounded-lg border border-danger-500/35 bg-danger-500/10 px-3 py-2.5 text-sm text-danger-200">
                  <AlertCircle size={16} className="mt-0.5 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              <Button type="submit" size="lg" block loading={mutation.isPending}>
                登 录
              </Button>
            </Form>
          </div>
          <div className="mt-5 flex items-center justify-center gap-2.5">
            <span className="w-5 h-px bg-line" />
            <p className="text-center text-[11px] text-ink-disabled">
              连续输错 <span className="font-mono">5</span> 次密码将锁定 <span className="font-mono">15</span> 分钟
            </p>
            <span className="w-5 h-px bg-line" />
          </div>
        </div>
      </main>
    </div>
  )
}
