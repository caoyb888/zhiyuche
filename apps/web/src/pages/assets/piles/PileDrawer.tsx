import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Controller, useForm } from 'react-hook-form'
import { errorMessage } from '../../../api/client'
import { createPile, updatePile } from '../../../api/piles'
import type { Pile, PileUpdate } from '../../../api/types'
import MapPicker from '../../../components/map/MapPicker'
import Button from '../../../components/ui/Button'
import Drawer from '../../../components/ui/Drawer'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Select from '../../../components/ui/Select'
import Textarea from '../../../components/ui/Textarea'
import { useToast } from '../../../components/ui/toast-context'
import { toLngLat } from '../../../utils/geo'
import { isPileManualStatus, numberOrUndefined, pileFormSchema, pileManualStatusOptions, pileStatusLabel, pileTypeOptions, type PileFormValues } from './schemas'

export type PileDrawerState = { mode: 'create' } | { mode: 'edit'; pile: Pile } | null

interface PileDrawerProps {
  state: PileDrawerState
  onClose: () => void
  onSaved: (pile: Pile) => void
}

function defaultsFor(p: Pile | undefined): PileFormValues {
  if (!p) {
    return { pile_code: '', name: '', type: 'slow', power_kw: '7', connector_count: '1', vendor: '', location: '', coord: null, status: '', remark: '' }
  }
  const pos = toLngLat(p.lng, p.lat)
  return {
    pile_code: p.pile_code,
    name: p.name,
    type: p.type,
    power_kw: String(p.power_kw),
    connector_count: String(p.connector_count),
    vendor: p.vendor ?? '',
    location: p.location ?? '',
    coord: pos ? { lng: pos[0], lat: pos[1] } : null,
    status: '',
    remark: p.remark ?? '',
  }
}

function PileForm({ pile, onCancel, onSaved }: { pile?: Pile; onCancel: () => void; onSaved: (p: Pile) => void }) {
  const toast = useToast()
  const isEdit = pile !== undefined
  const form = useForm<PileFormValues>({ resolver: zodResolver(pileFormSchema), defaultValues: defaultsFor(pile) })

  const save = useMutation({
    mutationFn: async (v: PileFormValues): Promise<Pile> => {
      if (!pile) {
        return createPile({
          pile_code: v.pile_code,
          name: v.name,
          type: v.type,
          // 契约带缺省值的字段在生成类型中为必填：留空时用契约缺省
          power_kw: numberOrUndefined(v.power_kw) ?? 7,
          connector_count: numberOrUndefined(v.connector_count) ?? 1,
          vendor: v.vendor || undefined,
          location: v.location || (v.coord?.address ?? undefined),
          lng: v.coord?.lng,
          lat: v.coord?.lat,
          remark: v.remark || undefined,
        })
      }
      const body: PileUpdate = {}
      if (v.name !== pile.name) body.name = v.name
      if (v.type !== pile.type) body.type = v.type
      const power = numberOrUndefined(v.power_kw)
      if (power !== undefined && power !== pile.power_kw) body.power_kw = power
      const conn = numberOrUndefined(v.connector_count)
      if (conn !== undefined && conn !== pile.connector_count) body.connector_count = conn
      if (v.vendor !== (pile.vendor ?? '')) body.vendor = v.vendor
      if (v.location !== (pile.location ?? '')) body.location = v.location
      // 契约无清空坐标的标记：只有选了新坐标才提交
      if (v.coord && (v.coord.lng !== pile.lng || v.coord.lat !== pile.lat)) {
        body.lng = v.coord.lng
        body.lat = v.coord.lat
      }
      if (v.status && isPileManualStatus(v.status) && v.status !== pile.status) body.status = v.status
      if (v.remark !== (pile.remark ?? '')) body.remark = v.remark
      if (Object.keys(body).length === 0) return pile
      return updatePile(pile.id, body)
    },
    onSuccess: (p) => {
      toast.success(isEdit ? '充电桩已更新' : '充电桩已创建')
      onSaved(p)
    },
    onError: (e) => toast.error(isEdit ? '更新失败' : '创建失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => save.mutate(v)}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField name="pile_code" label="桩编号" required hint={isEdit ? '编号不可修改' : '2–64 位字母、数字、_ 或 -；与 OCPP 上报的 ChargePointId 一致'}>
          <Input placeholder="如 CP-001" autoComplete="off" disabled={isEdit} className="font-mono" {...form.register('pile_code')} />
        </FormField>
        <FormField name="name" label="名称" required>
          <Input placeholder="如 1 号快充桩" {...form.register('name')} />
        </FormField>
        <FormField name="type" label="类型" required>
          <Select options={pileTypeOptions} {...form.register('type')} />
        </FormField>
        <FormField name="power_kw" label="功率（kW）">
          <Input type="number" inputMode="decimal" min={0.1} step="0.1" {...form.register('power_kw')} />
        </FormField>
        <FormField name="connector_count" label="枪数">
          <Input type="number" inputMode="numeric" min={1} max={20} {...form.register('connector_count')} />
        </FormField>
        <FormField name="vendor" label="厂商">
          <Input placeholder="可选" {...form.register('vendor')} />
        </FormField>
      </div>

      <FormField name="location" label="位置说明" hint="如 园区地下车库 B2 区；留空时使用选点得到的地址">
        <Input placeholder="文字位置描述" {...form.register('location')} />
      </FormField>

      <FormField name="coord" label="坐标" hint="在地图上选点或输入经纬度；充电管理页与总览地图按此定位">
        {({ id, invalid }) => <Controller control={form.control} name="coord" render={({ field }) => <MapPicker id={id} invalid={invalid} value={field.value} onChange={field.onChange} pointLabel="充电桩" height={240} />} />}
      </FormField>

      {isEdit && (
        <FormField name="status" label="手动状态" hint={`当前状态：${pileStatusLabel[pile.status]}。空闲 / 充电中 / 故障由 OCPP 上报；这里只能停用或恢复为离线（等待桩上线）`}>
          <Select options={pileManualStatusOptions} placeholder="不修改" {...form.register('status')} />
        </FormField>
      )}

      <FormField name="remark" label="备注">
        <Textarea placeholder="可选" {...form.register('remark')} />
      </FormField>

      <div className="flex justify-end gap-2 border-t border-line pt-2">
        <Button variant="secondary" onClick={onCancel} disabled={save.isPending}>
          取消
        </Button>
        <Button type="submit" loading={save.isPending}>
          {isEdit ? '保存修改' : '创建充电桩'}
        </Button>
      </div>
    </Form>
  )
}

/** 新建 / 编辑充电桩抽屉（坐标用 MapPicker 选点） */
export default function PileDrawer({ state, onClose, onSaved }: PileDrawerProps) {
  const pile = state?.mode === 'edit' ? state.pile : undefined
  return (
    <Drawer open={state !== null} onClose={onClose} title={pile ? `编辑充电桩 · ${pile.pile_code}` : '新建充电桩'} description={pile ? '只提交有变化的字段' : '新桩缺省为离线状态，桩接入 OCPP 后自动更新'} width="md">
      {state && <PileForm key={pile?.id ?? 'create'} pile={pile} onCancel={onClose} onSaved={onSaved} />}
    </Drawer>
  )
}
