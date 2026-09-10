import { useState, type MouseEvent } from 'react'
import { healthList, aiReport } from '../data/mock'
import type { AiItemStatus, BadgeClass, BatteryGrade, ScoreLevel } from '../types'
import { AlertTriangle, CheckCircle, Cpu, ChevronRight, Wrench } from 'lucide-react'

const scoreColor = (s: number): string => s >= 85 ? '#10b981' : s >= 70 ? '#f0a32b' : '#ef4b3c'
const scoreLabel = (s: number): ScoreLevel => s >= 85 ? '优秀' : s >= 70 ? '良好' : '较差'

const batteryBadge: Record<BatteryGrade, BadgeClass> = {
  '优秀': 'badge-green',
  '良好': 'badge-green',
  '关注': 'badge-amber',
  '预警': 'badge-red',
}

const aiItemStyle: Record<AiItemStatus, { box: string; label: string; value: string }> = {
  bad:  { box:'border-danger-500/30 bg-danger-500/10',         label:'text-danger-200',     value:'text-danger-200' },
  warn: { box:'border-warn-500/30 bg-warn-500/10',     label:'text-warn-200',   value:'text-warn-200' },
  good: { box:'border-ev-500/30 bg-ev-500/10', label:'text-ev-200', value:'text-ev-200' },
}

function SohBar({ value }: { value: number }) {
  const color = value >= 90 ? '#10b981' : value >= 80 ? '#f0a32b' : '#ef4b3c'
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-2 bg-surface-4 rounded-full overflow-hidden">
        <div className="h-full rounded-full transition-all" style={{ width:`${value}%`, background:color }}/>
      </div>
      <span className="text-xs font-mono font-medium" style={{color}}>{value}%</span>
    </div>
  )
}

function AIReportModal({ onClose }: { onClose: () => void }) {
  const r = aiReport
  return (
    <div className="fixed inset-0 z-50 flex items-end md:items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div className="bg-surface-2 rounded-2xl w-full max-w-md max-h-[90vh] overflow-y-auto shadow-2xl slide-up" onClick={e=>e.stopPropagation()}>
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
              <div className="text-warn-200 text-sm font-medium">{r.level} ⚠</div>
            </div>
          </div>
        </div>

        <div className="p-5 space-y-4">
          {/* Summary */}
          <div className="bg-surface-3 rounded-xl p-4 text-sm text-ink leading-relaxed">
            {r.summary}
          </div>

          {/* Item cards */}
          <div className="grid grid-cols-2 gap-3">
            {r.items.map(item => {
              const st = aiItemStyle[item.status]
              return (
                <div key={item.label} className={`p-3 rounded-xl border ${st.box}`}>
                  <div className="text-lg mb-1">{item.icon}</div>
                  <div className={`text-xs font-medium mb-0.5 ${st.label}`}>{item.label}</div>
                  <div className={`text-base font-bold ${st.value}`}>{item.value}</div>
                  <div className="text-[11px] text-ink-muted mt-1 leading-snug">{item.tip}</div>
                </div>
              )
            })}
          </div>

          <button type="button" onClick={onClose} className="w-full py-3 bg-brand-600 text-white rounded-xl text-sm font-medium hover:bg-brand-700">关闭报告</button>
        </div>
      </div>
    </div>
  )
}

export default function HealthPage() {
  const [showAI, setShowAI] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)

  const warn = healthList.filter(h => h.issues.length > 0).length

  const openAI = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation()
    setShowAI(true)
  }

  return (
    <div className="space-y-4 slide-up">
      {/* Banner */}
      {warn > 0 && (
        <div className="bg-warn-500/10 border border-warn-500/30 rounded-2xl p-4 flex items-center gap-3">
          <AlertTriangle size={20} className="text-warn-200 shrink-0"/>
          <div>
            <div className="text-sm font-semibold text-warn-200">共 {warn} 辆车需要关注</div>
            <div className="text-xs text-warn-200 mt-0.5">请及时安排维保，防止非计划停车</div>
          </div>
          <button type="button" onClick={() => setShowAI(true)} className="ml-auto shrink-0 flex items-center gap-1.5 bg-brand-600 text-white text-xs px-3 py-2 rounded-xl hover:bg-brand-700">
            <Cpu size={12}/> AI 诊断
          </button>
        </div>
      )}

      {/* Vehicle health cards */}
      <div className="space-y-3">
        {healthList.map(h => {
          const expandedCells: Array<[label: string, value: string, color: string]> = [
            ['急刹车', `${h.brake}次`, h.brake>8?'text-danger-200':'text-ink'],
            ['急加速', `${h.accel}次`, h.accel>6?'text-warn-200':'text-ink'],
            ['百公里耗电', `${h.energy}kWh`, h.energy>16?'text-warn-200':'text-ev-200'],
            ['下次保养', h.nextMaint, 'text-ink'],
          ]
          return (
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
                    <span className="text-sm font-semibold text-ink-strong">{h.plate}</span>
                    <span className={batteryBadge[h.battery]}>{h.battery}</span>
                  </div>
                  <div className="mt-1.5 text-xs text-ink-faint mb-2">电池健康度 SOH</div>
                  <SohBar value={h.soh}/>
                  {h.issues.length > 0 && (
                    <div className="mt-2 space-y-1">
                      {h.issues.map((iss,i) => (
                        <div key={i} className="flex items-start gap-1.5 text-xs text-warn-200">
                          <AlertTriangle size={11} className="shrink-0 mt-0.5"/>
                          <span>{iss}</span>
                        </div>
                      ))}
                    </div>
                  )}
                  {h.issues.length === 0 && (
                    <div className="flex items-center gap-1 text-xs text-ev-200 mt-1.5">
                      <CheckCircle size={11}/> 暂无异常，运行良好
                    </div>
                  )}
                </div>
                <ChevronRight size={15} className={`text-ink-disabled shrink-0 mt-1 transition-transform ${selected===h.id?'rotate-90':''}`}/>
              </div>

              {/* Expanded */}
              {selected === h.id && (
                <div className="mt-4 pt-4 border-t border-line grid grid-cols-2 md:grid-cols-4 gap-3 slide-up">
                  {expandedCells.map(([l,v,c]) => (
                    <div key={l} className="bg-surface-3 rounded-xl p-3">
                      <div className="text-xs text-ink-faint mb-1">{l}</div>
                      <div className={`text-sm font-semibold ${c}`}>{v}</div>
                    </div>
                  ))}
                  <div className="col-span-2 md:col-span-4 flex gap-2 mt-1">
                    <button type="button" onClick={openAI}
                      className="flex items-center gap-1.5 bg-brand-600/10 text-brand-300 text-xs px-3 py-2 rounded-xl hover:bg-brand-600/20 border border-brand-600/30">
                      <Cpu size={12}/> 查看 AI 诊断
                    </button>
                    <button type="button" onClick={e => e.stopPropagation()}
                      className="flex items-center gap-1.5 bg-surface-3 text-ink text-xs px-3 py-2 rounded-xl hover:bg-surface-4 border border-line-strong">
                      <Wrench size={12}/> 创建维保工单
                    </button>
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {showAI && <AIReportModal onClose={() => setShowAI(false)}/>}
    </div>
  )
}
