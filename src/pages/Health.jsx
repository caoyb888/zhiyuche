import { useState } from 'react'
import { healthList, aiReport } from '../data/mock'
import { HeartPulse, AlertTriangle, CheckCircle, Cpu, ChevronRight, Wrench } from 'lucide-react'

const scoreColor = s => s >= 85 ? '#10b981' : s >= 70 ? '#f59e0b' : '#ef4444'
const scoreLabel = s => s >= 85 ? '优秀' : s >= 70 ? '良好' : '较差'
const sohBadge = b => ({ '优秀':'badge-green','良好':'badge-green','关注':'badge-amber','预警':'badge-red' })[b] || 'badge-gray'

function SohBar({ value }) {
  const color = value >= 90 ? '#10b981' : value >= 80 ? '#f59e0b' : '#ef4444'
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-2 bg-slate-100 rounded-full overflow-hidden">
        <div className="h-full rounded-full transition-all" style={{ width:`${value}%`, background:color }}/>
      </div>
      <span className="text-xs font-mono font-medium" style={{color}}>{value}%</span>
    </div>
  )
}

function AIReportModal({ onClose }) {
  const r = aiReport
  return (
    <div className="fixed inset-0 z-50 flex items-end md:items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div className="bg-white rounded-2xl w-full max-w-md max-h-[90vh] overflow-y-auto shadow-2xl slide-up" onClick={e=>e.stopPropagation()}>
        {/* Header */}
        <div className="bg-gradient-to-r from-brand-900 to-brand-700 p-5 rounded-t-2xl">
          <div className="flex items-center gap-2 mb-3">
            <div className="w-6 h-6 rounded-full bg-white/20 flex items-center justify-center"><Cpu size={13} className="text-white"/></div>
            <span className="text-white/80 text-xs font-medium">智御 AI 诊断报告</span>
          </div>
          <div className="text-white font-semibold">{r.plate} · 本月健康诊断</div>
          <div className="flex items-center gap-3 mt-3">
            <div className="text-4xl font-bold text-white">{r.score}</div>
            <div>
              <div className="text-white/60 text-xs">综合评分</div>
              <div className="text-amber-300 text-sm font-medium">{r.level} ⚠</div>
            </div>
          </div>
        </div>

        <div className="p-5 space-y-4">
          {/* Summary */}
          <div className="bg-slate-50 rounded-xl p-4 text-sm text-slate-700 leading-relaxed">
            {r.summary}
          </div>

          {/* Item cards */}
          <div className="grid grid-cols-2 gap-3">
            {r.items.map(item => (
              <div key={item.label} className={`p-3 rounded-xl border ${item.status==='bad'?'border-red-100 bg-red-50':item.status==='warn'?'border-amber-100 bg-amber-50':'border-emerald-100 bg-emerald-50'}`}>
                <div className="text-lg mb-1">{item.icon}</div>
                <div className={`text-xs font-medium mb-0.5 ${item.status==='bad'?'text-red-600':item.status==='warn'?'text-amber-600':'text-emerald-600'}`}>{item.label}</div>
                <div className={`text-base font-bold ${item.status==='bad'?'text-red-700':item.status==='warn'?'text-amber-700':'text-emerald-700'}`}>{item.value}</div>
                <div className="text-[11px] text-slate-500 mt-1 leading-snug">{item.tip}</div>
              </div>
            ))}
          </div>

          <button onClick={onClose} className="w-full py-3 bg-brand-600 text-white rounded-xl text-sm font-medium hover:bg-brand-700">关闭报告</button>
        </div>
      </div>
    </div>
  )
}

