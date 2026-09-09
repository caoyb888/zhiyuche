import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import type { Vehicle, VehicleUpdate } from '../../../api/types'
import { createVehicle, updateVehicle } from '../../../api/vehicles'
import { VEHICLE_STATUS_LABEL } from '../../../components/map/vehicleStyle'
import Button from '../../../components/ui/Button'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select, { type SelectOption } from '../../../components/ui/Select'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import type { TreeNode } from '../../../components/ui/tree-utils'
import TreeSelect from '../../../components/ui/TreeSelect'
import { isManualStatus, isStatusLocked, numberOrUndefined, vehicleFormSchema, vehicleManualStatusOptions, type VehicleFormValues } from './schemas'

export type VehicleDrawerState = { mode: 'create' } | { mode: 'edit'; vehicle: Vehicle } | null

interface SharedProps {
  deptNodes: TreeNode[]
  deptLoading: boolean
  deptUnavailable: boolean
}

interface VehicleDrawerProps extends SharedProps {
  state: VehicleDrawerState
  onClose: () => void
  onSaved: (vehicle: Vehicle) => void
}

interface VehicleFormProps extends SharedProps {
  vehicle?: Vehicle
  onCancel: () => void
  onSaved: (vehicle: Vehicle) => void
}

function defaultsFor(v: Vehicle | undefined): VehicleFormValues {
  if (!v) {
    return {
      plate_no: '',
      vin: '',
      brand: '',
      model: '',
      color: '',
      seat_count: '5',
      battery_kwh: '60',
      range_km_full: '400',
      odometer_km: '0',
      purchase_date: '',
      insurance_expire: '',
      inspection_expire: '',
      home_dept_id: null,
      status: 'idle',
      remark: '',
    }
  }
  return {
    plate_no: v.plate_no,
    vin: v.vin ?? '',
    brand: v.brand ?? '',
    model: v.model ?? '',
    color: v.color ?? '',
    seat_count: String(v.seat_count),
    battery_kwh: String(v.battery_kwh),
    range_km_full: String(v.range_km_full),
    odometer_km: String(v.odometer_km),
    purchase_date: v.purchase_date ?? '',
    insurance_expire: v.insurance_expire ?? '',
    inspection_expire: v.inspection_expire ?? '',
    home_dept_id: v.home_dept_id ?? null,
    status: isManualStatus(v.status) ? v.status : 'idle',
    remark: v.remark ?? '',
  }
}

