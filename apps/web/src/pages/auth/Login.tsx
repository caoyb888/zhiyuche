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
    <div className="min-h-screen flex flex-col md:flex-row bg-slate-50">
      {/* ── 品牌区 ── */}
      <aside className="relative overflow-hidden bg-brand-900 text-white md:w-[46%] lg:w-[42%] flex flex-col justify-between px-8 py-8 md:px-12 md:py-12">
        <div className="pointer-events-none absolute -top-24 -right-24 w-80 h-80 rounded-full bg-brand-600/30 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-32 -left-20 w-96 h-96 rounded-full bg-ev-500/20 blur-3xl" />
        <div className="relative">
          <div className="flex items-center gap-3">
            <div className="w-11 h-11 rounded-xl bg-brand-600 flex items-center justify-center text-white font-bold text-lg shadow-lg shadow-brand-600/30">智</div>
            <div>
              <div className="text-xl font-semibold tracking-wide">智御系统</div>
              <div className="text-xs text-slate-300 mt-0.5">边缘控车 · 智慧驾驭</div>
            </div>
          </div>
          <p className="mt-6 md:mt-10 text-slate-300 text-sm leading-relaxed max-w-sm">
            面向企事业单位的车辆智能租赁与全生命周期管理平台，连接车端、桩端与业务流程，让每一次出行可管、可控、可追溯。
          </p>
          <ul className="hidden md:block mt-10 space-y-5">
            {features.map(({ icon: Icon, title, desc }) => (
              <li key={title} className="flex items-start gap-3">
                <div className="mt-0.5 w-9 h-9 rounded-lg bg-white/10 flex items-center justify-center shrink-0">
                  <Icon size={18} />
                </div>
                <div>
                  <div className="text-sm font-medium">{title}</div>
                  <div className="text-xs text-slate-400 mt-0.5">{desc}</div>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <div className="relative hidden md:block text-[11px] text-slate-500">测试版 V2.0 · © 智御系统</div>
      </aside>

      {/* ── 表单区 ── */}
      <main className="flex-1 flex items-center justify-center px-5 py-10 md:p-12">
        <div className="w-full max-w-md">
          <div className="card p-6 sm:p-8 slide-up">
            <h1 className="text-xl font-semibold text-slate-800">登录</h1>
            <p className="mt-1 text-sm text-slate-400">使用管理员分配的账号登录管理端</p>

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
                  className="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-brand-700"
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
                <div role="alert" className="flex items-start gap-2 rounded-lg border border-red-100 bg-red-50 px-3 py-2.5 text-sm text-red-700">
                  <AlertCircle size={16} className="mt-0.5 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              <Button type="submit" size="lg" block loading={mutation.isPending}>
                登 录
              </Button>
            </Form>
          </div>
          <p className="mt-4 text-center text-[11px] text-slate-400">连续输错 5 次密码将锁定 15 分钟</p>
        </div>
      </main>
    </div>
  )
}
