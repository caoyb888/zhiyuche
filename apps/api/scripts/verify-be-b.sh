#!/usr/bin/env bash
# 端到端验证：审批工作流 / 行程 / 总览（阶段 2 BE-B）。
#
# 前置：api 已启动并完成引导（admin / chenhua_admin，密码 Admin@123456），迁移 00003 已应用。
# 用法：BASE=http://localhost:20091/api/v1 PSQL="docker exec -i zhiyuche-postgres psql -U zhiyuche -d zhiyuche_be2 -tAq" \
#       bash apps/api/scripts/verify-be-b.sh
# 资产接口由另一工作包实现，车辆/实时状态/轨迹点等前置数据用 PSQL 直接写入（PSQL 必需）。
# 需要 curl 与 python3。
set -u
BASE=${BASE:-http://localhost:20091/api/v1}
PSQL=${PSQL:-}
if [ -z "$PSQL" ]; then echo "PSQL is required"; exit 2; fi
SFX=$(date +%s | tail -c 6)$RANDOM
TMP=${TMPDIR:-/tmp}/zy-beb-$$
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "  ok   $1"; }
bad() { FAIL=$((FAIL+1)); echo "  FAIL $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want [$2], got [$3])"; fi; }
contains() { case "$3" in *"$2"*) ok "$1";; *) bad "$1 (want contains [$2], got [$3])";; esac; }
section() { echo; echo "== $1"; }

# req METHOD PATH TOKEN [BODY] [EXTRA_HEADER] → STATUS / BODY
req() {
  local m=$1 p=$2 t=$3 b=${4:-} h=${5:-}
  local args=(-s -o "$TMP/body" -w '%{http_code}' -X "$m" -H "Authorization: Bearer $t" -H 'Content-Type: application/json')
  [ -n "$h" ] && args+=(-H "$h")
  [ -n "$b" ] && args+=(--data "$b")
  STATUS=$(curl "${args[@]}" "$BASE$p")
  BODY=$(cat "$TMP/body")
}
j() {
  printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k == "": continue
    if isinstance(d, list): d = d[int(k)] if int(k) < len(d) else None
    elif isinstance(d, dict): d = d.get(k)
    else: d = None
if d is None: print("null")
elif isinstance(d, bool): print(str(d).lower())
elif isinstance(d, (dict, list)): print(json.dumps(d, ensure_ascii=False))
else: print(d)
' "$1"
}
jlen() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d[int(k)] if isinstance(d, list) else (d.get(k) if isinstance(d, dict) else None)
print(len(d) if d is not None else 0)' "$1"; }
jfind() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d.get(k) if isinstance(d, dict) else None
for it in d or []:
    if str(it.get(sys.argv[2])) == sys.argv[3]:
        v = it.get(sys.argv[4]); print("null" if v is None else (str(v).lower() if isinstance(v, bool) else v)); sys.exit(0)
print("__missing__")' "$1" "$2" "$3" "$4"; }
# jhas ARRAYPATH FIELD VALUE → true/false
jhas() { [ "$(jfind "$1" "$2" "$3" "$2")" != "__missing__" ] && echo true || echo false; }
jall() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d.get(k) if isinstance(d, dict) else None
print("true" if d and all(str(it.get(sys.argv[2])) == sys.argv[3] for it in d) else "false")' "$1" "$2" "$3"; }
sql() { printf '%s' "$1" | $PSQL | head -n1; }
xlsx_rows() { python3 - "$1" <<'PY'
import sys, zipfile, re
z = zipfile.ZipFile(sys.argv[1])
sheet = [n for n in z.namelist() if n.startswith("xl/worksheets/sheet")][0]
print(len(re.findall(r"<row[ >]", z.read(sheet).decode())) - 1)
PY
}
utc() { date -u -d "$1" +%Y-%m-%dT%H:%M:%SZ; }
D=$(date -u -d '+1 day' +%Y-%m-%d)   # 明天（UTC 日期），时段均以 UTC 给出：CST = UTC+8

# ------------------------------------------------------------------ login & setup
section "登录与前置数据"
req POST /auth/login "" '{"username":"admin","password":"Admin@123456"}'
SUPER=$(j data.access_token); check "超级管理员登录" 200 "$STATUS"
req POST /auth/login "" '{"username":"chenhua_admin","password":"Admin@123456","tenant_code":"chenhua"}'
ADMIN=$(j data.access_token); check "租户管理员登录" 200 "$STATUS"
req GET /auth/me "$ADMIN"; CH_TID=$(j data.tenant.id); ADMIN_UID=$(j data.id)

# 阶段 1 库里的内置 employee/approver 角色可能尚未补齐 approval:*/trip:* 权限码，脚本自建等价角色
req POST /system/roles "$ADMIN" "{\"code\":\"e_$SFX\",\"name\":\"员工T\",\"permissions\":[\"dashboard:view\",\"approval:view\",\"approval:create\",\"trip:view\"]}"
R_EMP=$(j data.id); check "创建员工测试角色" 201 "$STATUS"
req POST /system/roles "$ADMIN" "{\"code\":\"a_$SFX\",\"name\":\"审批T\",\"permissions\":[\"dashboard:view\",\"approval:view\",\"approval:approve\",\"approval:create\",\"trip:view\",\"asset:vehicle:view\"]}"
R_APR=$(j data.id); check "创建审批人测试角色（approver 回退用 fallback_approver_role=a_$SFX）" 201 "$STATUS"

