import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createDevice, updateDevice } from '../../../api/devices'
import type { Device, DeviceUpdate, DeviceWithKey } from '../../../api/types'
import Button from '../../../components/ui/Button'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select, { type SelectOption } from '../../../components/ui/Select'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { DEFAULT_DEVICE_MODEL, deviceCreateSchema, deviceEditSchema, deviceStatusOptions, type DeviceCreateValues, type DeviceEditValues } from './schemas'

export type DeviceDrawerState = { mode: 'create' } | { mode: 'edit'; device: Device } | null

interface DeviceDrawerProps {
  state: DeviceDrawerState
  vehicleOptions: SelectOption[]
  vehiclesUnavailable: boolean
  onClose: () => void
  /** 新建成功：带一次性 api_key */
  onCreated: (device: DeviceWithKey) => void
  onUpdated: (device: Device) => void
}

function CreateForm({ vehicleOptions, vehiclesUnavailable, onCancel, onCreated }: { vehicleOptions: SelectOption[]; vehiclesUnavailable: boolean; onCancel: () => void; onCreated: (d: DeviceWithKey) => void }) {
  const toast = useToast()
  const form = useForm<DeviceCreateValues>({
    resolver: zodResolver(deviceCreateSchema),
    defaultValues: { serial_no: '', vehicle_id: '', model: DEFAULT_DEVICE_MODEL, firmware: '', iccid: '', remark: '' },
  })

  const save = useMutation({
    mutationFn: (v: DeviceCreateValues) =>
      createDevice({
        serial_no: v.serial_no,
        vehicle_id: v.vehicle_id || undefined,
        model: v.model || DEFAULT_DEVICE_MODEL,
        firmware: v.firmware || undefined,
        iccid: v.iccid || undefined,
        remark: v.remark || undefined,
      }),
    onSuccess: (d) => {
      toast.success('设备已创建', '请立即保存接入密钥')
      onCreated(d)
    },
    onError: (e) => toast.error('创建失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <FormField name="serial_no" label="序列号" required hint="4–64 位字母、数字、_ 或 -；租户内唯一">
        <Input placeholder="如 VIG-2024-000123" autoComplete="off" className="font-mono" {...form.register('serial_no')} />
      </FormField>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="model" label="型号" required>
          <Input placeholder={DEFAULT_DEVICE_MODEL} {...form.register('model')} />
        </FormField>
        <FormField name="firmware" label="固件版本">
          <Input placeholder="如 1.2.0" {...form.register('firmware')} />
        </FormField>
      </div>
      <FormField name="iccid" label="ICCID" hint="SIM 卡号，可选">
        <Input placeholder="19–20 位" className="font-mono" {...form.register('iccid')} />
      </FormField>
      <FormField name="vehicle_id" label="绑定车辆" hint={vehiclesUnavailable ? '车辆列表暂不可用，可稍后再绑定' : '可选；一车一网关，目标车辆已有设备时会提示冲突'}>
        <Select options={vehicleOptions} placeholder="暂不绑定" {...form.register('vehicle_id')} />
      </FormField>
      <FormField name="remark" label="备注">
        <Textarea placeholder="可选" {...form.register('remark')} />
      </FormField>
      <div className="flex justify-end gap-2 border-t border-slate-100 pt-2">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          创建并生成密钥
        </Button>
      </div>
    </Form>
  )
}

function EditForm({ device, onCancel, onUpdated }: { device: Device; onCancel: () => void; onUpdated: (d: Device) => void }) {
  const toast = useToast()
  const form = useForm<DeviceEditValues>({
    resolver: zodResolver(deviceEditSchema),
    defaultValues: { model: device.model, firmware: device.firmware ?? '', iccid: device.iccid ?? '', status: device.status, remark: device.remark ?? '' },
  })

  const save = useMutation({
    mutationFn: async (v: DeviceEditValues): Promise<Device> => {
      const body: DeviceUpdate = {}
      if (v.model !== device.model) body.model = v.model
      if (v.firmware !== (device.firmware ?? '')) body.firmware = v.firmware
      if (v.iccid !== (device.iccid ?? '')) body.iccid = v.iccid
      if (v.status !== device.status) body.status = v.status
      if (v.remark !== (device.remark ?? '')) body.remark = v.remark
      if (Object.keys(body).length === 0) return device
      return updateDevice(device.id, body)
    },
    onSuccess: (d) => {
      toast.success('设备已更新')
      onUpdated(d)
    },
    onError: (e) => toast.error('更新失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="model" label="型号" required>
          <Input {...form.register('model')} />
        </FormField>
        <FormField name="firmware" label="固件版本">
          <Input {...form.register('firmware')} />
        </FormField>
        <FormField name="iccid" label="ICCID">
          <Input className="font-mono" {...form.register('iccid')} />
        </FormField>
        <FormField name="status" label="状态" required hint="停用后该设备的上报将被拒绝">
          <Select options={deviceStatusOptions} {...form.register('status')} />
        </FormField>
      </div>
      <FormField name="remark" label="备注">
        <Textarea {...form.register('remark')} />
      </FormField>
      <div className="flex justify-end gap-2 border-t border-slate-100 pt-2">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          保存修改
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑设备抽屉 */
export default function DeviceDrawer({ state, vehicleOptions, vehiclesUnavailable, onClose, onCreated, onUpdated }: DeviceDrawerProps) {
  const device = state?.mode === 'edit' ? state.device : undefined
  return (
    <Drawer
      open={state !== null}
      onClose={onClose}
      title={device ? `编辑设备 · ${device.serial_no}` : '新建设备'}
      description={device ? '序列号不可修改；绑定 / 解绑车辆请使用列表操作' : '创建后返回一次性接入密钥，用于网关鉴权'}
      width="md"
    >
      {state?.mode === 'create' && <CreateForm key="create" vehicleOptions={vehicleOptions} vehiclesUnavailable={vehiclesUnavailable} onCancel={onClose} onCreated={onCreated} />}
      {device && <EditForm key={device.id} device={device} onCancel={onClose} onUpdated={onUpdated} />}
    </Drawer>
  )
}
