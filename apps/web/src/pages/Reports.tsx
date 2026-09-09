import { useState } from 'react'
import { BarChart, Bar, LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, PieChart, Pie, Cell, ResponsiveContainer } from 'recharts'
import { monthlyTrend, deptPie, dailyUsage, deptStats, top5 } from '../data/mock'
import type { Department } from '../types'
import { TrendingUp, TrendingDown } from 'lucide-react'

const COLORS: readonly string[] = ['#1d6fd8','#10b981','#f59e0b','#8b5cf6']
const DEPARTMENTS: Department[] = ['工程部','销售部','行政部','财务部']

type TrendTab = '费用' | '里程'
const trendTabs: TrendTab[] = ['费用','里程']

const summaryCards: Array<[label: string, value: string, color: string, bg: string]> = [
  ['本月总费用','¥ 7,510','text-brand-700','bg-brand-50'],
  ['本月里程','4,820 km','text-emerald-700','bg-emerald-50'],
  ['总行程次数','128 次','text-amber-700','bg-amber-50'],
  ['人均费用','¥ 156','text-purple-700','bg-purple-50'],
]

export default function ReportsPage() {
  const [tab, setTab] = useState<TrendTab>('费用')
  const topCost = top5[0]?.cost ?? 1

  return (
    <div className="space-y-5 slide-up">
      {/* Summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {summaryCards.map(([l,v,c,bg]) => (
          <div key={l} className={`stat-card ${bg} border-0`}>
            <div className="text-xs text-slate-500">{l}</div>
            <div className={`text-2xl font-bold mt-1 ${c}`}>{v}</div>
            <div className="text-xs text-emerald-600 mt-1 flex items-center gap-0.5"><TrendingDown size={11}/>较上月 -3.2%</div>
          </div>
        ))}
      </div>

      {/* Chart tabs */}
      <div className="card p-5">
        <div className="flex items-center justify-between mb-4 flex-wrap gap-3">
          <h2 className="text-sm font-semibold text-slate-700">月度趋势</h2>
          <div className="flex gap-1 bg-slate-100 p-1 rounded-xl">
            {trendTabs.map(t => (
              <button key={t} type="button" onClick={() => setTab(t)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${tab===t ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500'}`}>{t}</button>
            ))}
          </div>
        </div>
        <ResponsiveContainer width="100%" height={220}>
          <BarChart data={monthlyTrend} barSize={10} barGap={2}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" vertical={false}/>
            <XAxis dataKey="month" tick={{fontSize:11, fill:'#94a3b8'}} axisLine={false} tickLine={false}/>
            <YAxis tick={{fontSize:11, fill:'#94a3b8'}} axisLine={false} tickLine={false}/>
            <Tooltip contentStyle={{borderRadius:10,border:'none',boxShadow:'0 4px 20px #0001',fontSize:12}}/>
            <Legend iconSize={8} wrapperStyle={{fontSize:11}}/>
            {DEPARTMENTS.map((d,i) => (
              <Bar key={d} dataKey={d} fill={COLORS[i]} radius={[3,3,0,0]}/>
            ))}
          </BarChart>
        </ResponsiveContainer>
      </div>

      <div className="grid md:grid-cols-2 gap-5">
        {/* Pie */}
        <div className="card p-5">
          <h2 className="text-sm font-semibold text-slate-700 mb-4">本月费用构成</h2>
          <div className="flex items-center gap-4">
            <ResponsiveContainer width={140} height={140}>
              <PieChart>
                <Pie data={deptPie} cx="50%" cy="50%" innerRadius={40} outerRadius={65} paddingAngle={3} dataKey="value">
                  {deptPie.map((e) => <Cell key={e.name} fill={e.color}/>)}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
            <div className="space-y-2.5 flex-1">
              {deptPie.map((d) => (
                <div key={d.name} className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-2.5 h-2.5 rounded-full" style={{background:d.color}}/>
                    <span className="text-xs text-slate-600">{d.name}</span>
                  </div>
                  <span className="text-xs font-semibold text-slate-800">¥{d.value.toLocaleString()}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Daily usage */}
        <div className="card p-5">
          <h2 className="text-sm font-semibold text-slate-700 mb-4">近7日用车次数</h2>
          <ResponsiveContainer width="100%" height={140}>
            <LineChart data={dailyUsage}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" vertical={false}/>
              <XAxis dataKey="day" tick={{fontSize:11, fill:'#94a3b8'}} axisLine={false} tickLine={false}/>
              <YAxis tick={{fontSize:11, fill:'#94a3b8'}} axisLine={false} tickLine={false}/>
              <Tooltip contentStyle={{borderRadius:10,border:'none',fontSize:12}}/>
              <Line type="monotone" dataKey="trips" stroke="#1d6fd8" strokeWidth={2} dot={{r:3, fill:'#1d6fd8'}} name="行程次数"/>
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Dept table */}
      <div className="card p-5">
        <h2 className="text-sm font-semibold text-slate-700 mb-4">部门费用明细</h2>
        <div className="overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th>部门</th><th>行程次数</th><th>总里程</th><th>总费用</th><th>总耗电</th><th>环比</th>
              </tr>
            </thead>
            <tbody>
              {deptStats.map(d => (
                <tr key={d.dept}>
                  <td className="font-medium text-slate-800">{d.dept}</td>
                  <td>{d.trips} 次</td>
                  <td>{d.km.toLocaleString()} km</td>
                  <td className="font-semibold text-slate-800">¥ {d.cost.toLocaleString()}</td>
                  <td>{d.energy} kWh</td>
                  <td>
                    <span className={`flex items-center gap-0.5 text-xs font-medium ${d.change > 0 ? 'text-red-500' : 'text-emerald-600'}`}>
                      {d.change > 0 ? <TrendingUp size={11}/> : <TrendingDown size={11}/>}
                      {d.change > 0 ? '+' : ''}{d.change}%
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Top 5 */}
      <div className="card p-5">
        <h2 className="text-sm font-semibold text-slate-700 mb-4">用车费用 TOP 5</h2>
        <div className="space-y-3">
          {top5.map((p,i) => (
            <div key={p.name} className="flex items-center gap-3">
              <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold shrink-0 ${i===0?'bg-amber-400 text-white':i===1?'bg-slate-300 text-white':i===2?'bg-orange-300 text-white':'bg-slate-100 text-slate-500'}`}>{i+1}</div>
              <div className="w-8 h-8 rounded-full bg-brand-100 flex items-center justify-center text-brand-700 font-semibold text-xs shrink-0">{p.name.charAt(0)}</div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium text-slate-800">{p.name}</span>
                  <span className="text-sm font-bold text-slate-800">¥ {p.cost}</span>
                </div>
                <div className="flex items-center gap-3 mt-0.5 text-xs text-slate-400">
                  <span>{p.dept}</span>
                  <span>{p.trips}次</span>
                  <span>{p.km}km</span>
                </div>
                <div className="mt-1.5 h-1 bg-slate-100 rounded-full overflow-hidden">
                  <div className="h-full bg-brand-400 rounded-full" style={{width:`${(p.cost/topCost)*100}%`}}/>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
