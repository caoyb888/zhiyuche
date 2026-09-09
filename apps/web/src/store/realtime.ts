import { create } from 'zustand'
import type { VehicleLive } from '../api/types'

export type WsStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'

/** WebSocket `vehicle.status` 事件的数据：vehicle_status 行（不含车牌 / 型号 / 驾驶员姓名），按 vehicle_id 合并进已有快照 */
export type VehicleLivePatch = Partial<VehicleLive> & { vehicle_id: string }

/** 把增量合并到已有实时状态；没有旧值时用缺省补齐必填字段 */
export function mergeVehicleLive(prev: VehicleLive | null | undefined, patch: VehicleLivePatch): VehicleLive {
  const base: VehicleLive = prev ?? {
    vehicle_id: patch.vehicle_id,
    plate_no: '',
    status: 'idle',
    sign_on: false,
    locked: false,
    charging: false,
    online: false,
    updated_at: new Date().toISOString(),
  }
  const next: VehicleLive = { ...base }
  for (const [k, v] of Object.entries(patch) as Array<[keyof VehicleLive, VehicleLive[keyof VehicleLive]]>) {
    // 推送里缺省的字段（undefined）不覆盖；显式 null 视为清空
    if (v === undefined) continue
    ;(next as Record<string, unknown>)[k] = v
  }
  // 推送不带 plate_no 等摘要字段时保留旧值
  if (!patch.plate_no && prev) next.plate_no = prev.plate_no
  return next
}

export interface RealtimeState {
  status: WsStatus
  /** 当前重连尝试次数（open 后归零） */
  attempts: number
  /** 车辆实时状态快照（vehicle_id → VehicleLive），由 /assets/vehicles/status 与 WebSocket 共同维护 */
  vehicles: Record<string, VehicleLive>
  /** 未读通知数；null = 尚未从接口取得 */
  unread: number | null
  /** 最近一次收到事件的时间（RFC3339） */
  lastEventAt: string | null

  setStatus: (status: WsStatus, attempts?: number) => void
  /** 用接口结果整体替换快照 */
  setVehicles: (list: VehicleLive[]) => void
  /** 合并单车增量，返回合并后的值 */
  upsertVehicle: (patch: VehicleLivePatch) => VehicleLive
  setUnread: (n: number) => void
  incUnread: () => void
  touch: (ts?: string) => void
  reset: () => void
}

export const useRealtimeStore = create<RealtimeState>()((set, get) => ({
  status: 'idle',
  attempts: 0,
  vehicles: {},
  unread: null,
  lastEventAt: null,

  setStatus: (status, attempts) => set((s) => ({ status, attempts: attempts ?? (status === 'open' ? 0 : s.attempts) })),
  setVehicles: (list) => set({ vehicles: Object.fromEntries(list.map((v) => [v.vehicle_id, v])) }),
  upsertVehicle: (patch) => {
    const merged = mergeVehicleLive(get().vehicles[patch.vehicle_id], patch)
    set((s) => ({ vehicles: { ...s.vehicles, [patch.vehicle_id]: merged } }))
    return merged
  },
  setUnread: (n) => set({ unread: Math.max(0, n) }),
  incUnread: () => set((s) => ({ unread: (s.unread ?? 0) + 1 })),
  touch: (ts) => set({ lastEventAt: ts ?? new Date().toISOString() }),
  reset: () => set({ status: 'idle', attempts: 0, vehicles: {}, unread: null, lastEventAt: null }),
}))
