import { useEffect, useState } from 'react'

/** 返回延迟 `delay` 毫秒后才更新的值（用于搜索输入） */
export function useDebounce<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = window.setTimeout(() => setDebounced(value), delay)
    return () => window.clearTimeout(t)
  }, [value, delay])
  return debounced
}
