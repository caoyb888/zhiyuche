// ── 车辆数据 ──────────────────────────────────────────
export const vehicles = [
  { id:'V01', plate:'鲁A·A0001', model:'比亚迪 e6', status:'在途', driver:'张经理', soc:74, soh:91, odometer:28430, charge:'正常', location:[113.332,23.121], dept:'销售部', trip:'赴客户公司洽谈' },
  { id:'V02', plate:'鲁A·A0002', model:'比亚迪 e6', status:'充电中', driver:'—',     soc:38, soh:88, odometer:31200, charge:'慢充', location:[113.341,23.118], dept:'—', trip:'—' },
  { id:'V03', plate:'鲁A·A0003', model:'比亚迪 e6', status:'空闲',  driver:'—',     soc:95, soh:93, odometer:19870, charge:'正常', location:[113.338,23.113], dept:'—', trip:'—' },
  { id:'V04', plate:'鲁A·A0004', model:'比亚迪 e6', status:'在途', driver:'李主任', soc:61, soh:85, odometer:45610, charge:'正常', location:[113.318,23.129], dept:'行政部', trip:'接送市局领导' },
  { id:'V05', plate:'鲁A·A0005', model:'比亚迪 e6', status:'空闲',  driver:'—',     soc:82, soh:90, odometer:22100, charge:'正常', location:[113.345,23.107], dept:'—', trip:'—' },
  { id:'V06', plate:'鲁A·A0006', model:'比亚迪 e6', status:'维保中', driver:'—',    soc:55, soh:78, odometer:58200, charge:'注意', location:[113.330,23.115], dept:'—', trip:'定期保养' },
  { id:'V07', plate:'鲁A·A0007', model:'比亚迪 e6', status:'在途', driver:'王工程师',soc:29, soh:92, odometer:17320, charge:'低电警告', location:[113.325,23.132], dept:'工程部', trip:'现场勘查' },
  { id:'V08', plate:'鲁A·A0008', model:'比亚迪 e6', status:'空闲',  driver:'—',     soc:100,soh:95, odometer:8940,  charge:'正常', location:[113.352,23.110], dept:'—', trip:'—' },
  { id:'V09', plate:'鲁A·A0009', model:'比亚迪 e6', status:'空闲',  driver:'—',     soc:88, soh:87, odometer:33500, charge:'正常', location:[113.336,23.120], dept:'—', trip:'—' },
  { id:'V10', plate:'鲁A·A0010', model:'比亚迪 e6', status:'充电中', driver:'—',    soc:12, soh:83, odometer:41800, charge:'快充', location:[113.342,23.116], dept:'—', trip:'—' },
];

export const statusColor = { '在途':'badge-blue','空闲':'badge-green','充电中':'badge-amber','维保中':'badge-red','预约中':'badge-purple' };
export const socColor = soc => soc > 60 ? 'text-emerald-600' : soc > 25 ? 'text-amber-500' : 'text-red-500';

// ── 审批申请 ─────────────────────────────────────────
export const approvals = [
  { id:'ZY-2025-0618-0031', applicant:'陈部长', dept:'行政部', purpose:'公务出行', detail:'赴市政务中心办理年度报告备案', vehicle:'鲁A·A0003', startTime:'2025-06-19 09:00', endTime:'2025-06-19 12:00', dest:'某某市政务中心', route:'某某大道→政务中心路', distance:38, status:'待审批', submitTime:'2025-06-18 16:42', approver:'张总', level:1, urgency:'普通' },
  { id:'ZY-2025-0618-0030', applicant:'刘工程师', dept:'工程部', purpose:'业务出行', detail:'赴某某科技园参加技术对接会议', vehicle:'鲁A·A0005', startTime:'2025-06-19 13:30', endTime:'2025-06-19 17:30', dest:'某某科技园B座', route:'工业路→科技大道→科技园', distance:52, status:'待审批', submitTime:'2025-06-18 15:20', approver:'张总', level:1, urgency:'普通' },
  { id:'ZY-2025-0618-0029', applicant:'王总监', dept:'销售部', purpose:'客户拜访', detail:'拜访某某集团采购部，推进合同签署', vehicle:'鲁A·A0008', startTime:'2025-06-19 10:00', endTime:'2025-06-19 16:00', dest:'某某集团总部', route:'滨江大道→某某路', distance:67, status:'已通过', submitTime:'2025-06-18 14:05', approver:'张总', level:2, urgency:'紧急' },
  { id:'ZY-2025-0618-0028', applicant:'赵专员', dept:'行政部', purpose:'物资采购', detail:'采购办公耗材及劳保用品', vehicle:'鲁A·A0009', startTime:'2025-06-18 14:00', endTime:'2025-06-18 17:00', dest:'某某采购中心', route:'园区路→采购大道', distance:24, status:'已通过', submitTime:'2025-06-18 10:30', approver:'李主任', level:1, urgency:'普通' },
  { id:'ZY-2025-0618-0027', applicant:'孙经理', dept:'财务部', purpose:'公务出行', detail:'赴税务局办理季度税务申报', vehicle:'鲁A·A0003', startTime:'2025-06-18 09:00', endTime:'2025-06-18 11:30', dest:'某某税务局', route:'财务路→税务大道', distance:19, status:'已驳回', submitTime:'2025-06-17 17:00', approver:'李主任', level:1, urgency:'普通', rejectReason:'该时段车辆已被预约，请重新选择时间或车辆' },
];