req POST /system/depts "$ADMIN" "{\"name\":\"研发部_$SFX\"}";  DEPT_A=$(j data.id); check "创建部门 A" 201 "$STATUS"
req POST /system/depts "$ADMIN" "{\"name\":\"车队部_$SFX\"}";  DEPT_B=$(j data.id); check "创建部门 B" 201 "$STATUS"
PW='Emp@123456'
mkuser() { # name dept role → id
  req POST /system/users "$ADMIN" "{\"username\":\"$1_$SFX\",\"name\":\"$1\",\"password\":\"$PW\",\"dept_id\":\"$2\",\"role_ids\":[\"$3\"]}"
  [ "$STATUS" = 201 ] || echo "  create user $1 failed: $BODY" >&2
  j data.id
}
U_EMP=$(mkuser emp "$DEPT_A" "$R_EMP")
U_EMP2=$(mkuser emp2 "$DEPT_A" "$R_EMP")
U_LEAD=$(mkuser lead "$DEPT_A" "$R_APR")
U_L2=$(mkuser l2 "$DEPT_B" "$R_APR")
req PUT "/system/depts/$DEPT_A" "$ADMIN" "{\"leader_user_id\":\"$U_LEAD\"}"; check "部门 A 负责人 = lead" 200 "$STATUS"
login() { req POST /auth/login "" "{\"username\":\"$1_$SFX\",\"password\":\"$PW\",\"tenant_code\":\"chenhua\"}"; j data.access_token; }
EMP=$(login emp); EMP2=$(login emp2); LEAD=$(login lead); L2=$(login l2)
check "员工/审批人登录" true "$([ -n "$EMP" ] && [ -n "$LEAD" ] && [ -n "$L2" ] && [ "$EMP" != null ] && echo true)"

mkveh() { # plate status home_dept → id
  local id
  id=$(sql "INSERT INTO vehicles (tenant_id, plate_no, battery_kwh, status, odometer_km, home_dept_id) VALUES ('$CH_TID', '$1', 60, '$2', 1000, $3) RETURNING id;")
  sql "INSERT INTO vehicle_status (vehicle_id, tenant_id, status, odometer_km, soc, range_km, lng, lat, last_telemetry_at) VALUES ('$id', '$CH_TID', '$2', 1000, 80, 320, 117.0, 36.6, now());" >/dev/null
  echo "$id"
}
V1=$(mkveh "鲁B$SFX" idle "'$DEPT_B'")       # 车队部车辆 → 对研发部员工是跨部门
V2=$(mkveh "鲁A$SFX" idle "'$DEPT_A'")
V3=$(mkveh "鲁M$SFX" maintenance NULL)
check "车辆写入" true "$([ -n "$V1" ] && [ -n "$V2" ] && [ -n "$V3" ] && echo true)"
echo "  tenant=$CH_TID emp=$U_EMP lead=$U_LEAD l2=$U_L2 V1=$V1 V2=$V2"

# ------------------------------------------------------------------ rules
section "审批规则 rules"
sql "DELETE FROM approval_rules WHERE tenant_id = '$CH_TID';" >/dev/null   # 保证"无规则行"起点（清掉被中断的上次运行残留）
req GET /approvals/rules "$ADMIN"
check "GET 规则（无行时返回默认）" 200 "$STATUS"
check "  updated_at=null" null "$(j data.updated_at)"
check "  默认 night 22:00-06:00 fallback approver" "22:00 06:00 approver true" "$(j data.night_start) $(j data.night_end) $(j data.fallback_approver_role) $(j data.level2_night)"
req GET /approvals/rules "$EMP"; check "员工无 approval:rule → 403" 403 "$STATUS"
req PUT /approvals/rules "$ADMIN" '{"night_start":"25:00"}'; check "night_start 非法 → 400" 400 "$STATUS"
req PUT /approvals/rules "$ADMIN" '{"overdue_alert_minutes":1}'; check "overdue_alert_minutes<5 → 400" 400 "$STATUS"
req PUT /approvals/rules "$ADMIN" '{"level2_approver_id":"00000000-0000-0000-0000-000000000001"}'; check "二级审批人不存在 → 400" 400 "$STATUS"
req PUT /approvals/rules "$ADMIN" "{\"level2_km\":50,\"level2_cross_dept\":true,\"level2_approver_id\":\"$U_L2\",\"night_start\":\"22:00\",\"night_end\":\"06:00\",\"level2_trip_types\":[],\"fallback_approver_role\":\"a_$SFX\"}"
check "PUT upsert → 200" 200 "$STATUS"
check "  level2_km=50 cross_dept=true" "50 true" "$(j data.level2_km) $(j data.level2_cross_dept)"
check "  level2_approver 为 UserBrief" "l2" "$(j data.level2_approver.name)"
check "  updated_at 非空" true "$([ "$(j data.updated_at)" != null ] && echo true)"
req PUT /approvals/rules "$ADMIN" '{"level2_km":null}'; check "level2_km 显式 null 清空" null "$(j data.level2_km)"
req PUT /approvals/rules "$ADMIN" '{"level2_km":50}'; check "再设回 50" 50 "$(j data.level2_km)"

