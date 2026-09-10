import dayjs from 'dayjs'
import { useMemo } from 'react'
import { Area, CartesianGrid, ComposedChart, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { MeterValue } from '../../api/types'
import Empty from '../../components/ui/Empty'
import { sampleEvenly } from '../../utils/geo'

interface MeterChartProps {
  values: MeterValue[]
  height?: number
  /** 超过该点数时等间隔抽样（曲线接口返回全部点） */
  maxPoints?: number
}

interface Row {
  ts: number
  kwh: number | null
  power: number | null
  soc: number | null
}

const KWH_COLOR = '#f0a32b'
const POWER_COLOR = '#5c9df0'

function num(v: number | null | undefined): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

/** 电表曲线：累计电量（面积，左轴）与功率（折线，右轴），横轴为时间 */
export default function MeterChart({ values, height = 220, maxPoints = 600 }: MeterChartProps) {
  const rows = useMemo<Row[]>(() => {
    const list = values
      .map((m) => ({ ts: Date.parse(m.ts), kwh: num(m.kwh), power: num(m.power_kw), soc: num(m.soc) }))
      .filter((r) => Number.isFinite(r.ts))
      .sort((a, b) => a.ts - b.ts)
    return sampleEvenly(list, maxPoints)
  }, [values, maxPoints])

  const hasKwh = rows.some((r) => r.kwh !== null)
  const hasPower = rows.some((r) => r.power !== null)
  if (rows.length === 0 || (!hasKwh && !hasPower)) return <Empty size="sm" title="暂无电表数据" description="桩上报 MeterValues 后显示累计电量与功率曲线" />

  const span = rows[rows.length - 1].ts - rows[0].ts
  const fmt = span > 24 * 3600_000 ? 'MM-DD HH:mm' : 'HH:mm'

  return (
    <ResponsiveContainer width="100%" height={height}>
      <ComposedChart data={rows} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#16304f" vertical={false} />
        <XAxis dataKey="ts" type="number" scale="time" domain={['dataMin', 'dataMax']} tickFormatter={(v: number) => dayjs(v).format(fmt)} tick={{ fontSize: 11, fill: '#93aac4' }} axisLine={false} tickLine={false} minTickGap={40} />
        <YAxis yAxisId="kwh" tick={{ fontSize: 11, fill: '#93aac4' }} axisLine={false} tickLine={false} width={40} tickFormatter={(v: number) => v.toFixed(1)} />
        <YAxis yAxisId="power" orientation="right" tick={{ fontSize: 11, fill: '#93aac4' }} axisLine={false} tickLine={false} width={36} tickFormatter={(v: number) => v.toFixed(0)} />
        <Tooltip
          contentStyle={{ borderRadius: 10, border: '1px solid #23456B', background: '#0D1E33', boxShadow: '0 16px 40px -20px rgba(0,0,0,.9)', fontSize: 12 }} labelStyle={{ color: '#93AAC4' }} itemStyle={{ color: '#DCE7F2' }}
          labelFormatter={(v) => dayjs(Number(v)).format('YYYY-MM-DD HH:mm:ss')}
          formatter={(value, name) => {
            const n = typeof value === 'number' ? value : Number(value)
            if (name === '累计电量') return [`${n.toFixed(3)} kWh`, name]
            if (name === '功率') return [`${n.toFixed(2)} kW`, name]
            if (name === 'SOC') return [`${n.toFixed(0)}%`, name]
            return [String(value), String(name)]
          }}
        />
        <Legend iconSize={8} wrapperStyle={{ fontSize: 11 }} />
        {hasKwh && <Area yAxisId="kwh" type="monotone" dataKey="kwh" name="累计电量" stroke={KWH_COLOR} strokeWidth={2} fill={KWH_COLOR} fillOpacity={0.12} dot={false} activeDot={{ r: 4 }} connectNulls isAnimationActive={false} />}
        {hasPower && <Line yAxisId="power" type="monotone" dataKey="power" name="功率" stroke={POWER_COLOR} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls isAnimationActive={false} />}
      </ComposedChart>
    </ResponsiveContainer>
  )
}
