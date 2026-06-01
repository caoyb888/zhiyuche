import { useState } from 'react'
import { chargers, chargeHistory } from '../data/mock'
import { Zap, AlertTriangle, CheckCircle, Clock } from 'lucide-react'

const pileStatusStyle = {
  '使用中': { dot:'bg-blue-500',  badge:'badge-blue',  bg:'bg-blue-50  border-blue-100' },
  '空闲':   { dot:'bg-emerald-500', badge:'badge-green', bg:'bg-emerald-50 border-emerald-100' },
  '故障':   { dot:'bg-red-500',   badge:'badge-red',   bg:'bg-red-50   border-red-100' },
}

function SocRing({ soc }) {
  const r = 18, circ = 2 * Math.PI * r
  const color = soc > 60 ? '#10b981' : soc > 25 ? '#f59e0b' : '#ef4444'
  return (
    <svg width="48" height="48" viewBox="0 0 48 48">
      <circle cx="24" cy="24" r={r} fill="none" stroke="#f1f5f9" strokeWidth="4"/>
      <circle cx="24" cy="24" r={r} fill="none" stroke={color} strokeWidth="4"
        strokeDasharray={circ} strokeDashoffset={circ * (1 - soc/100)}
        strokeLinecap="round" transform="rotate(-90 24 24)"/>
      <text x="24" y="28" textAnchor="middle" fontSize="11" fontWeight="600" fill={color}>{soc}%</text>
    </svg>
  )
}

export default function ChargingPage() {
  const [history, setHistory] = useState(false)
  const inUse = chargers.filter(c => c.status==='使用中').length
  const idle  = chargers.filter(c => c.status==='空闲').length
  const fault = chargers.filter(c => c.status==='故障').length

  const todayKwh = chargeHistory.slice(0,2).reduce((s,h) => s + h.kwh, 0)
  const todayCost = chargeHistory.slice(0,2).reduce((s,h) => s + h.cost, 0)

  return (
    <div className="space-y-5 slide-up">
      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {[['使用中', inUse, 'text-blue-600', 'bg-blue-50'],
          ['空闲桩位', idle, 'text-emerald-600', 'bg-emerald-50'],
          ['今日充电量', `${todayKwh.toFixed(1)}kWh`, 'text-amber-600', 'bg-amber-50'],
          ['今日充电费', `¥${todayCost.toFixed(1)}`, 'text-purple-600', 'bg-purple-50'],
        ].map(([l,v,c,bg]) => (
          <div key={l} className={`stat-card ${bg} border-0`}>
            <div className="text-xs text-slate-500">{l}</div>
            <div className={`text-2xl font-bold mt-1 ${c}`}>{v}</div>
          </div>
        ))}
      </div>

      {/* Fault alert */}
      {fault > 0 && (
        <div className="bg-red-50 border border-red-100 rounded-2xl p-4 flex items-center gap-3">
          <AlertTriangle size={18} className="text-red-500 shrink-0"/>
          <div>
            <div className="text-sm font-semibold text-red-800">{fault} 个充电桩故障</div>
            <div className="text-xs text-red-600 mt-0.5">5号慢充桩异常，已自动生成维修工单</div>
          </div>
        </div>
      )}

      {/* Pile grid */}
      <div>
        <h2 className="text-sm font-semibold text-slate-700 mb-3">充电桩实时状态</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
          {chargers.map(pile => {
            const s = pileStatusStyle[pile.status] || pileStatusStyle['空闲']
            return (
              <div key={pile.id} className={`card p-4 border ${s.bg}`}>
                <div className="flex items-start justify-between mb-3">
                  <div>
                    <div className="text-sm font-semibold text-slate-800">{pile.name}</div>
                    <div className="text-xs text-slate-400 mt-0.5">{pile.type}</div>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <div className={`w-2 h-2 rounded-full ${s.dot} pulse-dot`}/>
                    <span className={s.badge}>{pile.status}</span>
                  </div>
                </div>

                {pile.status === '使用中' && pile.soc !== null && (
                  <div className="flex items-center gap-4">
                    <SocRing soc={pile.soc}/>
                    <div className="flex-1 text-xs space-y-1.5">
                      <div className="flex justify-between">
                        <span className="text-slate-400">车辆</span>
                        <span className="font-medium text-slate-700">{pile.vehicle}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-slate-400">开始</span>
                        <span className="font-medium text-slate-700">{pile.startTime}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-slate-400">已充</span>
                        <span className="font-medium text-amber-600">{pile.energy} kWh</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-slate-400">费用</span>
                        <span className="font-medium text-emerald-600">¥ {pile.cost}</span>
                      </div>
                    </div>
                  </div>
                )}

                {pile.status === '空闲' && (
                  <div className="flex items-center gap-2 text-xs text-emerald-600">
                    <CheckCircle size={13}/>
                    <span>可用，等待接入</span>
                  </div>
                )}

                {pile.status === '故障' && (
                  <div className="flex items-center gap-2 text-xs text-red-500">
                    <AlertTriangle size={13}/>
                    <span>维修工单已派发</span>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>

      {/* History toggle */}
      <div className="card">
        <button onClick={() => setHistory(h => !h)}
          className="w-full flex items-center justify-between p-4 text-sm font-semibold text-slate-700 hover:bg-slate-50 transition-colors rounded-2xl">
          <div className="flex items-center gap-2"><Clock size={16} className="text-brand-600"/>充电历史记录</div>
          <span className={`text-slate-400 transition-transform text-xs ${history ? 'rotate-180' : ''}`}>▼</span>
        </button>

        {history && (
          <div className="px-4 pb-4 slide-up">
            <div className="overflow-x-auto">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>日期</th><th>车辆</th><th>驾驶员</th><th>充电桩</th><th>充电量</th><th>费用</th><th>时长</th><th>部门</th>
                  </tr>
                </thead>
                <tbody>
                  {chargeHistory.map((h,i) => (
                    <tr key={i}>
                      <td className="text-slate-500">{h.date.slice(5)}</td>
                      <td className="font-medium">{h.plate}</td>
                      <td>{h.driver}</td>
                      <td>{h.pile}</td>
                      <td className="text-amber-600 font-medium">{h.kwh} kWh</td>
                      <td className="text-emerald-600 font-medium">¥ {h.cost}</td>
                      <td>{h.duration}</td>
                      <td><span className="badge-gray">{h.dept}</span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