# ------------------------------------------------------------------ precheck
section "预检 precheck"
S1=$(utc "$D 01:00"); E1=$(utc "$D 03:00")   # 09:00-11:00 CST
mkreq() { # start end vehicle_json extra_json
  echo "{\"trip_type\":\"daily\",\"purpose_code\":\"official_trip\",\"purpose_detail\":\"客户拜访测试\",\"planned_start\":\"$1\",\"planned_end\":\"$2\",\"destination\":\"济南西站\"$3$4}"
}
req POST /approvals/precheck "$EMP" "$(mkreq "$E1" "$S1" ",\"vehicle_id\":\"$V2\"" "")"
check "结束早于开始 → ok=false" "200 false" "$STATUS $(j data.ok)"; contains "  problems 含时间" "结束时间必须晚于开始时间" "$(j data.problems)"
req POST /approvals/precheck "$EMP" "$(mkreq "$(utc '-1 hour')" "$(utc '+1 hour')" "" "")"
contains "开始时间早于当前 → problems" "开始时间不能早于当前时间" "$(j data.problems)"
req POST /approvals/precheck "$EMP" "$(mkreq "$S1" "$(utc "$D 01:00 UTC 8 days")" "" "")"
contains "时长 > 7 天 → problems" "用车时长不能超过 7 天" "$(j data.problems)"
req POST /approvals/precheck "$EMP" "$(mkreq "$S1" "$E1" ",\"vehicle_id\":\"$V3\"" "")"
contains "维保车辆 → 车辆不可用" "车辆不可用" "$(j data.problems)"
req POST /approvals/precheck "$EMP" "$(mkreq "$S1" "$E1" ",\"vehicle_id\":\"00000000-0000-0000-0000-000000000001\"" "")"
contains "车辆不存在 → problems" "车辆不存在" "$(j data.problems)"
req POST /approvals/precheck "$EMP" "$(mkreq "$S1" "$E1" ",\"vehicle_id\":\"$V2\"" ",\"planned_km\":10")"
check "正常日间申请 → ok" true "$(j data.ok)"
check "  level_required=1，一级审批人=部门负责人 lead" "1 lead" "$(j data.level_required) $(j data.approvers.0.name)"
check "  level2_reasons 为空" 0 "$(jlen data.level2_reasons)"
req POST /approvals/precheck "$EMP" "$(mkreq "$S1" "$E1" ",\"vehicle_id\":\"$V2\"" ",\"planned_km\":80")"
check "里程 80 ≥ 50 → level 2" 2 "$(j data.level_required)"; contains "  原因含里程" "预计里程" "$(j data.level2_reasons)"
check "  审批人 [lead, l2]" "lead l2" "$(j data.approvers.0.name) $(j data.approvers.1.name)"
req POST /approvals/precheck "$EMP" "$(mkreq "$S1" "$E1" ",\"vehicle_id\":\"$V1\"" "")"
contains "跨部门车辆 → 原因" "跨部门" "$(j data.level2_reasons)"
req POST /approvals/precheck "$EMP" "$(mkreq "$(utc "$D 13:00")" "$(utc "$D 15:00")" ",\"vehicle_id\":\"$V2\"" "")"
contains "夜间时段 21:00-23:00 CST → 原因" "夜间" "$(j data.level2_reasons)"
req POST /approvals/precheck "$LEAD" "$(mkreq "$S1" "$E1" "" "")"
check "lead 自己申请：负责人是本人 → 回退 approver 角色用户" true "$([ "$(j data.approvers.0.name)" != lead ] && [ "$(j data.ok)" = true ] && echo true)"
req POST /approvals/precheck "$EMP2" "$(mkreq "$S1" "$E1" "" ",\"applicant_id\":\"$U_EMP\"")"
check "员工代人发起 → 403" 403 "$STATUS"
req POST /approvals/precheck "$EMP" '{"trip_type":"x"}'; check "参数校验 → 400" 400 "$STATUS"

