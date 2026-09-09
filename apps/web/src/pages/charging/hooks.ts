import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'
import { chargingKeys, listPilesLive } from '../../api/charging'
import { dashboardKeys } from '../../api/dashboard'
import { pileKeys } from '../../api/piles'
import { vehicleKeys } from '../../api/vehicles'
import type { SelectOption } from '../../components/ui/Select'
import { PILES_LIVE_INTERVAL_MS } from './style'

/** 桩下拉：复用桩实时视图缓存（charging:view 即可，不依赖桩档案权限） */
export function usePileOptions(): { options: SelectOption[]; isError: boolean } {
  const live = useQuery({ queryKey: chargingKeys.pilesLive, queryFn: listPilesLive, staleTime: PILES_LIVE_INTERVAL_MS })
  const options = useMemo<SelectOption[]>(
    () =>
      (live.data ?? [])
        .map((p) => ({ value: p.id, label: `${p.name}（${p.pile_code}）` }))
        .sort((a, b) => a.label.localeCompare(b.label)),
    [live.data],
  )
  return { options, isError: live.isError }
}

/** 充电相关写操作后失效的缓存：充电全部、桩档案（状态）、车辆（充电中标记）、总览 */
export function useInvalidateCharging() {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: chargingKeys.all })
    void queryClient.invalidateQueries({ queryKey: pileKeys.all })
    void queryClient.invalidateQueries({ queryKey: vehicleKeys.all })
    void queryClient.invalidateQueries({ queryKey: dashboardKeys.all })
  }
}