// ── 行程记录 ──────────────────────────────────────────
export const trips = [
  { id:'T-20250618-047', plate:'鲁A·A0001', driver:'张经理', dept:'销售部', purpose:'公务出行', startTime:'09:05', endTime:'11:48', date:'2025-06-18', distance:47.3, duration:'2小时43分', energy:6.8, cost:46.4, avgSpeed:42, maxSpeed:78, accel:3, brake:5, sign:true, status:'已完成', route:'公司→某某路→客户大厦→返回', routePoints:[[113.338,23.113],[113.298,23.145],[113.265,23.162],[113.280,23.158],[113.338,23.113]] },
  { id:'T-20250618-046', plate:'鲁A·A0004', driver:'李主任', dept:'行政部', purpose:'接送领导', startTime:'08:30', endTime:'10:15', date:'2025-06-18', distance:23.6, duration:'1小时45分', energy:3.4, cost:22.8, avgSpeed:38, maxSpeed:65, accel:1, brake:2, sign:true, status:'已完成', route:'公司→某某宾馆→政府大楼→返回', routePoints:[] },
  { id:'T-20250618-045', plate:'鲁A·A0007', driver:'王工程师', dept:'工程部', purpose:'现场勘查', startTime:'13:00', endTime:'17:20', date:'2025-06-18', distance:61.2, duration:'4小时20分', energy:8.9, cost:59.3, avgSpeed:45, maxSpeed:92, accel:8, brake:11, sign:false, status:'已完成', route:'公司→某某工业园→返回', routePoints:[] },
  { id:'T-20250618-044', plate:'鲁A·A0002', driver:'赵专员', dept:'行政部', purpose:'物资采购', startTime:'14:05', endTime:'16:50', date:'2025-06-18', distance:24.1, duration:'2小时45分', energy:3.5, cost:23.4, avgSpeed:32, maxSpeed:58, accel:2, brake:3, sign:false, status:'已完成', route:'公司→某某采购中心→返回', routePoints:[] },
  { id:'T-20250617-038', plate:'鲁A·A0001', driver:'张经理', dept:'销售部', purpose:'客户拜访', startTime:'09:30', endTime:'12:10', date:'2025-06-17', distance:35.8, duration:'2小时40分', energy:5.2, cost:34.7, avgSpeed:40, maxSpeed:72, accel:4, brake:6, sign:false, status:'已完成', route:'公司→某某广场→返回', routePoints:[] },
];

// ── 费用图表数据 ──────────────────────────────────────
export const monthlyTrend = [
  { month:'1月', 销售部:3200, 工程部:4100, 行政部:1800, 财务部:900 },
  { month:'2月', 销售部:2800, 工程部:3600, 行政部:1600, 财务部:800 },
  { month:'3月', 销售部:3800, 工程部:4800, 行政部:2100, 财务部:1100 },
  { month:'4月', 销售部:4100, 工程部:4300, 行政部:1900, 财务部:950 },
  { month:'5月', 销售部:3600, 工程部:5200, 行政部:2200, 财务部:1050 },
  { month:'6月', 销售部:2340, 工程部:3100, 行政部:1450, 财务部:620 },
];
export const deptPie = [
  { name:'工程部', value:3100, color:'#1d6fd8' },
  { name:'销售部', value:2340, color:'#10b981' },
  { name:'行政部', value:1450, color:'#f59e0b' },
  { name:'财务部', value:620,  color:'#8b5cf6' },
];
export const dailyUsage = [
  { day:'12日',trips:8,km:312},{day:'13日',trips:11,km:438},{day:'14日',trips:7,km:265},
  { day:'15日',trips:13,km:521},{day:'16日',trips:9,km:347},{day:'17日',trips:10,km:389},{day:'18日',trips:12,km:475},
];