# ------------------------------------------------------------------ create & visibility
section "创建申请 / 可见范围"
req POST /approvals "$EMP" "$(mkreq "$S1" "$E1" ",\"vehicle_id\":\"$V2\"" ",\"planned_km\":10,\"passenger_ids\":[\"$U_EMP2\"]")"
check "emp 创建 A1 → 201" 201 "$STATUS"; A1=$(j data.id); A1_NO=$(j data.apply_no)
check "  apply_no 形如 ZY-YYYYMMDD-NNNN" true "$(printf '%s' "$A1_NO" | grep -Eq '^ZY-[0-9]{8}-[0-9]{4}$' && echo true)"
check "  status=pending_l1 level=1 step=1" "pending_l1 1 1" "$(j data.status) $(j data.level_required) $(j data.current_step)"
check "  steps[0].approver=lead pending" "lead pending" "$(j data.steps.0.approver.name) $(j data.steps.0.action)"
check "  applicant/dept/vehicle/purpose_label" "emp 研发部_$SFX 鲁A$SFX 公务出行" "$(j data.applicant.name) $(j data.dept_name) $(j data.vehicle.plate_no) $(j data.purpose_label)"
check "  passengers 含 emp2" emp2 "$(j data.passengers.0.name)"
check "  can_cancel=true can_approve=false（申请人视角）" "true false" "$(j data.can_cancel) $(j data.can_approve)"
req POST /approvals "$EMP" "$(mkreq "$(utc "$D 02:00")" "$(utc "$D 04:00")" ",\"vehicle_id\":\"$V2\"" "")"
check "同车时段重叠 → 409" 409 "$STATUS"
req POST /approvals/precheck "$EMP" "$(mkreq "$(utc "$D 03:10")" "$(utc "$D 04:00")" ",\"vehicle_id\":\"$V2\"" "")"
check "结束后 10 分钟（< 15 分钟缓冲）预检 → conflicts 含 A1" "$A1_NO" "$(j data.conflicts.0.apply_no)"
req POST /approvals "$EMP" "$(mkreq "$S1" "$E1" ",\"vehicle_id\":\"$V3\"" "")"
check "维保车辆创建 → 400" 400 "$STATUS"
req GET "/notifications?type=approval.pending" "$LEAD"
check "lead 收到待审批通知" true "$(jhas data.items ref_id "$A1")"

req GET "/approvals?scope=mine" "$EMP";  check "emp scope=mine 含 A1" true "$(jhas data.items id "$A1")"
req GET "/approvals?scope=mine" "$EMP2"; check "emp2 scope=mine 不含 A1" false "$(jhas data.items id "$A1")"
req GET "/approvals?scope=todo" "$EMP";  check "emp scope=todo → 403（无 approve 权限）" 403 "$STATUS"
req GET "/approvals?scope=all" "$EMP";   check "emp scope=all → 403" 403 "$STATUS"
req GET "/approvals?scope=todo" "$LEAD"; check "lead scope=todo 含 A1" true "$(jhas data.items id "$A1")"
req GET "/approvals?scope=todo" "$L2";   check "l2 scope=todo 不含 A1" false "$(jhas data.items id "$A1")"
req GET "/approvals?scope=all&keyword=$A1_NO" "$ADMIN"; check "admin scope=all keyword 单号" 1 "$(j data.total)"
req GET "/approvals?scope=all&dept_id=$DEPT_A&status=pending_l1" "$ADMIN"; check "  dept_id+status 过滤" true "$(jall data.items dept_id "$DEPT_A")"
req GET "/approvals?scope=all&vehicle_id=$V2&from=$S1&to=$S1" "$ADMIN"; check "  vehicle_id+from/to" true "$(jhas data.items id "$A1")"
req GET "/approvals?scope=all&sort=-planned_start&pageSize=2" "$ADMIN"; check "  排序分页 → 200" "200 2" "$STATUS $(j data.pageSize)"
req GET "/approvals?from=bad" "$EMP"; check "from 非法 → 400" 400 "$STATUS"
req GET /approvals/todo-count "$LEAD"; check "lead todo-count ≥ 1" true "$([ "$(j data.todo)" -ge 1 ] && echo true)"
req GET /approvals/todo-count "$EMP";  check "emp todo-count = 0" 0 "$(j data.todo)"
req GET "/approvals/$A1" "$EMP2"; check "无关员工看详情 → 403" 403 "$STATUS"
req GET "/approvals/$A1" "$LEAD"; check "审批人看详情 → 200 can_approve=true" "200 true" "$STATUS $(j data.can_approve)"
req GET "/approvals/$A1" "$ADMIN"; check "manage 看详情 → 200 can_cancel=true" "200 true" "$STATUS $(j data.can_cancel)"
req GET "/approvals/00000000-0000-0000-0000-000000000001" "$ADMIN"; check "不存在 → 404" 404 "$STATUS"
req POST /approvals "$ADMIN" "$(mkreq "$(utc "$D 11:00")" "$(utc "$D 12:00")" "" ",\"applicant_id\":\"$U_EMP\"")"
check "manage 代人发起 → 201 applicant=emp" "201 emp" "$STATUS $(j data.applicant.name)"; A_PROXY=$(j data.id)

# ------------------------------------------------------------------ approve (level 1)
section "审批通过（一级）"
req POST "/approvals/$A1/approve" "$EMP" '{}';   check "emp 审批 → 403（无权限）" 403 "$STATUS"
req POST "/approvals/$A1/approve" "$L2" '{}';    check "非当前步骤审批人 → 403" 403 "$STATUS"
req POST "/approvals/$A1/approve" "$LEAD" '{"remark":"同意"}'
check "lead 通过 → approved" "200 approved" "$STATUS $(j data.status)"
check "  approved_at 非空，step 已记录" "approved 同意" "$(j data.steps.0.action) $(j data.steps.0.remark)"
check "  can_approve=false can_cancel=false（审批人）" "false false" "$(j data.can_approve) $(j data.can_cancel)"
req POST "/approvals/$A1/approve" "$LEAD" '{}'; check "重复审批 → 409" 409 "$STATUS"
req GET "/notifications?type=approval.approved" "$EMP"; check "emp 收到通过通知" true "$(jhas data.items ref_id "$A1")"
req GET "/approvals?scope=todo" "$LEAD"; check "lead todo 不再含 A1" false "$(jhas data.items id "$A1")"

