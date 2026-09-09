import { useState } from 'react'
import { vehicles, statusColor, socColor, alerts } from '../data/mock'
import { AlertTriangle, Car, Zap, type LucideIcon } from 'lucide-react'

interface StatCard {
  label: string
  value: number
  icon: LucideIcon
  color: string
  bg: string
}

export default function Dashboard() {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const sel = vehicles.find(v => v.id === selectedId)

  const stats: StatCard[] = [
    { label:'在途车辆', value: vehicles.filter(v=>v.status==='在途').length,    icon:Car,           color:'text-blue-600',   bg:'bg-blue-50' },
    { label:'空闲车辆', value: vehicles.filter(v=>v.status==='空闲').length,    icon:Car,           color:'text-emerald-600',bg:'bg-emerald-50' },
    { label:'充电中',   value: vehicles.filter(v=>v.status==='充电中').length,  icon:Zap,           color:'text-amber-600',  bg:'bg-amber-50' },
    { label:'需关注',   value: vehicles.filter(v=>v.soc<25||v.soh<80).length,  icon:AlertTriangle, color:'text-red-500',    bg:'bg-red-50' },
  ]

  const detailRows: Array<[label: string, value: string]> = sel
    ? [
        ['驾驶员', sel.driver || '—'],
        ['部门', sel.dept || '—'],
        ['事由', sel.trip || '—'],
        ['电量', `${sel.soc}%`],
        ['健康度', `${sel.soh}%`],
        ['里程', `${sel.odometer.toLocaleString()}km`],
      ]
    : []

  return (
    <div className="space-y-5 slide-up">
      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {stats.map(s => (
          <div key={s.label} className="stat-card flex items-center gap-3">
            <div className={`w-10 h-10 rounded-xl ${s.bg} flex items-center justify-center shrink-0`}>
              <s.icon size={18} className={s.color} />
            </div>
            <div>
              <div className={`text-2xl font-semibold ${s.color}`}>{s.value}</div>
              <div className="text-xs text-slate-400 mt-0.5">{s.label}</div>
            </div>
          </div>
        ))}
      </div>

      {/* Vehicle list */}
      <div className="card p-4">
        <h2 className="text-sm font-semibold text-slate-700 mb-3">车辆列表</h2>
        <div className="grid md:grid-cols-2 gap-1.5 max-h-80 overflow-y-auto pr-1">
          {vehicles.map(v => (
            <button key={v.id} type="button" onClick={() => setSelectedId(v.id===selectedId?null:v.id)}
              className={`w-full text-left px-3 py-2.5 rounded-xl border transition-all text-sm ${selectedId===v.id ? 'border-brand-200 bg-brand-50' : 'border-transparent hover:bg-slate-50'}`}>
              <div className="flex items-center justify-between">
                <span className="font-medium text-slate-700 text-xs">{v.plate}</span>
                <span className={statusColor[v.status]}>{v.status}</span>
              </div>
              <div className="flex items-center gap-3 mt-1.5">
                {/* SOC bar */}
                <div className="flex-1 flex items-center gap-1.5">
                  <div className="flex-1 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                    <div className={`h-full rounded-full transition-all ${v.soc>60?'bg-emerald-400':v.soc>25?'bg-amber-400':'bg-red-400'}`}
                      style={{width:`${v.soc}%`}}/>
                  </div>
                  <span className={`text-[11px] font-mono ${socColor(v.soc)}`}>{v.soc}%</span>
                </div>
                <span className="text-[11px] text-slate-400">SOH {v.soh}%</span>
              </div>
            </button>
          ))}
        </div>

        {/* Selected vehicle detail */}
        {sel && (
          <div className="mt-3 p-3 bg-slate-50 rounded-xl border border-slate-100 slide-up">
            <div className="flex items-center justify-between mb-2">
              <div className="font-medium text-sm text-slate-800">{sel.plate}</div>
              <span className={statusColor[sel.status]}>{sel.status}</span>
            </div>
            <div className="grid grid-cols-3 gap-2 text-xs">
              {detailRows.map(([l,v])=>(
                <div key={l}>
                  <div className="text-slate-400">{l}</div>
                  <div className={`font-medium mt-0.5 ${l==='电量'?socColor(sel.soc):''}`}>{v}</div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Alerts */}
      <div className="card p-4">
        <h2 className="text-sm font-semibold text-slate-700 mb-3">今日告警</h2>
        <div className="space-y-2">
          {alerts.map((a,i) => (
            <div key={i} className={`flex items-start gap-3 p-3 rounded-xl ${a.level==='red'?'bg-red-50':'bg-amber-50'}`}>
              <AlertTriangle size={15} className={`mt-0.5 shrink-0 ${a.level==='red'?'text-red-500':'text-amber-500'}`}/>
              <div className="flex-1 min-w-0">
                <p className={`text-sm ${a.level==='red'?'text-red-700':'text-amber-700'}`}>{a.msg}</p>
              </div>
              <span className="text-xs text-slate-400 shrink-0">{a.time}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
