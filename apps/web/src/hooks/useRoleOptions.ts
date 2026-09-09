import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { useMemo } from 'react'
import { listRoleOptions, roleKeys } from '../api/roles'
import type { RoleBrief } from '../api/types'
import type { SelectOption } from '../components/ui/Select'

export interface RoleOptionsResult {
  query: UseQueryResult<RoleBrief[]>
  roles: RoleBrief[]
  options: SelectOption[]
}

/** 角色简表（后端未就绪时为空数组，query.isError 为 true） */
export function useRoleOptions(enabled = true): RoleOptionsResult {
  const query = useQuery({
    queryKey: roleKeys.options,
    queryFn: listRoleOptions,
    enabled,
    staleTime: 60_000,
  })
  const roles = useMemo(() => query.data ?? [], [query.data])
  const options = useMemo(() => roles.map((r) => ({ value: r.id, label: r.name })), [roles])
  return { query, roles, options }
}