# no vehicle → must assign at approval
req POST /approvals "$EMP" "$(mkreq "$(utc "$D 05:00")" "$(utc "$D 06:00")" "" "")"
check "无车辆申请 A3 → 201 vehicle=null" "201 null" "$STATUS $(j data.vehicle)"; A3=$(j data.id)
req POST "/approvals/$A3/approve" "$LEAD" '{}'; check "无车辆时不传 vehicle_id → 400" 400 "$STATUS"
req POST "/approvals/$A3/approve" "$LEAD" "{\"vehicle_id\":\"$V3\"}"; check "指派维保车辆 → 409" 409 "$STATUS"
req POST "/approvals/$A3/approve" "$LEAD" "{\"vehicle_id\":\"$V1\"}"
check "指派 V1 → approved" "approved 鲁B$SFX" "$(j data.status) $(j data.vehicle.plate_no)"

# ------------------------------------------------------------------ level 2
section "二级审批"
req POST /approvals "$EMP" "$(mkreq "$(utc "$D 04:00")" "$(utc "$D 05:00")" ",\"vehicle_id\":\"$V2\"" ",\"planned_km\":80")"
check "里程 80 创建 A4 → level_required=2" "201 2" "$STATUS $(j data.level_required)"; A4=$(j data.id); A4_NO=$(j data.apply_no)
check "  steps: lead, l2" "lead l2" "$(j data.steps.0.approver.name) $(j data.steps.1.approver.name)"
req POST "/approvals/$A4/approve" "$L2" '{}'; check "二级审批人抢先 → 403" 403 "$STATUS"
req POST "/approvals/$A4/approve" "$LEAD" '{"remark":"一级同意"}'
check "一级通过 → pending_l2 step=2" "pending_l2 2" "$(j data.status) $(j data.current_step)"
req GET "/notifications?type=approval.pending" "$L2"; check "l2 收到二级待审批通知" true "$(jhas data.items ref_id "$A4")"
req GET "/notifications?type=approval.approved" "$EMP"; check "emp 收到一级已通过通知" true "$(jhas data.items ref_id "$A4")"
req POST "/approvals/$A4/approve" "$LEAD" '{}'; check "一级审批人再审 → 403" 403 "$STATUS"
req GET "/approvals?scope=todo" "$L2"; check "l2 todo 含 A4" true "$(jhas data.items id "$A4")"
req POST "/approvals/$A4/approve" "$L2" "{\"vehicle_id\":\"$V1\"}"; check "二级审批人改派车辆 → 403（需 manage）" 403 "$STATUS"
req POST "/approvals/$A4/approve" "$L2" '{"remark":"二级同意"}'
check "二级通过 → approved" "approved" "$(j data.status)"
check "  两步均 approved" "approved approved" "$(j data.steps.0.action) $(j data.steps.1.action)"

# ------------------------------------------------------------------ reject / cancel
section "驳回 / 撤销"
req POST /approvals "$EMP" "$(mkreq "$(utc "$D 06:00")" "$(utc "$D 07:00")" ",\"vehicle_id\":\"$V2\"" "")"; A5=$(j data.id)
req POST "/approvals/$A5/reject" "$LEAD" '{}'; check "驳回缺 reason → 400" 400 "$STATUS"
req POST "/approvals/$A5/reject" "$EMP2" '{"reason":"x"}'; check "无权限驳回 → 403" 403 "$STATUS"
req POST "/approvals/$A5/reject" "$LEAD" '{"reason":"车辆另有安排"}'
check "驳回 → rejected" "rejected 车辆另有安排" "$(j data.status) $(j data.reject_reason)"
check "  step action=rejected" rejected "$(j data.steps.0.action)"
req GET "/notifications?type=approval.rejected" "$EMP"; check "emp 收到驳回通知" true "$(jhas data.items ref_id "$A5")"
req POST "/approvals/$A5/cancel" "$EMP" '{}'; check "已驳回不可撤销 → 409" 409 "$STATUS"
req POST "/approvals/$A5/approve" "$LEAD" '{}'; check "已驳回不可审批 → 409" 409 "$STATUS"

req POST /approvals "$EMP" "$(mkreq "$(utc "$D 08:00")" "$(utc "$D 09:00")" ",\"vehicle_id\":\"$V2\"" "")"; A6=$(j data.id)
req POST "/approvals/$A6/cancel" "$EMP2" '{"reason":"x"}'; check "他人撤销 → 403" 403 "$STATUS"
req POST "/approvals/$A6/cancel" "$EMP" '{"reason":"计划取消"}'
check "申请人撤销 pending → cancelled" "cancelled 计划取消" "$(j data.status) $(j data.cancel_reason)"
check "  步骤置 skipped" skipped "$(j data.steps.0.action)"
req GET "/notifications?type=approval.cancelled" "$LEAD"; check "lead 收到撤销通知" true "$(jhas data.items ref_id "$A6")"
req POST "/approvals/$A3/cancel" "$ADMIN" '{"reason":"管理员撤销已批准申请"}'; check "manage 撤销 approved → cancelled" cancelled "$(j data.status)"
req GET "/approvals/available-vehicles?start=$(utc "$D 05:00")&end=$(utc "$D 06:00")" "$EMP"
check "A3 撤销后 05-06 时段 V1 可用" true "$(jhas data id "$V1")"

