import { Compass } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'
import Button from '../components/ui/Button'

/** 404：路由不存在或页面尚未开放 */
export default function NotFound() {
  const navigate = useNavigate()
  const loc = useLocation()
  return (
    <div className="flex min-h-[60vh] items-center justify-center slide-up">
      <div className="card px-10 py-12 text-center max-w-md w-full">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-surface-4 text-ink-faint">
          <Compass size={30} />
        </div>
        <div className="mt-5 text-4xl font-semibold text-ink-strong">404</div>
        <p className="mt-2 text-sm text-ink-muted">页面不存在或尚未开放</p>
        <p className="mt-1 text-xs text-ink-faint font-mono break-all">{loc.pathname}</p>
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
