import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { errorMessage } from '../../../api/client'
import { bindDevice } from '../../../api/devices'
import type { Device } from '../../../api/types'
import Button from '../../../components/ui/Button'
import Modal from '../../../components/ui/Modal'
import Select, { type SelectOption } from '../../../components/ui/Select'
import { useToast } from '../../../components/ui/toast-context'

interface BindVehicleModalProps {
  device: Device | null
  vehicleOptions: SelectOption[]
  vehiclesUnavailable: boolean
  onClose: () => void
  onSaved: (device: Device) => void
}

function BindBody({ device, vehicleOptions, vehiclesUnavailable, onClose, onSaved }: BindVehicleModalProps & { device: Device }) {
  const toast = useToast()
  const [vehicleId, setVehicleId] = useState('')

  const bind = useMutation({
    mutationFn: () => bindDevice(device.id, vehicleId),
    onSuccess: (d) => {
      toast.success('已绑定车辆', d.vehicle_plate ? `${d.serial_no} → ${d.vehicle_plate}` : undefined)
      onSaved(d)
    },
    onError: (e) => toast.error('绑定失败', errorMessage(e)),
  })

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-600">
        为设备 <span className="font-mono font-medium text-slate-800">{device.serial_no}</span> 绑定车辆。一车一网关：目标车辆已有设备时会提示冲突。
      </p>
      <div className="space-y-1.5">
        <label htmlFor="bind-vehicle" className="block text-sm font-medium text-slate-700">
          车辆
        </label>
        <Select id="bind-vehicle" options={vehicleOptions} placeholder={vehiclesUnavailable ? '车辆列表暂不可用' : '请选择车辆'} value={vehicleId} onChange={(e) => setVehicleId(e.target.value)} disabled={vehiclesUnavailable} />
      </div>
      <div className="flex justify-end gap-2 pt-2">
        <Button variant="secondary" onClick={onClose} disabled={bind.isPending}>
          取消
        </Button>
        <Button onClick={() => bind.mutate()} loading={bind.isPending} disabled={!vehicleId}>
          绑定
        </Button>
      </div>
    </div>
  )
}

/** 设备绑定车辆 */
export default function BindVehicleModal(props: BindVehicleModalProps) {
  const { device, onClose } = props
  return (
    <Modal open={device !== null} onClose={onClose} title="绑定车辆" size="sm">
      {device && <BindBody key={device.id} {...props} device={device} />}
    </Modal>
  )
}