# ------------------------------------------------------------------ trips (web)
section "行程 trips（Web 开始/结束）"
req POST /trips/start "$EMP" "{\"approval_id\":\"$A1\"}"; check "员工开始行程 → 403" 403 "$STATUS"
req POST /trips/start "$ADMIN" "{\"approval_id\":\"$A5\"}"; check "已驳回申请开始 → 409" 409 "$STATUS"
req POST /trips/start "$ADMIN" "{\"approval_id\":\"$A1\"}"
check "manage 开始 A1 → 201" 201 "$STATUS"; T1=$(j data.id); T1_NO=$(j data.trip_no)
check "  trip_no 形如 T-YYYYMMDD-NNN" true "$(printf '%s' "$T1_NO" | grep -Eq '^T-[0-9]{8}-[0-9]{3,}$' && echo true)"
check "  ongoing/web/driver=emp/start_odometer=1000/start_soc=80" "ongoing web emp 1000 80" "$(j data.status) $(j data.source) $(j data.driver.name) $(j data.start_odometer) $(j data.start_soc)"
check "  approval 摘要" "$A1_NO" "$(j data.approval.apply_no)"
req GET "/approvals/$A1" "$EMP"; check "A1 → in_use，trip 摘要" "in_use $T1_NO" "$(j data.status) $(j data.trip.trip_no)"
check "  in_use 不可撤销 can_cancel=false" false "$(j data.can_cancel)"
req POST "/approvals/$A1/cancel" "$EMP" '{}'; check "in_use 撤销 → 409" 409 "$STATUS"
check "车辆 V2 状态 in_use（vehicles 与 vehicle_status 同步）" "in_use in_use $T1" "$(sql "SELECT v.status || ' ' || vs.status || ' ' || vs.current_trip_id FROM vehicles v JOIN vehicle_status vs ON vs.vehicle_id = v.id WHERE v.id = '$V2';")"
req POST /trips/start "$ADMIN" "{\"approval_id\":\"$A1\"}"; check "再次开始 → 409" 409 "$STATUS"
req POST /trips/start "$ADMIN" "{\"approval_id\":\"$A4\"}"; check "同车另一申请开始（车辆 in_use）→ 409" 409 "$STATUS"
req GET "/notifications?type=trip.started" "$EMP"; check "驾驶员收到行程开始通知" true "$(jhas data.items ref_id "$T1")"

# 轨迹点：沿 117.00→117.09 东行，含一次超速；start_at 回拨 30 分钟
sql "INSERT INTO trip_points (ts, trip_id, vehicle_id, lng, lat, speed, soc) VALUES
 (now() - interval '20 min', '$T1', '$V2', 117.00, 36.60, 0, 80),
 (now() - interval '15 min', '$T1', '$V2', 117.03, 36.60, 60, 77),
 (now() - interval '10 min', '$T1', '$V2', 117.06, 36.60, 95, 75),
 (now() - interval '5 min',  '$T1', '$V2', 117.09, 36.60, 30, 72);
 UPDATE trips SET point_count = 4, start_at = now() - interval '30 minutes' WHERE id = '$T1';" >/dev/null
req GET "/trips/$T1/track?step=2" "$EMP"
check "驾驶员看轨迹 step=2 → 首/中/尾 3 点" "200 3" "$STATUS $(jlen data.points)"
check "  points 按 ts 升序（首点 lng=117）" 117 "$(j data.points.0.lng)"
req GET "/trips/$T1/track" "$EMP2"; check "无关员工看轨迹 → 403" 403 "$STATUS"
req GET "/trips/$T1/track?step=0" "$EMP"; check "step=0 → 400" 400 "$STATUS"
req POST "/trips/$T1/end" "$EMP" '{}'; check "员工结束行程 → 403" 403 "$STATUS"
req POST "/trips/$T1/end" "$ADMIN" '{"end_odometer":1012.4,"end_soc":70,"remark":"正常还车"}'
check "manage 结束 → completed" "200 completed" "$STATUS $(j data.status)"
check "  distance=12.4(里程表) energy=6 kWh max_speed=95 point_count=4" "12.4 6 95 4" "$(j data.distance_km) $(j data.energy_kwh) $(j data.max_speed) $(j data.point_count)"
check "  energy_per_100km≈48.39, end_soc=70, roof=unknown(daily)" "48.39 70 unknown" "$(j data.energy_per_100km) $(j data.end_soc) $(j data.roof_sign_status)"
check "  duration_min≈30" true "$(python3 -c "import sys; v=float('$(j data.duration_min)'); print(str(29.5 <= v <= 31).lower())")"
check "  events 含 start/end" "start end" "$(j data.events.0.type) $(j data.events.1.type)"
req GET "/approvals/$A1" "$EMP"; check "A1 → completed" completed "$(j data.status)"
check "车辆 V2 回到 idle" "idle idle" "$(sql "SELECT v.status || ' ' || vs.status FROM vehicles v JOIN vehicle_status vs ON vs.vehicle_id = v.id WHERE v.id = '$V2';")"
req GET "/notifications?type=trip.ended" "$EMP"; check "驾驶员收到结束通知" true "$(jhas data.items ref_id "$T1")"
req POST "/trips/$T1/end" "$ADMIN" '{}';    check "再次结束 → 409" 409 "$STATUS"
req POST "/trips/$T1/cancel" "$ADMIN" '{}'; check "已完成不可作废 → 409" 409 "$STATUS"
req GET "/trips/$T1/events" "$LEAD"; check "审批人看事件 → 200 ≥2 条" true "$([ "$STATUS" = 200 ] && [ "$(jlen data)" -ge 2 ] && echo true)"