// ── 车辆健康 ──────────────────────────────────────────
export const healthList = [
  { id:'V01', plate:'鲁A·A0001', soh:91, score:88, battery:'良好', brake:5,  accel:3,  energy:14.4, issues:[], nextMaint:'2025-07-15', lastCharge:'今日09:00' },
  { id:'V06', plate:'鲁A·A0006', soh:78, score:62, battery:'预警', brake:14, accel:11, energy:17.8, issues:['电池SOH低于80%，建议专项检测','急刹车频次偏高，检查刹车片'], nextMaint:'2025-06-25', lastCharge:'昨日18:30' },
  { id:'V10', plate:'鲁A·A0010', soh:83, score:71, battery:'关注', brake:9,  accel:7,  energy:16.1, issues:['快充次数偏多（月14次），建议控制'], nextMaint:'2025-07-05', lastCharge:'今日08:45' },
  { id:'V04', plate:'鲁A·A0004', soh:85, score:79, battery:'关注', brake:7,  accel:5,  energy:15.3, issues:['里程保养即将到期（剩余约800km）'], nextMaint:'2025-06-28', lastCharge:'昨日20:00' },
  { id:'V08', plate:'鲁A·A0008', soh:95, score:95, battery:'优秀', brake:2,  accel:1,  energy:13.9, issues:[], nextMaint:'2025-08-10', lastCharge:'今日07:30' },
];

export const aiReport = {
  plate:'鲁A·A0006',
  score:62,
  level:'较差',
  summary:'本月6号车综合行车状况需重点关注。急刹车次数达14次，远超同型均值（5次），建议尽快检查刹车片磨损情况，并提示驾驶员保持安全跟车距离。电池SOH已降至78%，处于预警区间，按当前衰减趋势预计4个月内需进行电池包专项检测，建议提前安排。此外，本月能耗17.8 kWh/百公里，高于基准14.5 kWh约23%，与急加速频率偏高直接相关。',
  items:[
    { icon:'⚡', label:'电池SOH', value:'78%', status:'warn', tip:'已进入预警区间，建议4个月内专项检测' },
    { icon:'🛑', label:'急刹车', value:'14次', status:'bad',  tip:'超出均值近3倍，检查刹车片' },
    { icon:'🔋', label:'百公里耗电', value:'17.8 kWh', status:'warn', tip:'高于基准23%，与驾驶习惯相关' },
    { icon:'🔧', label:'下次保养', value:'剩余约800km', status:'warn', tip:'建议本月内安排，勿超期' },
  ]
};

// ── 充电桩 ───────────────────────────────────────────
export const chargers = [
  { id:'CP-01', name:'1号快充桩', type:'快充 60kW', status:'使用中', vehicle:'鲁A·A0010', soc:12, startTime:'08:45', energy:9.2, cost:8.7 },
  { id:'CP-02', name:'2号快充桩', type:'快充 60kW', status:'空闲', vehicle:'—', soc:null, startTime:'—', energy:0, cost:0 },
  { id:'CP-03', name:'3号慢充桩', type:'慢充 7kW',  status:'使用中', vehicle:'鲁A·A0002', soc:38, startTime:'07:30', energy:8.4, cost:5.5 },
  { id:'CP-04', name:'4号慢充桩', type:'慢充 7kW',  status:'空闲', vehicle:'—', soc:null, startTime:'—', energy:0, cost:0 },
  { id:'CP-05', name:'5号慢充桩', type:'慢充 7kW',  status:'故障', vehicle:'—', soc:null, startTime:'—', energy:0, cost:0 },
  { id:'CP-06', name:'6号慢充桩', type:'慢充 7kW',  status:'空闲', vehicle:'—', soc:null, startTime:'—', energy:0, cost:0 },
];

export const chargeHistory = [
  { date:'2025-06-18', plate:'鲁A·A0010', driver:'周专员', pile:'1号快充桩', kwh:28.4, cost:18.5, duration:'42分钟', dept:'工程部' },
  { date:'2025-06-18', plate:'鲁A·A0002', driver:'赵专员', pile:'3号慢充桩', kwh:21.7, cost:14.1, duration:'3.1小时', dept:'行政部' },
  { date:'2025-06-17', plate:'鲁A·A0006', driver:'运营', pile:'2号快充桩', kwh:35.2, cost:22.9, duration:'58分钟', dept:'车队' },
  { date:'2025-06-17', plate:'鲁A·A0003', driver:'张经理', pile:'3号慢充桩', kwh:18.6, cost:12.1, duration:'2.7小时', dept:'销售部' },
];
