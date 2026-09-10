import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { CarFront } from 'lucide-react'
import { useMemo } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { bookingKeys, createBooking, listBookingAvailableVehicles, updateBooking } from '../../api/bookings'
import { errorMessage } from '../../api/client'
import type { Booking, BookingSource } from '../../api/types'
import Button from '../../components/ui/Button'
import DateTimeInput from '../../components/ui/DateTimeInput'
import Drawer from '../../components/ui/Drawer'
import { Form, FormField } from '../../components/ui/Form'
import Input from '../../components/ui/Input'
import Select, { type SelectOption } from '../../components/ui/Select'
import Textarea from '../../components/ui/Textarea'
import { useToast } from '../../components/ui/toast-context'
import UserPicker from '../../components/UserPicker'
import { localToRFC3339 } from '../../utils/format'
import { vehicleOptionLabel } from '../approval/style'
import { bookingFormSchema, bookingToForm, defaultBookingForm, toBookingCreate, toBookingUpdate, type BookingFormValues } from './schemas'
import { BOOKING_SOURCE_LABEL, BOOKING_SOURCE_OPTIONS } from './style'

export type BookingDrawerState = { mode: 'create'; source: BookingSource } | { mode: 'edit'; booking: Booking } | null

interface BookingDrawerProps {
  state: BookingDrawerState
  onClose: () => void
  onSaved: (booking: Booking) => void
}

function BookingForm({ booking, source, onCancel, onSaved }: { booking?: Booking; source: BookingSource; onCancel: () => void; onSaved: (b: Booking) => void }) {
  const toast = useToast()
  const isEdit = booking !== undefined
  const form = useForm<BookingFormValues>({
    resolver: zodResolver(bookingFormSchema),
    defaultValues: booking ? bookingToForm(booking) : defaultBookingForm(source),
  })

  // 派车下拉随预约时段变化：只列该时段空闲、且与其它预约/公务申请不冲突的车
  const start = useWatch({ control: form.control, name: 'reserve_start' })
  const end = useWatch({ control: form.control, name: 'reserve_end' })
  const slot = useMemo(() => {
    const s = localToRFC3339(start)
    const e = localToRFC3339(end)
    if (!s || !e || new Date(e) <= new Date(s)) return null
    return { start: s, end: e, exclude_booking_id: booking?.id }
  }, [start, end, booking?.id])

  const vehicles = useQuery({
    queryKey: bookingKeys.availableVehicles(slot ?? { start: '', end: '' }),
    queryFn: () => (slot ? listBookingAvailableVehicles(slot) : Promise.resolve([])),
    enabled: slot !== null,
    staleTime: 30_000,
  })

  const vehicleOptions = useMemo<SelectOption[]>(() => (vehicles.data ?? []).map((v) => ({ value: v.id, label: vehicleOptionLabel(v) })), [vehicles.data])

  const vehicleHint = (() => {
    if (!slot) return '先选好预约时段，再选择车辆'
    if (vehicles.isPending) return '正在查询该时段可用车辆…'
    if (vehicles.isError) return `可用车辆查询失败：${errorMessage(vehicles.error)}`
    if (vehicleOptions.length === 0) return '该时段没有空闲车辆，可先留空（待派车），空出车辆后再改派'
    return `该时段有 ${vehicleOptions.length} 辆车可用；留空表示待派车`
  })()

  const save = useMutation({
    mutationFn: async (v: BookingFormValues): Promise<Booking> => {
      if (!booking) return createBooking(toBookingCreate(v))
      const body = toBookingUpdate(v, booking)
      if (Object.keys(body).length === 0) return booking
      return updateBooking(booking.id, body)
    },
    onSuccess: (b) => {
      toast.success(isEdit ? '预约已更新' : `${BOOKING_SOURCE_LABEL[b.source]}已登记 ${b.booking_no}`)
      onSaved(b)
    },
    onError: (e) => toast.error(isEdit ? '保存失败' : '登记失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="source" label="预约来源" required hint="电话预约：客户来电由调度台代录；直接预约：当面 / 内部直接登记">
          <Select options={BOOKING_SOURCE_OPTIONS} {...form.register('source')} />
        </FormField>
        <FormField name="contact_name" label="来电人 / 预约人" required hint="记录人自动记为当前登录账号">
          <Input placeholder="如 张伟" {...form.register('contact_name')} />
        </FormField>
        <FormField name="contact_phone" label="联系电话">
          <Input placeholder="如 13800138000" inputMode="tel" {...form.register('contact_phone')} />
        </FormField>
        <FormField name="passenger" label="用车人" hint="可选：登记为系统用户后，行程与费用可归属到人">
          {({ id, invalid }) => (
            <Controller
              control={form.control}
              name="passenger"
              render={({ field }) => <UserPicker id={id} invalid={invalid} value={field.value} onChange={field.onChange} placeholder="搜索姓名 / 用户名（可留空）" />}
            />
          )}
        </FormField>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="reserve_start" label="预约开始" required>
          {({ id, invalid }) => (
            <Controller control={form.control} name="reserve_start" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />
          )}
        </FormField>
        <FormField name="reserve_end" label="预计结束" required>
          {({ id, invalid }) => (
            <Controller control={form.control} name="reserve_end" render={({ field }) => <DateTimeInput id={id} invalid={invalid} value={field.value} onChange={field.onChange} />} />
          )}
        </FormField>
      </div>

      <FormField name="vehicle_id" label="派车" hint={vehicleHint}>
        <Select options={vehicleOptions} placeholder="待派车（暂不指定）" disabled={slot === null} {...form.register('vehicle_id')} />
      </FormField>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="origin" label="出发地" required>
          <Input placeholder="如 公司总部 A 座" {...form.register('origin')} />
        </FormField>
        <FormField name="destination" label="目的地" required>
          <Input placeholder="如 首都机场 T3" {...form.register('destination')} />
        </FormField>
      </div>

      <FormField name="purpose" label="用车事由">
        <Input placeholder="如 接送客户" {...form.register('purpose')} />
      </FormField>

      <FormField name="remark" label="备注">
        <Textarea placeholder="可选" {...form.register('remark')} />
      </FormField>

      <div className="flex justify-end gap-2 border-t border-line pt-2">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" icon={CarFront} loading={save.isPending}>
          {isEdit ? '保存修改' : '登记预约'}
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑预约抽屉；已出车之后的预约不可编辑，由列表控制不再打开 */
export default function BookingDrawer({ state, onClose, onSaved }: BookingDrawerProps) {
  const booking = state?.mode === 'edit' ? state.booking : undefined
  const source: BookingSource = state?.mode === 'create' ? state.source : (booking?.source ?? 'phone')
  return (
    <Drawer
      open={state !== null}
      onClose={onClose}
      title={booking ? `编辑预约 · ${booking.booking_no}` : `新建${BOOKING_SOURCE_LABEL[source]}`}
      description={booking ? '只提交有变化的字段；改派或改时段会重新检查车辆占用' : '登记后立即生效并占用车辆，不需要审批'}
      width="md"
    >
      {state && <BookingForm key={booking?.id ?? `create-${source}`} booking={booking} source={source} onCancel={onClose} onSaved={onSaved} />}
    </Drawer>
  )
}