# 行程列表可见范围
req GET "/trips?scope=mine" "$EMP";  check "emp scope=mine 含 T1" true "$(jhas data.items id "$T1")"
req GET "/trips?scope=mine" "$EMP2"; check "emp2 scope=mine 不含 T1" false "$(jhas data.items id "$T1")"
req GET "/trips?scope=all" "$EMP";   check "emp scope=all → 403" 403 "$STATUS"
req GET "/trips?scope=all&keyword=鲁A$SFX&status=completed&dept_id=$DEPT_A" "$ADMIN"; check "admin scope=all 筛选含 T1" true "$(jhas data.items id "$T1")"
req GET "/trips?scope=all&sort=-distance_km&deviation=false&trip_type=daily" "$ADMIN"; check "  排序/deviation/trip_type → 200" 200 "$STATUS"
req GET "/trips?status=bogus" "$EMP"; check "status 非法 → 400" 400 "$STATUS"
req GET "/trips/$T1" "$EMP2"; check "无关员工看详情 → 403" 403 "$STATUS"
req GET "/trips/$T1" "$LEAD"; check "审批人看详情 → 200" 200 "$STATUS"

# 作废：开始 A4 后作废 → 申请回到 approved
req POST /trips/start "$ADMIN" "{\"approval_id\":\"$A4\",\"driver_id\":\"$U_EMP2\"}"
check "开始 A4 指定驾驶员 emp2 → 201" "201 emp2" "$STATUS $(j data.driver.name)"; T2=$(j data.id)
req POST "/trips/$T2/cancel" "$ADMIN" '{"reason":"误触发"}'
check "作废 → cancelled" "200 cancelled" "$STATUS $(j data.status)"
req GET "/approvals/$A4" "$EMP"; check "A4 回到 approved" approved "$(j data.status)"
check "车辆 V2 回到 idle" idle "$(sql "SELECT status FROM vehicles WHERE id = '$V2';")"
req GET "/approvals/available-vehicles?start=$(utc "$D 10:00")&end=$(utc "$D 10:30")" "$EMP"
check "可用车辆含 V1、V2，不含维保 V3" "true true false" "$(jhas data id "$V1") $(jhas data id "$V2") $(jhas data id "$V3")"
req GET "/approvals/available-vehicles?start=$(utc "$D 04:00")&end=$(utc "$D 04:30")" "$EMP"
check "A4（approved，V2）时段内 V2 不可用" false "$(jhas data id "$V2")"
req GET "/approvals/available-vehicles?start=x" "$EMP"; check "参数缺失 → 400" 400 "$STATUS"

# ------------------------------------------------------------------ summary / overview / export
section "汇总 / 总览 / 导出"
TODAY=$(TZ=Asia/Shanghai date +%Y-%m-%d)
req GET "/trips/summary?date=$TODAY" "$EMP"
check "summary → 200 date=today" "200 $TODAY" "$STATUS $(j data.date)"
check "  trips ≥ 1，distance ≥ 12.4，ongoing=0" true "$([ "$(j data.trips)" -ge 1 ] && python3 -c "import sys; sys.exit(0 if float('$(j data.distance_km)') >= 12.4 else 1)" && [ "$(j data.ongoing)" = 0 ] && echo true)"
req GET "/trips/summary?date=2000-01-01" "$EMP"; check "远古日期 → 0 行程" 0 "$(j data.trips)"
req GET "/trips/summary?date=bad" "$EMP"; check "date 非法 → 400" 400 "$STATUS"
req GET /dashboard/overview "$ADMIN"
check "overview → 200" 200 "$STATUS"
check "  vehicles.total ≥ 3, maintenance ≥ 1" true "$([ "$(j data.vehicles.total)" -ge 3 ] && [ "$(j data.vehicles.maintenance)" -ge 1 ] && echo true)"
check "  today/devices/recent_events 存在" "$TODAY true" "$(j data.today.date) $([ "$(j data.recent_events)" != null ] && [ "$(j data.devices.total)" != null ] && echo true)"
check "  approvals_pending ≥ 1（manage）" true "$([ "$(j data.approvals_pending)" -ge 1 ] && echo true)"
req GET /dashboard/overview "$LEAD"
check "lead overview：approvals_todo ≥ 0，approvals_pending=0（无 manage）" "200 0" "$STATUS $(j data.approvals_pending)"
curl -s -D "$TMP/exp.h" -o "$TMP/trips.xlsx" -H "Authorization: Bearer $ADMIN" "$BASE/trips/export?scope=all&keyword=$SFX"
check "导出 xlsx Content-Type" true "$(grep -qi 'content-type: application/vnd.openxmlformats' "$TMP/exp.h" && echo true)"
check "  行数=本次创建的 2 条行程" 2 "$(xlsx_rows "$TMP/trips.xlsx")"
curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $EMP" "$BASE/trips/export" > "$TMP/code"; check "员工导出 → 403" 403 "$(cat "$TMP/code")"