export default function HealthPage() {
  const [showAI, setShowAI] = useState(false)
  const [selected, setSelected] = useState(null)

  const warn = healthList.filter(h => h.issues.length > 0).length

  return (
    <div className="space-y-4 slide-up">
      {/* Banner */}
      {warn > 0 && (
        <div className="bg-amber-50 border border-amber-100 rounded-2xl p-4 flex items-center gap-3">
          <AlertTriangle size={20} className="text-amber-500 shrink-0"/>
          <div>
            <div className="text-sm font-semibold text-amber-800">共 {warn} 辆车需要关注</div>
            <div className="text-xs text-amber-600 mt-0.5">请及时安排维保，防止非计划停车</div>
          </div>
          <button onClick={() => setShowAI(true)} className="ml-auto shrink-0 flex items-center gap-1.5 bg-brand-600 text-white text-xs px-3 py-2 rounded-xl hover:bg-brand-700">
            <Cpu size={12}/> AI 诊断
          </button>
        </div>
      )}

      {/* Vehicle health cards */}
      <div className="space-y-3">
        {healthList.map(h => (
          <div key={h.id} onClick={() => setSelected(selected===h.id ? null : h.id)}
            className="card p-4 cursor-pointer hover:shadow-md transition-all">
            <div className="flex items-start gap-3">
              {/* Score circle */}
              <div className="w-12 h-12 rounded-xl flex flex-col items-center justify-center border-2 shrink-0"
                style={{borderColor: scoreColor(h.score)}}>
                <div className="text-base font-bold leading-none" style={{color: scoreColor(h.score)}}>{h.score}</div>
                <div className="text-[9px] mt-0.5" style={{color: scoreColor(h.score)}}>{scoreLabel(h.score)}</div>
              </div>

              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-semibold text-slate-800">{h.plate}</span>
                  <span className={sohBadge(h.battery)}>{h.battery}</span>
                </div>
                <div className="mt-1.5 text-xs text-slate-400 mb-2">电池健康度 SOH</div>
                <SohBar value={h.soh}/>
                {h.issues.length > 0 && (
                  <div className="mt-2 space-y-1">
                    {h.issues.map((iss,i) => (
                      <div key={i} className="flex items-start gap-1.5 text-xs text-amber-700">
                        <AlertTriangle size={11} className="shrink-0 mt-0.5"/>
                        <span>{iss}</span>
                      </div>
                    ))}
                  </div>
                )}
                {h.issues.length === 0 && (
                  <div className="flex items-center gap-1 text-xs text-emerald-600 mt-1.5">
                    <CheckCircle size={11}/> 暂无异常，运行良好
                  </div>
                )}
              </div>
              <ChevronRight size={15} className={`text-slate-300 shrink-0 mt-1 transition-transform ${selected===h.id?'rotate-90':''}`}/>
            </div>

            {/* Expanded */}
            {selected === h.id && (
              <div className="mt-4 pt-4 border-t border-slate-100 grid grid-cols-2 md:grid-cols-4 gap-3 slide-up">
                {[['急刹车', `${h.brake}次`, h.brake>8?'text-red-500':'text-slate-700'],
                  ['急加速', `${h.accel}次`, h.accel>6?'text-amber-500':'text-slate-700'],
                  ['百公里耗电', `${h.energy}kWh`, h.energy>16?'text-amber-500':'text-emerald-600'],
                  ['下次保养', h.nextMaint, 'text-slate-700'],
                ].map(([l,v,c]) => (
                  <div key={l} className="bg-slate-50 rounded-xl p-3">
                    <div className="text-xs text-slate-400 mb-1">{l}</div>
                    <div className={`text-sm font-semibold ${c}`}>{v}</div>
                  </div>
                ))}
                <div className="col-span-2 md:col-span-4 flex gap-2 mt-1">
                  <button onClick={e=>{e.stopPropagation();setShowAI(true)}}
                    className="flex items-center gap-1.5 bg-brand-50 text-brand-700 text-xs px-3 py-2 rounded-xl hover:bg-brand-100 border border-brand-100">
                    <Cpu size={12}/> 查看 AI 诊断
                  </button>
                  <button className="flex items-center gap-1.5 bg-slate-50 text-slate-600 text-xs px-3 py-2 rounded-xl hover:bg-slate-100 border border-slate-200">
                    <Wrench size={12}/> 创建维保工单
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      {showAI && <AIReportModal onClose={() => setShowAI(false)}/>}
    </div>
  )
}
