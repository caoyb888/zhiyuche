import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { ChevronDown, Search, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { errorMessage } from '../api/client'
import type { UserBrief } from '../api/types'
import { listUserOptions, userKeys } from '../api/users'
import { useDebounce } from '../hooks/useDebounce'
import Spinner from './ui/Spinner'
import { controlClass } from './ui/styles'

interface CommonProps {
  placeholder?: string
  disabled?: boolean
  invalid?: boolean
  id?: string
  className?: string
  /** 不可选的用户 id（如申请人本人不能作为随行人员） */
  excludeIds?: ReadonlySet<string>
}

interface SingleProps extends CommonProps {
  multiple?: false
  value: UserBrief | null
  onChange: (user: UserBrief | null) => void
}

interface MultipleProps extends CommonProps {
  multiple: true
  value: UserBrief[]
  onChange: (users: UserBrief[]) => void
}

export type UserPickerProps = SingleProps | MultipleProps

const PAGE_SIZE = 20

/**
 * 用户搜索选择（单选 / 多选）：关键字防抖后调用 GET /system/users/options
 * （任何登录用户可用的在职用户简表，不需要 system:user:view）。
 */
export default function UserPicker(props: UserPickerProps) {
  const { placeholder = '搜索姓名 / 用户名', disabled, invalid, id, className, excludeIds } = props
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const debounced = useDebounce(keyword.trim(), 300)
  const rootRef = useRef<HTMLDivElement>(null)

  const params = { keyword: debounced || undefined, limit: PAGE_SIZE }
  const query = useQuery({
    queryKey: userKeys.options(params),
    queryFn: () => listUserOptions(params),
    enabled: open,
    staleTime: 60_000,
  })

  const close = useCallback(() => {
    setOpen(false)
    setKeyword('')
  }, [])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) close()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        close()
      }
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey, true)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey, true)
    }
  }, [open, close])

  const selectedIds = new Set(props.multiple ? props.value.map((u) => u.id) : props.value ? [props.value.id] : [])
  const items = (query.data ?? []).filter((u) => !excludeIds?.has(u.id))
  const isDisabled = disabled

  const pick = (u: UserBrief) => {
    if (props.multiple) {
      if (selectedIds.has(u.id)) props.onChange(props.value.filter((x) => x.id !== u.id))
      else props.onChange([...props.value, u])
    } else {
      props.onChange(u)
      close()
    }
  }

  const clearAll = () => {
    if (props.multiple) props.onChange([])
    else props.onChange(null)
  }

  const hasValue = selectedIds.size > 0

  return (
    <div ref={rootRef} className={clsx('relative w-full', className)}>
      <button
        type="button"
        id={id}
        disabled={isDisabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-invalid={invalid || undefined}
        onClick={() => (open ? close() : setOpen(true))}
        className={controlClass(invalid, 'min-h-[2.25rem] py-1 pl-3 pr-14 text-left flex items-center')}
        title={undefined}
      >
        {props.multiple ? (
          props.value.length > 0 ? (
            <span className="flex flex-wrap gap-1">
              {props.value.map((u) => (
                <span key={u.id} className="inline-flex items-center gap-1 rounded-md bg-slate-100 px-1.5 py-0.5 text-xs text-slate-700">
                  {u.name}
                  {!disabled && (
                    <span
                      role="button"
                      aria-label={`移除 ${u.name}`}
                      onClick={(e) => {
                        e.stopPropagation()
                        props.onChange(props.value.filter((x) => x.id !== u.id))
                      }}
                      className="text-slate-400 hover:text-red-500"
                    >
                      <X size={11} />
                    </span>
                  )}
                </span>
              ))}
            </span>
          ) : (
            <span className="text-slate-400">{placeholder}</span>
          )
        ) : props.value ? (
          <span className="truncate">
            {props.value.name}
            {props.value.dept_name && <span className="ml-1 text-xs text-slate-400">{props.value.dept_name}</span>}
          </span>
        ) : (
          <span className="text-slate-400">{placeholder}</span>
        )}
      </button>
      <div className="absolute right-2 top-1/2 flex -translate-y-1/2 items-center gap-0.5">
        {hasValue && !disabled && (
          <button type="button" aria-label="清空" onMouseDown={(e) => e.preventDefault()} onClick={clearAll} className="rounded p-0.5 text-slate-400 hover:text-slate-600">
            <X size={14} />
          </button>
        )}
        <ChevronDown size={16} className={clsx('pointer-events-none text-slate-400 transition-transform', open && 'rotate-180')} />
      </div>

      {open && (
        <div className="absolute z-40 mt-1 w-full min-w-[16rem] rounded-lg border border-slate-200 bg-white shadow-lg">
          <div className="relative border-b border-slate-100 p-2">
            <Search size={14} className="absolute left-4 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              autoFocus
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              placeholder="输入姓名 / 用户名 / 手机号"
              aria-label="搜索用户"
              className="h-8 w-full rounded-md border border-slate-200 pl-7 pr-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-100"
            />
          </div>
          <ul role="listbox" aria-multiselectable={props.multiple || undefined} className="max-h-64 overflow-y-auto p-1">
            {query.isPending ? (
              <li className="flex justify-center py-4">
                <Spinner size="sm" label="搜索中" />
              </li>
            ) : query.isError ? (
              <li className="px-3 py-3 text-center text-xs text-red-500">{errorMessage(query.error)}</li>
            ) : items.length === 0 ? (
              <li className="px-3 py-3 text-center text-xs text-slate-400">{debounced ? '无匹配用户' : '暂无用户'}</li>
            ) : (
              items.map((u) => {
                const selected = selectedIds.has(u.id)
                return (
                  <li key={u.id}>
                    <button
                      type="button"
                      role="option"
                      aria-selected={selected}
                      onClick={() => pick(u)}
                      className={clsx('flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-left text-sm hover:bg-slate-50', selected && 'bg-brand-50 text-brand-700')}
                    >
                      <span className="min-w-0">
                        <span className="font-medium">{u.name}</span>
                        <span className="ml-1.5 text-xs text-slate-400">{u.username}</span>
                      </span>
                      {u.dept_name && <span className="shrink-0 truncate text-xs text-slate-400">{u.dept_name}</span>}
                    </button>
                  </li>
                )
              })
            )}
            {query.data && query.data.length >= PAGE_SIZE && <li className="px-3 py-1.5 text-center text-[11px] text-slate-400">仅显示前 {PAGE_SIZE} 条，请输入更精确的关键字</li>}
          </ul>
        </div>
      )}
    </div>
  )
}
