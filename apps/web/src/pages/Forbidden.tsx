import { ShieldOff } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import Button from '../components/ui/Button'

/** 403：已登录但无该页面权限 */
export default function Forbidden() {
  const navigate = useNavigate()
  return (
    <div className="flex min-h-[60vh] items-center justify-center slide-up">
      <div className="card px-10 py-12 text-center max-w-md w-full">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-warn-500/10 text-warn-200">
          <ShieldOff size={30} />
        </div>
        <div className="mt-5 text-4xl font-semibold text-ink-strong">403</div>
        <p className="mt-2 text-sm text-ink-muted">您没有访问该页面的权限，请联系管理员分配相应角色。</p>
        <div className="mt-6 flex justify-center gap-2">
          <Button variant="secondary" onClick={() => navigate(-1)}>
            返回上一页
          </Button>
          <Button onClick={() => navigate('/', { replace: true })}>回到首页</Button>
        </div>
      </div>
    </div>
  )
}
