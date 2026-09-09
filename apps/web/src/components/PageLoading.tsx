import Spinner from './ui/Spinner'

/** 路由懒加载 / 资料加载中的整页占位 */
export default function PageLoading({ label = '加载中…' }: { label?: string }) {
  return (
    <div className="flex h-full min-h-[40vh] w-full items-center justify-center">
      <Spinner size="lg" label={label} />
    </div>
  )
}
