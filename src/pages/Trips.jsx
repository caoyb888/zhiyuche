import { useState } from 'react'
import { trips } from '../data/mock'
import { MapPin, Clock, Zap, DollarSign, ChevronRight, Navigation, ShieldCheck } from 'lucide-react'

function TripDetail({ trip, onClose }) {
  const [playing, setPlaying] = useState(false)
  const [progress, setProgress] = useState(0)

  const play = () => {
    if (playing) { setPlaying(false); return }
    setPlaying(true)
    setProgress(0)
    let p = 0
    const iv = setInterval(() => {
      p += 2
      setProgress(p)
      if (p >= 100) { clearInterval(iv); setPlaying(false) }
    }, 60)
  }

  const scoreColor = s => s >= 85 ? 'text-emerald-600' : s >= 70 ? 'text-amber-500' : 'text-red-500'
  const scoreLabel = s => s >= 85 ? '优秀' : s >= 70 ? '良好' : '较差'
  const score = Math.max(60, 100 - trip.brake * 3 - trip.accel * 2)

  return (
    <div className="fixed inset-0 z-50 flex items-end md:items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div className="bg-white rounded-2xl w-full max-w-lg max-h-[90vh] overflow-y-auto shadow-2xl slide-up" onClick={e=>e.stopPropagation()}>
        <div className="p-5 border-b border-slate-100">
          <div className="flex items-center justify-between">
            <div>
              <div className="font-semibold text-slate-800">{trip.plate} · {trip.purpose}</div>
              <div className="text-xs text-slate-400 mt-0.5">{trip.date} · {trip.driver} · {trip.dept}</div>
            </div>
            {trip.sign && <div className="bg-brand-900 text-white text-[11px] px-3 py-1.5 rounded-lg font-bold tracking-wider sign-glow">市中公务用车</div>}
          </div>
        </div>

        <div className="p-5 space-y-4">
          {/* Timeline */}
          <div className="flex items-start gap-3">
            <div className="flex flex-col items-center shrink-0">
              <div className="w-3 h-3 rounded-full bg-brand-600 mt-0.5"/>
              <div className="w-0.5 bg-slate-200 flex-1 my-1" style={{height:36}}/>
              <div className="w-3 h-3 rounded-full bg-emerald-500"/>
            </div>
            <div className="flex-1 space-y-4">
              <div>
                <div className="text-xs text-slate-400">出发</div>
                <div className="text-sm font-medium text-slate-800">{trip.startTime} · 公司园区</div>
              </div>
              <div>
                <div className="text-xs text-slate-400">到达</div>
                <div className="text-sm font-medium text-slate-800">{trip.endTime} · 返回公司</div>
              </div>
            </div>
            <div className="text-right">
              <div className="text-xs text-slate-400">行驶时长</div>
              <div className="text-sm font-semibold text-slate-800">{trip.duration}</div>
            </div>
          </div>

          {/* Route map mock */}
          <div className="bg-slate-800 rounded-xl overflow-hidden relative" style={{height:160}}>
            <svg width="100%" height="100%" viewBox="0 0 400 160" preserveAspectRatio="none" className="opacity-30">
              <line x1="0" y1="80" x2="400" y2="80" stroke="#475569" strokeWidth="2"/>
              <line x1="200" y1="0" x2="200" y2="160" stroke="#475569" strokeWidth="2"/>
            </svg>
            {/* Route line */}
            <svg width="100%" height="100%" viewBox="0 0 400 160" className="absolute inset-0">
              <polyline points="40,120 100,90 160,70 250,60 320,50 370,40"
                fill="none" stroke="#60a5fa" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" strokeDasharray="300" strokeDashoffset={300 - progress*3} style={{transition:'stroke-dashoffset 0.1s'}}/>
              {progress > 0 && (
                <circle cx={40 + progress*3.3} cy={120 - progress*0.8} r="5" fill="#3b82f6" className="pulse-dot"/>
              )}
              <circle cx="40" cy="120" r="5" fill="#10b981"/>
              <circle cx="370" cy="40" r="5" fill="#f59e0b"/>
            </svg>
            <div className="absolute bottom-3 left-3 text-white/60 text-[10px]">{trip.route}</div>
            <button onClick={play} className={`absolute top-3 right-3 text-xs px-3 py-1.5 rounded-lg font-medium transition-all ${playing ? 'bg-amber-500 text-white' : 'bg-white/20 text-white hover:bg-white/30'}`}>
              {playing ? '▶ 回放中…' : '▶ 轨迹回放'}
            </button>
          </div>

          {/* Stats grid */}
          <div className="grid grid-cols-2 gap-3">
            {[
              ['总里程', `${trip.distance} km`, MapPin, 'text-blue-600'],
              ['消耗电量', `${trip.energy} kWh`, Zap, 'text-amber-600'],
              ['行程费用', `¥ ${trip.cost}`, DollarSign, 'text-emerald-600'],
              ['百公里耗电', `${((trip.energy/trip.distance)*100).toFixed(1)} kWh`, Navigation, 'text-purple-600'],
            ].map(([l,v,Icon,c]) => (
              <div key={l} className="bg-slate-50 rounded-xl p-3">
                <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1"><Icon size={12} className={c}/>{l}</div>
                <div className={`text-lg font-semibold ${c}`}>{v}</div>
              </div>
            ))}
          </div>

          {/* Driving score */}
          <div className="bg-slate-50 rounded-xl p-4">
            <div className="flex items-center justify-between mb-3">
              <div className="text-sm font-medium text-slate-700 flex items-center gap-2"><ShieldCheck size={15} className="text-brand-600"/>驾驶行为评分</div>
              <div className={`text-xl font-bold ${scoreColor(score)}`}>{score}<span className="text-sm font-normal ml-1">{scoreLabel(score)}</span></div>
            </div>
            <div className="space-y-2">
              {[['急刹车', trip.brake, 10, '次'],['急加速', trip.accel, 8, '次'],['最高速度', trip.maxSpeed, 90, 'km/h']].map(([l,v,max,unit]) => (
                <div key={l} className="flex items-center gap-3 text-sm">
                  <div className="w-16 text-slate-500 text-xs shrink-0">{l}</div>
                  <div className="flex-1 h-1.5 bg-slate-200 rounded-full overflow-hidden">
                    <div className={`h-full rounded-full ${v/max > 0.8 ? 'bg-red-400' : v/max > 0.5 ? 'bg-amber-400' : 'bg-emerald-400'}`} style={{width:`${Math.min(100,(v/max)*100)}%`}}/>
                  </div>
                  <div className="text-xs text-slate-600 w-16 text-right">{v} {unit}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Fee breakdown */}
          <div className="border border-slate-100 rounded-xl overflow-hidden">
            <div className="px-4 py-3 bg-slate-50 text-xs font-medium text-slate-500 border-b border-slate-100">费用明细</div>
            {[
              ['里程费', `${trip.distance} km × ¥0.60`, (trip.distance * 0.6).toFixed(2)],
              ['时长费', `按实际时长计`, (trip.cost - trip.distance*0.6 - trip.energy*0.65).toFixed(2)],
              ['电费',   `${trip.energy} kWh × ¥0.65`, (trip.energy * 0.65).toFixed(2)],
            ].map(([l,d,v]) => (
              <div key={l} className="flex items-center justify-between px-4 py-3 border-b border-slate-50 last:border-0 text-sm">
                <div className="text-slate-600">{l}</div>
                <div className="text-slate-400 text-xs">{d}</div>
                <div className="font-medium text-slate-800">¥ {v}</div>
              </div>
            ))}
            <div className="flex items-center justify-between px-4 py-3 bg-brand-50">
              <div className="font-semibold text-brand-800">合计</div>
              <div className="font-bold text-brand-700 text-lg">¥ {trip.cost}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default function TripsPage() {
  const [selected, setSelected] = useState(null)
  const [search, setSearch] = useState('')
  const [dateFilter, setDateFilter] = useState('全部')

  const dates = ['全部', '2025-06-18', '2025-06-17']
  const filtered = trips.filter(t =>
    (dateFilter === '全部' || t.date === dateFilter) &&
    (!search || t.plate.includes(search) || t.driver.includes(search) || t.purpose.includes(search))
  )

  const totalKm = filtered.reduce((s,t)=>s+t.distance,0)
  const totalCost = filtered.reduce((s,t)=>s+t.cost,0)
  const totalEnergy = filtered.reduce((s,t)=>s+t.energy,0)

  return (
    <div className="space-y-4 slide-up">
      {/* Summary */}
      <div className="grid grid-cols-3 gap-3">
        {[['总里程', `${totalKm.toFixed(1)} km`, 'text-blue-600'],
          ['总费用', `¥ ${totalCost.toFixed(1)}`, 'text-emerald-600'],
          ['总耗电', `${totalEnergy.toFixed(1)} kWh`, 'text-amber-600'],
        ].map(([l,v,c]) => (
          <div key={l} className="stat-card">
            <div className="text-xs text-slate-400">{l}</div>
            <div className={`text-xl font-bold mt-1 ${c}`}>{v}</div>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <input value={search} onChange={e=>setSearch(e.target.value)} placeholder="搜索车牌、驾驶员、事由…"
          className="flex-1 min-w-40 px-3 py-2 text-sm border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-brand-200"/>
        <div className="flex gap-1 bg-slate-100 p-1 rounded-xl">
          {dates.map(d => (
            <button key={d} onClick={() => setDateFilter(d)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${dateFilter===d ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500'}`}>
              {d==='全部'?'全部':d.slice(5)}
            </button>
          ))}
        </div>
      </div>

      {/* Trip list */}
      <div className="space-y-2.5">
        {filtered.map(trip => (
          <div key={trip.id} onClick={() => setSelected(trip)}
            className="card p-4 cursor-pointer hover:shadow-md transition-all hover:-translate-y-0.5">
            <div className="flex items-start gap-3">
              <div className="w-10 h-10 rounded-xl bg-brand-50 flex items-center justify-center shrink-0">
                <Navigation size={18} className="text-brand-600"/>
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2 flex-wrap">
                  <div className="font-semibold text-slate-800 text-sm">{trip.plate} · {trip.driver}</div>
                  <div className="flex items-center gap-2">
                    {trip.sign && <span className="badge-blue text-[10px] px-2">公务</span>}
                    <span className="badge-green">{trip.status}</span>
                  </div>
                </div>
                <div className="text-xs text-slate-400 mt-1">{trip.date} {trip.startTime}→{trip.endTime} · {trip.purpose}</div>
                <div className="flex items-center gap-4 mt-2 text-xs">
                  <span className="flex items-center gap-1 text-blue-600"><MapPin size={11}/>{trip.distance} km</span>
                  <span className="flex items-center gap-1 text-amber-600"><Zap size={11}/>{trip.energy} kWh</span>
                  <span className="flex items-center gap-1 text-emerald-600"><DollarSign size={11}/>¥{trip.cost}</span>
                  <span className="text-slate-400">{trip.duration}</span>
                </div>
              </div>
              <ChevronRight size={16} className="text-slate-300 shrink-0 mt-2"/>
            </div>
          </div>
        ))}
      </div>

      {selected && <TripDetail trip={selected} onClose={() => setSelected(null)}/>}
    </div>
  )
}