# ------------------------------------------------------------------ expired
section "过期懒标记"
EXP=$(sql "INSERT INTO approvals (tenant_id, apply_no, applicant_id, dept_id, trip_type, purpose_code, purpose_detail, planned_start, planned_end, destination, vehicle_id, status, approved_at)
 VALUES ('$CH_TID', 'ZY-EXP-$SFX', '$U_EMP', '$DEPT_A', 'daily', 'other', '过期测试', now() - interval '5 hours', now() - interval '3 hours', '测试', '$V1', 'approved', now()) RETURNING id;")
sql "INSERT INTO approval_steps (approval_id, step_no, approver_id, action, acted_at) VALUES ('$EXP', 1, '$U_LEAD', 'approved', now());" >/dev/null
EXP2=$(sql "INSERT INTO approvals (tenant_id, apply_no, applicant_id, dept_id, trip_type, purpose_code, purpose_detail, planned_start, planned_end, destination, status)
 VALUES ('$CH_TID', 'ZY-EXP2-$SFX', '$U_EMP', '$DEPT_A', 'daily', 'other', '过期待审', now() - interval '5 hours', now() - interval '3 hours', '测试', 'pending_l1') RETURNING id;")
sql "INSERT INTO approval_steps (approval_id, step_no, approver_id) VALUES ('$EXP2', 1, '$U_LEAD');" >/dev/null
req GET "/approvals?scope=mine&status=expired" "$EMP"
check "列表读取后 approved 过期申请被标记 expired" true "$(jhas data.items id "$EXP")"
check "  pending 过期申请也被标记" true "$(jhas data.items id "$EXP2")"
req GET "/approvals/$EXP2" "$LEAD"; check "详情 status=expired，步骤 skipped" "expired skipped" "$(j data.status) $(j data.steps.0.action)"
req GET "/approvals?scope=todo" "$LEAD"; check "过期申请不再出现在待办" false "$(jhas data.items id "$EXP2")"
req POST /trips/start "$ADMIN" "{\"approval_id\":\"$EXP\"}"; check "过期申请不能开始行程 → 409" 409 "$STATUS"

# ------------------------------------------------------------------ cleanup
section "清理"
sql "DELETE FROM notifications WHERE tenant_id = '$CH_TID' AND ref_id IN (SELECT id::text FROM approvals WHERE tenant_id = '$CH_TID' AND applicant_id IN ('$U_EMP','$U_EMP2','$U_LEAD','$ADMIN_UID') AND created_at > now() - interval '1 hour');
 DELETE FROM notifications WHERE user_id IN ('$U_EMP','$U_EMP2','$U_LEAD','$U_L2');
 DELETE FROM trip_events WHERE vehicle_id IN ('$V1','$V2','$V3');
 DELETE FROM trip_points WHERE vehicle_id IN ('$V1','$V2','$V3');
 DELETE FROM trips WHERE vehicle_id IN ('$V1','$V2','$V3');
 DELETE FROM approval_steps WHERE approval_id IN (SELECT id FROM approvals WHERE applicant_id IN ('$U_EMP','$U_EMP2','$U_LEAD') OR vehicle_id IN ('$V1','$V2','$V3'));
 DELETE FROM approvals WHERE applicant_id IN ('$U_EMP','$U_EMP2','$U_LEAD') OR vehicle_id IN ('$V1','$V2','$V3');
 DELETE FROM approval_rules WHERE tenant_id = '$CH_TID';
 DELETE FROM vehicle_status WHERE vehicle_id IN ('$V1','$V2','$V3');
 DELETE FROM vehicles WHERE id IN ('$V1','$V2','$V3');
 UPDATE departments SET leader_user_id = NULL WHERE id IN ('$DEPT_A','$DEPT_B');
 DELETE FROM user_roles WHERE user_id IN ('$U_EMP','$U_EMP2','$U_LEAD','$U_L2');
 DELETE FROM users WHERE id IN ('$U_EMP','$U_EMP2','$U_LEAD','$U_L2');
 DELETE FROM departments WHERE id IN ('$DEPT_A','$DEPT_B');
 DELETE FROM role_permissions WHERE role_id IN ('$R_EMP','$R_APR');
 DELETE FROM roles WHERE id IN ('$R_EMP','$R_APR');" >/dev/null
ok "测试数据已清理"

echo
echo "== 结果：通过 $PASS，失败 $FAIL"
[ "$FAIL" -eq 0 ]
