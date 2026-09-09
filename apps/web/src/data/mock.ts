import type {
  AiReport,
  DailyUsagePoint,
  DeptPieSlice,
  DeptStat,
  HealthItem,
  MonthlyTrendPoint,
  TopUser,
} from '../types'

// 审批申请与行程记录已接真实接口（src/api/approvals.ts、src/api/trips.ts），此处不再保留 mock。

// ── 费用图表数据 ──────────────────────────────────────
export const monthlyTrend: MonthlyTrendPoint[] = [
  { month:'1月', 销售部:3200, 工程部:4100, 行政部:1800, 财务部:900 },
  { month:'2月', 销售部:2800, 工程部:3600, 行政部:1600, 财务部:800 },
  { month:'3月', 销售部:3800, 工程部:4800, 行政部:2100, 财务部:1100 },
  { month:'4月', 销售部:4100, 工程部:4300, 行政部:1900, 财务部:950 },
  { month:'5月', 销售部:3600, 工程部:5200, 行政部:2200, 财务部:1050 },
  { month:'6月', 销售部:2340, 工程部:3100, 行政部:1450, 财务部:620 },
]
export const deptPie: DeptPieSlice[] = [
  { name:'工程部', value:3100, color:'#1d6fd8' },
  { name:'销售部', value:2340, color:'#10b981' },
  { name:'行政部', value:1450, color:'#f59e0b' },
  { name:'财务部', value:620,  color:'#8b5cf6' },
]
export const dailyUsage: DailyUsagePoint[] = [
  { day:'12日',trips:8,km:312},{day:'13日',trips:11,km:438},{day:'14日',trips:7,km:265},
  { day:'15日',trips:13,km:521},{day:'16日',trips:9,km:347},{day:'17日',trips:10,km:389},{day:'18日',trips:12,km:475},
]
export const deptStats: DeptStat[] = [
  { dept:'工程部', trips:47, km:1876, cost:3100, energy:271, change:+5.2 },
  { dept:'销售部', trips:38, km:1543, cost:2340, energy:222, change:-2.1 },
  { dept:'行政部', trips:29, km:989,  cost:1450, energy:143, change:+8.4 },
  { dept:'财务部', trips:14, km:412,  cost:620,  energy:60,  change:-3.0 },
]
export const top5: TopUser[] = [
  { name:'王工程师', dept:'工程部', trips:18, km:720, cost:890 },
  { name:'张经理',   dept:'销售部', trips:15, km:632, cost:780 },
  { name:'李主任',   dept:'行政部', trips:12, km:498, cost:623 },
  { name:'刘工程师', dept:'工程部', trips:10, km:412, cost:511 },
  { name:'赵专员',   dept:'行政部', trips:9,  km:378, cost:468 },
]

// ── 车辆健康 ──────────────────────────────────────────
export const healthList: HealthItem[] = [
  { id:'V01', plate:'鲁A·A0001', soh:91, score:88, battery:'良好', brake:5,  accel:3,  energy:14.4, issues:[], nextMaint:'2025-07-15', lastCharge:'今日09:00' },
  { id:'V06', plate:'鲁A·A0006', soh:78, score:62, battery:'预警', brake:14, accel:11, energy:17.8, issues:['电池SOH低于80%，建议专项检测','急刹车频次偏高，检查刹车片'], nextMaint:'2025-06-25', lastCharge:'昨日18:30' },
  { id:'V10', plate:'鲁A·A0010', soh:83, score:71, battery:'关注', brake:9,  accel:7,  energy:16.1, issues:['快充次数偏多（月14次），建议控制'], nextMaint:'2025-07-05', lastCharge:'今日08:45' },
  { id:'V04', plate:'鲁A·A0004', soh:85, score:79, battery:'关注', brake:7,  accel:5,  energy:15.3, issues:['里程保养即将到期（剩余约800km）'], nextMaint:'2025-06-28', lastCharge:'昨日20:00' },
  { id:'V08', plate:'鲁A·A0008', soh:95, score:95, battery:'优秀', brake:2,  accel:1,  energy:13.9, issues:[], nextMaint:'2025-08-10', lastCharge:'今日07:30' },
]

export const aiReport: AiReport = {
  plate:'鲁A·A0006',
  score:62,
  level:'较差',
  summary:'本月6号车综合行车状况需重点关注。急刹车次数达14次，远超同型均值（5次），建议尽快检查刹车片磨损情况，并提示驾驶员保持安全跟车距离。电池SOH已降至78%，处于预警区间，按当前衰减趋势预计4个月内需进行电池包专项检测，建议提前安排。此外，本月能耗17.8 kWh/百公里，高于基准14.5 kWh约23%，与急加速频率偏高直接相关。',
  items:[
    { icon:'⚡', label:'电池SOH', value:'78%', status:'warn', tip:'已进入预警区间，建议4个月内专项检测' },
    { icon:'🛑', label:'急刹车', value:'14次', status:'bad',  tip:'超出均值近3倍，检查刹车片' },
    { icon:'🔋', label:'百公里耗电', value:'17.8 kWh', status:'warn', tip:'高于基准23%，与驾驶习惯相关' },
    { icon:'🔧', label:'下次保养', value:'剩余约800km', status:'warn', tip:'建议本月内安排，勿超期' },
  ],
}

