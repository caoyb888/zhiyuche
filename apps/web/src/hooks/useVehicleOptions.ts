import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { useMemo } from 'react'
import type { VehicleBrief, VehicleStatus } from '../api/types'
import { listVehicleOptions, vehicleKeys } from '../api/vehicles'
import type { SelectOption } from '../components/ui/Select'
import { VEHICLE_STATUS_LABEL } from '../components/map/vehicleStyle'

export interface VehicleOptionsResult {
  query: UseQueryResult<VehicleBrief[]>
  vehicles: VehicleBrief[]
  options: SelectOption[]
}

/** 车辆简表（下拉：车牌 + 型号 + 状态）；后端未就绪时为空数组，query.isError 为 true */
export function useVehicleOptions(enabled = true, status?: VehicleStatus | ''): VehicleOptionsResult {
  const query = useQuery({
    queryKey: vehicleKeys.options(status),
    queryFn: () => listVehicleOptions(status),
    enabled,
    staleTime: 60_000,
  })
  const vehicles = useMemo(() => query.data ?? [], [query.data])
  const options = useMemo(
    () =>
      vehicles.map((v) => ({
        value: v.id,
        label: `${v.plate_no}${v.model ? ` · ${v.model}` : ''}（${VEHICLE_STATUS_LABEL[v.status]}）`,
      })),
    [vehicles],
  )
  return { query, vehicles, options }
}
