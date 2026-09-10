import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { Search, UserRound, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { errorMessage } from '../../../api/client'
import { listUsers, userKeys } from '../../../api/users'
import Spinner from '../../../components/ui/Spinner'
import { controlClass } from '../../../components/ui/styles'
import { useDebounce } from '../../../hooks/useDebounce'

export interface PickedUser {
  id: string
  name: string
  username?: string
  dept_name?: string | null
}

export interface UserPickerProps {
  value: PickedUser | null
  onChange: (user: PickedUser | null) => void
  disabled?: boolean
  invalid?: boolean
  id?: string
  placeholder?: string
}

const PAGE_SIZE = 10

/** 持卡人选择：关键词搜索 `GET /system/users?keyword=`（去抖），下拉选择 */
export default function UserPicker({ value, onChange, disabled, invalid, id, placeholder = '搜索姓名 / 用户名 / 手机' }: UserPickerProps) {
  const [keyword, setKeyword] = useState('')
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const debounced = useDebounce(keyword.trim(), 300)

  const params = { keyword: debounced, pageSize: PAGE_SIZE, status: 'active' as const }
  const users = useQuery({
    queryKey: userKeys.list(params),
    queryFn: () => listUsers(params),
    enabled: open && debounced.length > 0,
    staleTime: 30_000,
  })

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  if (value) {
    return (
      <div className={controlClass(invalid, 'flex h-9 items-center gap-2 px-3')}>
        <UserRound size={14} className="shrink-0 text-ink-faint" />
        <span className="min-w-0 flex-1 truncate">
          <span className="font-medium text-ink-strong">{value.name}</span>
          {value.username && <span className="ml-1 text-xs text-ink-faint">@{value.username}</span>}
          {value.dept_name && <span className="ml-1 text-xs text-ink-faint">· {value.dept_name}</span>}
        </span>
        {!disabled && (
          <button type="button" aria-label="清除持卡人" onClick={() => onChange(null)} className="rounded p-0.5 text-ink-faint hover:text-ink">
            <X size={14} />
          </button>
        )}
      </div>
    )
  }

  const items = users.data?.items ?? []

  return (
    <div ref={rootRef} className="relative w-full">
      <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-faint" />
      <input
        id={id}
        value={keyword}
        disabled={disabled}
        aria-invalid={invalid || undefined}
        placeholder={placeholder}
        autoComplete="off"
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setKeyword(e.target.value)
          setOpen(true)
        }}
        onKeyDown={(e) => {
          if (e.key === 'Escape') setOpen(false)
        }}
        className={controlClass(invalid, 'h-9 pl-9 pr-3')}
      />
      {open && keyword.trim() && (
        <div className="absolute z-40 mt-1 w-full rounded-lg border border-line-strong bg-surface-2 shadow-lg">
          {users.isPending && debounced ? (
            <div className="flex justify-center py-3">
              <Spinner size="sm" label="搜索中" />
            </div>
          ) : users.isError ? (
            <div className="px-3 py-3 text-xs text-danger-200">{errorMessage(users.error)}</div>
          ) : items.length === 0 ? (
            <div className="px-3 py-3 text-center text-xs text-ink-faint">{debounced ? '无匹配用户' : '输入关键词搜索'}</div>
          ) : (
            <ul className="max-h-60 overflow-y-auto py-1">
              {items.map((u) => (
                <li key={u.id}>
                  <button
                    type="button"
                    onClick={() => {
                      onChange({ id: u.id, name: u.name, username: u.username, dept_name: u.dept_name })
                      setKeyword('')
                      setOpen(false)
                    }}
                    className={clsx('flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-surface-3')}
                  >
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-brand-600/10 text-[11px] font-medium text-brand-300">{u.name.slice(0, 1)}</span>
                    <span className="min-w-0 flex-1 truncate">
                      <span className="text-ink-strong">{u.name}</span>
                      <span className="ml-1 text-xs text-ink-faint">@{u.username}</span>
                    </span>
                    {u.dept_name && <span className="shrink-0 text-xs text-ink-faint">{u.dept_name}</span>}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