function VehicleForm({ vehicle, deptNodes, deptLoading, deptUnavailable, onCancel, onSaved }: VehicleFormProps) {
  const toast = useToast()
  const isEdit = vehicle !== undefined
  const currentStatus = vehicle?.live?.status ?? vehicle?.status
  const statusLocked = currentStatus !== undefined && isStatusLocked(currentStatus)

  const form = useForm<VehicleFormValues>({
    resolver: zodResolver(vehicleFormSchema),
    defaultValues: defaultsFor(vehicle),
  })

  // 在途 / 充电中：当前状态不在手动选项里，补一个禁用项用于展示
  const statusOptions: SelectOption[] =
    currentStatus && !isManualStatus(currentStatus) ? [{ value: currentStatus, label: VEHICLE_STATUS_LABEL[currentStatus], disabled: true }, ...vehicleManualStatusOptions] : vehicleManualStatusOptions

  const save = useMutation({
    mutationFn: async (v: VehicleFormValues): Promise<Vehicle> => {
      if (!vehicle) {
        return createVehicle({
          plate_no: v.plate_no,
          vin: v.vin ? v.vin.toUpperCase() : undefined,
          brand: v.brand || undefined,
          model: v.model || undefined,
          color: v.color || undefined,
          // 契约带缺省值的字段在生成类型中为必填：留空时用契约缺省
          seat_count: numberOrUndefined(v.seat_count) ?? 5,
          battery_kwh: numberOrUndefined(v.battery_kwh) ?? 60,
          range_km_full: numberOrUndefined(v.range_km_full) ?? 400,
          odometer_km: numberOrUndefined(v.odometer_km),
          purchase_date: v.purchase_date || undefined,
          insurance_expire: v.insurance_expire || undefined,
          inspection_expire: v.inspection_expire || undefined,
          home_dept_id: v.home_dept_id ?? undefined,
          remark: v.remark || undefined,
        })
      }
      // 只提交有变化的字段
      const body: VehicleUpdate = {}
      if (v.plate_no !== vehicle.plate_no) body.plate_no = v.plate_no
      const vin = v.vin.toUpperCase()
      if (vin !== (vehicle.vin ?? '')) body.vin = vin
      const textFields = ['brand', 'model', 'color', 'remark'] as const
      for (const key of textFields) {
        if (v[key] !== (vehicle[key] ?? '')) body[key] = v[key]
      }
      const dateFields = ['purchase_date', 'insurance_expire', 'inspection_expire'] as const
      for (const key of dateFields) {
        // 契约无清空标记：只有填写了新日期才提交
        if (v[key] && v[key] !== (vehicle[key] ?? '')) body[key] = v[key]
      }
      const seat = numberOrUndefined(v.seat_count)
      if (seat !== undefined && seat !== vehicle.seat_count) body.seat_count = seat
      const battery = numberOrUndefined(v.battery_kwh)
      if (battery !== undefined && battery !== vehicle.battery_kwh) body.battery_kwh = battery
      const range = numberOrUndefined(v.range_km_full)
      if (range !== undefined && range !== vehicle.range_km_full) body.range_km_full = range
      const odo = numberOrUndefined(v.odometer_km)
      if (odo !== undefined && odo !== vehicle.odometer_km) body.odometer_km = odo
      if (v.home_dept_id !== (vehicle.home_dept_id ?? null)) {
        if (v.home_dept_id) body.home_dept_id = v.home_dept_id
        else body.clear_home_dept = true
      }
      if (!statusLocked && v.status !== vehicle.status) body.status = v.status
      if (Object.keys(body).length === 0) return vehicle
      return updateVehicle(vehicle.id, body)
    },
    onSuccess: (saved) => {
      toast.success(isEdit ? '车辆已更新' : '车辆已创建')
      onSaved(saved)
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '创建失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="plate_no" label="车牌" required hint="2–16 个字符，租户内唯一">
          <Input placeholder="如 鲁A·A0001" autoComplete="off" {...form.register('plate_no')} />
        </FormField>
        <FormField name="vin" label="VIN" hint="可选；17 位车架号">
          <Input placeholder="17 位车架号" autoComplete="off" maxLength={17} className="font-mono uppercase" {...form.register('vin')} />
        </FormField>
        <FormField name="brand" label="品牌">
          <Input placeholder="如 比亚迪" {...form.register('brand')} />
        </FormField>
        <FormField name="model" label="型号">
          <Input placeholder="如 e6" {...form.register('model')} />
        </FormField>
        <FormField name="color" label="颜色">
          <Input placeholder="如 白色" {...form.register('color')} />
        </FormField>
        <FormField name="seat_count" label="座位数">
          <Input type="number" inputMode="numeric" min={1} max={60} {...form.register('seat_count')} />
        </FormField>
        <FormField name="battery_kwh" label="电池容量（kWh）">
          <Input type="number" inputMode="decimal" min={1} step="0.1" {...form.register('battery_kwh')} />
        </FormField>
        <FormField name="range_km_full" label="满电续航（km）">
          <Input type="number" inputMode="numeric" min={1} {...form.register('range_km_full')} />
        </FormField>
        <FormField name="odometer_km" label="当前里程（km）">
          <Input type="number" inputMode="decimal" min={0} step="0.1" {...form.register('odometer_km')} />
        </FormField>
        <FormField name="purchase_date" label="购置日期">
          <Input type="date" {...form.register('purchase_date')} />
        </FormField>
        <FormField name="insurance_expire" label="保险到期">
          <Input type="date" {...form.register('insurance_expire')} />
        </FormField>
        <FormField name="inspection_expire" label="年检到期">
          <Input type="date" {...form.register('inspection_expire')} />
        </FormField>
      </div>

      <FormField name="home_dept_id" label="归属部门" hint={deptUnavailable ? '部门数据暂不可用，可稍后再分配' : undefined}>
        {({ id, invalid }) => (
          <Controller
            control={form.control}
            name="home_dept_id"
            render={({ field }) => (
              <TreeSelect
                id={id}
                invalid={invalid}
                nodes={deptNodes}
                value={field.value}
                onChange={field.onChange}
                loading={deptLoading}
                placeholder="未指定归属部门"
                emptyText={deptUnavailable ? '部门数据暂不可用' : '暂无部门'}
              />
            )}
          />
        )}
      </FormField>

      {isEdit && (
        <FormField
          name="status"
          label="手动状态"
          hint={statusLocked && currentStatus ? `车辆当前${VEHICLE_STATUS_LABEL[currentStatus]}，不可手动切换状态；结束行程 / 充电后再操作` : '只能在空闲 / 维保中 / 停用之间切换；在途与充电中由系统维护'}
        >
          <Select options={statusOptions} disabled={statusLocked} {...form.register('status')} />
        </FormField>
      )}

      <FormField name="remark" label="备注">
        <Textarea placeholder="可选" {...form.register('remark')} />
      </FormField>

      <div className="flex justify-end gap-2 border-t border-slate-100 pt-2">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '创建车辆'}
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑车辆抽屉；按车辆 id 作为 key 重建表单 */
export default function VehicleDrawer({ state, onClose, onSaved, ...shared }: VehicleDrawerProps) {
  const vehicle = state?.mode === 'edit' ? state.vehicle : undefined
  return (
    <Drawer open={state !== null} onClose={onClose} title={vehicle ? `编辑车辆 · ${vehicle.plate_no}` : '新建车辆'} description={vehicle ? '只提交有变化的字段' : '新车辆缺省为空闲状态，并同步创建实时状态记录'} width="md">
      {state && <VehicleForm key={vehicle?.id ?? 'create'} vehicle={vehicle} onCancel={onClose} onSaved={onSaved} {...shared} />}
    </Drawer>
  )
}
