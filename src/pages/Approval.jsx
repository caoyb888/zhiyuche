import { useState } from 'react'
import { approvals } from '../data/mock'
import { CheckCircle, XCircle, Clock, ChevronRight, FileText, MapPin, Calendar, User } from 'lucide-react'

const statusMap = {
  '待审批': { cls:'badge-amber', icon:Clock },
  '已通过': { cls:'badge-green', icon:CheckCircle },
  '已驳回': { cls:'badge-red',   icon:XCircle },
}

function ApprovalDetail({ item, onClose, onApprove, onReject }) {
  const [rejectNote, setRejectNote] = useState('')
  const [showReject, setShowReject] = useState(false)
  const [signOn, setSignOn] = useState(false)

  return (
    <div className="fixed inset-0 z-50 flex items-end md:items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div className="bg-white rounded-2xl w-full max-w-lg max-h-[90vh] overflow-y-auto shadow-2xl slide-up" onClick={e=>e.stopPropagation()}>
        <div className="p-5 border-b border-slate-100 flex items-center justify-between">
          <div>
            <div className="font-semibold text-slate-800">{item.purpose}</div>
            <div className="text-xs text-slate-400 mt-0.5">{item.id}</div>
          </div>
          <span className={statusMap[item.status].cls}>{item.status}</span>
        </div>

        <div className="p-5 space-y-4">
          {/* Applicant */}
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-brand-100 flex items-center justify-center font-semibold text-brand-700 text-sm">{item.applicant[0]}</div>
            <div>
              <div className="text-sm font-medium text-slate-800">{item.applicant}</div>
              <div className="text-xs text-slate-400">{item.dept} · 提交于 {item.submitTime}</div>
            </div>
            <span className={item.urgency==='紧急'?'badge-red ml-auto':'badge-gray ml-auto'}>{item.urgency}</span>
          </div>

          {/* Detail grid */}
          <div className="grid grid-cols-2 gap-3">
            {[
              ['用车事由', item.detail],
              ['指派车辆', item.vehicle],
              ['取车时间', item.startTime],
              ['还车时间', item.endTime],
              ['目的地',   item.dest],
              ['预计里程', `约 ${item.distance} km`],
            ].map(([l,v])=>(
              <div key={l} className={`${l==='用车事由'||l==='目的地'?'col-span-2':''}`}>
                <div className="text-xs text-slate-400 mb-1">{l}</div>
                <div className="text-sm text-slate-700 font-medium">{v}</div>
              </div>
            ))}
          </div>

          {/* Route */}
          <div className="bg-slate-50 rounded-xl p-3">
            <div className="text-xs text-slate-400 mb-2 flex items-center gap-1"><MapPin size={11}/>计划路线</div>
            <div className="text-sm text-slate-700">{item.route}</div>
          </div>

          {/* Roof sign preview */}
          {item.status !== '已驳回' && (
            <div className="bg-brand-900 rounded-xl p-4">
              <div className="text-xs text-slate-400 mb-3">车顶灯牌预览（审批通过后自动亮起）</div>
              <div className="flex items-center justify-center">
                <button onClick={() => setSignOn(p=>!p)}
                  className={`px-8 py-3 rounded-lg text-white font-bold tracking-widest text-base border-2 transition-all cursor-pointer select-none ${signOn ? 'border-blue-400 bg-blue-900 sign-glow' : 'border-slate-600 bg-slate-800 opacity-50'}`}>
                  市中公务用车
                </button>
              </div>
              <div className="text-center text-xs text-slate-500 mt-2">点击模拟灯牌亮起效果</div>
            </div>
          )}

          {/* Reject reason */}
          {item.status === '已驳回' && item.rejectReason && (
            <div className="bg-red-50 rounded-xl p-3 border border-red-100">
              <div className="text-xs text-red-500 font-medium mb-1">驳回原因</div>
              <div className="text-sm text-red-700">{item.rejectReason}</div>
            </div>
          )}

          {/* Actions */}
          {item.status === '待审批' && !showReject && (
            <div className="flex gap-3 pt-2">
              <button onClick={() => setShowReject(true)} className="flex-1 py-2.5 border border-red-200 text-red-600 rounded-xl text-sm font-medium hover:bg-red-50 transition-colors">驳回</button>
              <button onClick={onApprove} className="flex-1 py-2.5 bg-brand-600 text-white rounded-xl text-sm font-medium hover:bg-brand-700 transition-colors">审批通过</button>
            </div>
          )}

          {showReject && (
            <div className="space-y-3">
              <textarea value={rejectNote} onChange={e=>setRejectNote(e.target.value)}
                placeholder="请填写驳回原因…" rows={3}
                className="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm resize-none focus:outline-none focus:ring-2 focus:ring-brand-200" />
              <div className="flex gap-3">
                <button onClick={() => setShowReject(false)} className="flex-1 py-2.5 border border-slate-200 text-slate-600 rounded-xl text-sm hover:bg-slate-50">取消</button>
                <button onClick={() => onReject(rejectNote)} className="flex-1 py-2.5 bg-red-500 text-white rounded-xl text-sm font-medium hover:bg-red-600">确认驳回</button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default function ApprovalPage() {
  const [list, setList] = useState(approvals)
  const [selected, setSelected] = useState(null)
  const [filter, setFilter] = useState('全部')
  const [showForm, setShowForm] = useState(false)

  const tabs = ['全部','待审批','已通过','已驳回']
  const filtered = filter === '全部' ? list : list.filter(a => a.status === filter)
  const pending = list.filter(a => a.status === '待审批').length

  const handleApprove = id => {
    setList(l => l.map(a => a.id === id ? {...a, status:'已通过'} : a))
    setSelected(null)
  }
  const handleReject = (id, reason) => {
    setList(l => l.map(a => a.id === id ? {...a, status:'已驳回', rejectReason: reason||'不符合用车规定'} : a))
    setSelected(null)
  }

  return (
    <div className="space-y-4 slide-up">
      {/* Header action */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {pending > 0 && (
            <div className="flex items-center gap-1.5 bg-amber-50 text-amber-700 text-sm px-3 py-1.5 rounded-full border border-amber-100">
              <Clock size={14}/>
              <span>{pending} 条待审批</span>
            </div>
          )}
        </div>
        <button onClick={() => setShowForm(true)}
          className="bg-brand-600 text-white text-sm px-4 py-2 rounded-xl hover:bg-brand-700 transition-colors flex items-center gap-2">
          <FileText size={15}/> 发起申请
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-slate-100 p-1 rounded-xl w-fit">
        {tabs.map(t => (
          <button key={t} onClick={() => setFilter(t)}
            className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-all ${filter===t ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-700'}`}>
            {t}{t==='待审批' && pending > 0 ? ` (${pending})` : ''}
          </button>
        ))}
      </div>

      {/* List */}
      <div className="space-y-2.5">
        {filtered.map(item => {
          const S = statusMap[item.status]
          return (
            <div key={item.id} onClick={() => setSelected(item)}
              className="card p-4 cursor-pointer hover:shadow-md transition-all hover:-translate-y-0.5">
              <div className="flex items-start gap-3">
                <div className="w-9 h-9 rounded-xl bg-brand-50 flex items-center justify-center shrink-0 mt-0.5">
                  <FileText size={17} className="text-brand-600"/>
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-semibold text-slate-800 truncate">{item.purpose} · {item.detail.slice(0,18)}…</span>
                    <span className={S.cls + ' shrink-0'}>{item.status}</span>
                  </div>
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 mt-1.5 text-xs text-slate-400">
                    <span className="flex items-center gap-1"><User size={11}/>{item.applicant} · {item.dept}</span>
                    <span className="flex items-center gap-1"><Calendar size={11}/>{item.startTime}</span>
                    <span className="flex items-center gap-1"><MapPin size={11}/>{item.dest}</span>
                  </div>
                </div>
                <ChevronRight size={16} className="text-slate-300 shrink-0 mt-2"/>
              </div>
            </div>
          )
        })}
      </div>

      {/* Apply form modal (simplified) */}
      {showForm && (
        <div className="fixed inset-0 z-50 flex items-end md:items-center justify-center bg-black/40 p-4" onClick={() => setShowForm(false)}>
          <div className="bg-white rounded-2xl w-full max-w-md shadow-2xl slide-up" onClick={e=>e.stopPropagation()}>
            <div className="p-5 border-b border-slate-100">
              <div className="font-semibold text-slate-800">发起用车申请</div>
              <div className="text-xs text-slate-400 mt-0.5">公务用车须经审批方可使用</div>
            </div>
            <div className="p-5 space-y-4">
              {[
                { label:'用车事由', type:'select', opts:['公务出行','客户拜访','接送领导','物资采购','业务出行'] },
                { label:'具体说明', type:'textarea' },
                { label:'目的地', type:'text', ph:'如：某某政务中心' },
                { label:'取车时间', type:'datetime-local' },
                { label:'还车时间', type:'datetime-local' },
              ].map(f => (
                <div key={f.label}>
                  <label className="text-xs text-slate-500 mb-1.5 block">{f.label}</label>
                  {f.type === 'select' ? (
                    <select className="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-200">
                      {f.opts.map(o => <option key={o}>{o}</option>)}
                    </select>
                  ) : f.type === 'textarea' ? (
                    <textarea rows={2} placeholder="请描述具体用途…" className="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm resize-none focus:outline-none focus:ring-2 focus:ring-brand-200"/>
                  ) : (
                    <input type={f.type} placeholder={f.ph} className="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-200"/>
                  )}
                </div>
              ))}
              <div className="flex gap-3 pt-1">
                <button onClick={() => setShowForm(false)} className="flex-1 py-2.5 border border-slate-200 rounded-xl text-sm text-slate-600 hover:bg-slate-50">取消</button>
                <button onClick={() => setShowForm(false)} className="flex-1 py-2.5 bg-brand-600 text-white rounded-xl text-sm font-medium hover:bg-brand-700">提交申请</button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Detail modal */}
      {selected && (
        <ApprovalDetail
          item={selected}
          onClose={() => setSelected(null)}
          onApprove={() => handleApprove(selected.id)}
          onReject={(reason) => handleReject(selected.id, reason)}
        />
      )}
    </div>
  )
}
