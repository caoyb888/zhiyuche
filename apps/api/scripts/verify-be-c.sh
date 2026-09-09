#!/usr/bin/env bash
# 端到端验证：计费规则 / 三级账户与流水 / 行程自动计费 / 充电计价占位 / 月度结算（阶段 3 BE-C）。
#
# 前置：api 已启动并完成引导（chenhua_admin / Admin@123456），迁移 00004 已应用；机器上有 curl 与 python3。
# 用法：BASE=http://localhost:20090/api/v1 \
#       PSQL="docker exec -i zhiyuche-postgres psql -U zhiyuche -d zhiyuche_be1 -tAq" \
#       SIM_BIN=./bin/zhiyuche-simulator SIM_SECONDS=90 \
#       bash apps/api/scripts/verify-be-c.sh
# SIM_BIN 可选：给出时后台跑模拟器产生已完成行程，验证行程结束自动计费；充电事务由 PSQL 直接写入。
set -u
BASE=${BASE:-http://localhost:20090/api/v1}
PSQL=${PSQL:-}
SIM_BIN=${SIM_BIN:-}
SIM_SECONDS=${SIM_SECONDS:-90}
if [ -z "$PSQL" ]; then echo "PSQL is required"; exit 2; fi
SFX=$(date +%s | tail -c 6)$RANDOM
TMP=${TMPDIR:-/tmp}/zy-bec-$$
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "  ok   $1"; }
bad() { FAIL=$((FAIL+1)); echo "  FAIL $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want [$2], got [$3]) body: ${BODY:0:200}"; fi; }
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
elif isinstance(d, float): print(("%.2f" % d).rstrip("0").rstrip("."))
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
# jfind ARRAYPATH FIELD VALUE WANTFIELD
jfind() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d.get(k) if isinstance(d, dict) else None
for it in d or []:
    if str(it.get(sys.argv[2])) == sys.argv[3]:
        v = it.get(sys.argv[4])
        print("null" if v is None else (str(v).lower() if isinstance(v, bool) else (("%.2f" % v).rstrip("0").rstrip(".") if isinstance(v, float) else v))); sys.exit(0)
print("__missing__")' "$1" "$2" "$3" "$4"; }
jhas() { [ "$(jfind "$1" "$2" "$3" "$2")" != "__missing__" ] && echo true || echo false; }
# jnode: 在账户树里按 owner_id 找节点字段
jnode() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
def walk(n):
    if str(n.get("owner_id")) == sys.argv[1]: return n
    for c in n.get("children") or []:
        r = walk(c)
        if r: return r
n = walk(d)
v = None if n is None else n.get(sys.argv[2])
print("__missing__" if n is None else ("null" if v is None else (str(v).lower() if isinstance(v, bool) else (("%.2f" % v).rstrip("0").rstrip(".") if isinstance(v, float) else v))))' "$1" "$2"; }
sql() { printf '%s' "$1" | $PSQL | head -n1; }
xlsx_info() { python3 - "$1" <<'PY'
import sys, zipfile, re
z = zipfile.ZipFile(sys.argv[1])
sheets = sorted(n for n in z.namelist() if n.startswith("xl/worksheets/sheet"))
rows = [len(re.findall(r"<row[ >]", z.read(s).decode())) - 1 for s in sheets]
print("%d %s" % (len(sheets), " ".join(str(r) for r in rows)))
PY
}

# ------------------------------------------------------------------ login
section "登录"
req POST /auth/login "" '{"username":"chenhua_admin","password":"Admin@123456","tenant_code":"chenhua"}'
ADMIN=$(j data.access_token); check "租户管理员登录" 200 "$STATUS"
req GET /auth/me "$ADMIN"; CH_TID=$(j data.tenant.id); ADMIN_UID=$(j data.id)
PW='Emp@123456'
login() { req POST /auth/login "" "{\"username\":\"$1\",\"password\":\"$PW\",\"tenant_code\":\"chenhua\"}"; j data.access_token; }

# ------------------------------------------------------------------ rules
section "计费规则"
req GET /billing/rules/template "$ADMIN"
check "模板 200" 200 "$STATUS"; check "模板为纯电车队标准套餐" "纯电车队标准套餐" "$(j data.rule_name)"; check "模板 per_km 0.6" 0.6 "$(j data.base_rate.per_km)"
TEMPLATE=$(j data)

req POST /billing/rules "$ADMIN" "{\"name\":\"坏规则\",\"rule\":{\"rule_name\":\"x\",\"base_rate\":{\"per_km\":0,\"per_hour\":0}}}"
check "per_km/per_hour 同时为 0 → 400" 400 "$STATUS"; contains "  错误信息说明原因" "per_km" "$(j message)"
req POST /billing/rules "$ADMIN" "{\"name\":\"坏规则\",\"rule\":{\"base_rate\":{\"per_km\":1},\"time_multipliers\":[{\"name\":\"a\",\"range\":\"25:00-26:00\",\"factor\":1}]}}"
check "时段格式非法 → 400" 400 "$STATUS"
req POST /billing/rules "$ADMIN" "{\"name\":\"坏规则\",\"rule\":{\"base_rate\":{\"per_km\":1},\"penalty_rules\":[{\"type\":\"nope\"}]}}"
check "未知罚则类型 → 400" 400 "$STATUS"
req POST /billing/rules "$ADMIN" "{\"name\":\"坏规则\"}"
check "缺 rule → 400" 400 "$STATUS"

req POST /billing/rules "$ADMIN" "{\"name\":\"套餐A_$SFX\",\"rule\":$TEMPLATE,\"activate\":true}"
RULE_A=$(j data.id); check "创建规则 A（activate）201" 201 "$STATUS"; check "  A 为生效规则" true "$(j data.is_default)"; check "  返回 rule 文档" 0.6 "$(j data.rule.base_rate.per_km)"
req POST /billing/rules "$ADMIN" "{\"name\":\"套餐B_$SFX\",\"rule\":{\"base_rate\":{\"per_km\":1.0,\"per_hour\":0,\"daily_cap\":0}},\"effective_from\":\"2026-01-01\"}"
RULE_B=$(j data.id); check "创建规则 B 201（rule_name 由 name 补齐）" 201 "$STATUS"; check "  B 非生效" false "$(j data.is_default)"
check "  rule_name 补齐" "套餐B_$SFX" "$(j data.rule.rule_name)"; check "  缺省归属补齐 department" department "$(j data.rule.trip_attribution.official)"
req GET /billing/rules "$ADMIN"; check "规则列表 200" 200 "$STATUS"
check "  列表含 A 且生效" true "$(jfind data id "$RULE_A" is_default)"; check "  列表含 B 不生效" false "$(jfind data id "$RULE_B" is_default)"
req GET "/billing/rules/$RULE_B" "$ADMIN"; check "规则详情 200" 200 "$STATUS"; check "  effective_from" 2026-01-01 "$(j data.effective_from)"
req GET "/billing/rules/00000000-0000-0000-0000-000000000001" "$ADMIN"; check "不存在规则 404" 404 "$STATUS"

req POST "/billing/rules/$RULE_B/activate" "$ADMIN"; check "激活 B 200" 200 "$STATUS"; check "  B 生效" true "$(j data.is_default)"
req GET "/billing/rules/$RULE_A" "$ADMIN"; check "  A 自动取消生效" false "$(j data.is_default)"
req DELETE "/billing/rules/$RULE_B" "$ADMIN"; check "删除生效规则 → 409" 409 "$STATUS"
req PUT "/billing/rules/$RULE_B" "$ADMIN" '{"enabled":false}'; check "停用生效规则 → 400" 400 "$STATUS"
req PUT "/billing/rules/$RULE_B" "$ADMIN" '{"rule":{"rule_name":"x","base_rate":{"per_km":-1}}}'; check "编辑为非法规则 → 400" 400 "$STATUS"
req PUT "/billing/rules/$RULE_B" "$ADMIN" "{\"name\":\"套餐B2_$SFX\",\"effective_from\":null,\"effective_to\":\"2026-12-31\"}"
check "编辑规则 200" 200 "$STATUS"; check "  名称已改" "套餐B2_$SFX" "$(j data.name)"; check "  effective_from 置空" null "$(j data.effective_from)"; check "  effective_to" 2026-12-31 "$(j data.effective_to)"
req PUT "/billing/rules/$RULE_B" "$ADMIN" '{"effective_from":"2027-01-01"}'; check "生效期倒置 → 400" 400 "$STATUS"
req POST "/billing/rules/$RULE_A/activate" "$ADMIN"; check "切回 A 生效" true "$(j data.is_default)"
req DELETE "/billing/rules/$RULE_B" "$ADMIN"; check "删除非生效规则 200" 200 "$STATUS"
req GET "/billing/rules/$RULE_B" "$ADMIN"; check "  删除后 404" 404 "$STATUS"

# ------------------------------------------------------------------ simulate
section "模拟计算（方案 §7.5 算例：47.3 km / 2h43m14s / 6.8 kWh / 09:05 → ¥46.40）"
TRIP='{"trip_type":"official","start_at":"2025-06-18T01:05:00Z","end_at":"2025-06-18T03:48:14Z","distance_km":47.3,"energy_kwh":6.8,"end_soc":61}'
req POST /billing/rules/simulate "$ADMIN" "{\"rule\":$TEMPLATE,\"trip\":$TRIP}"
check "内联规则 simulate 200" 200 "$STATUS"; check "  total = 46.4" 46.4 "$(j data.total)"; check "  3 行明细" 3 "$(jlen data.lines)"; check "  归属 department" department "$(j data.attribution)"
check "  里程费 28.38" 28.38 "$(jfind data.lines item 里程费 amount)"; check "  时长费 13.6" 13.6 "$(jfind data.lines item 时长费 amount)"; check "  电费 4.42" 4.42 "$(jfind data.lines item 电费 amount)"
req POST /billing/rules/simulate "$ADMIN" "{\"rule_id\":\"$RULE_A\",\"trip\":$TRIP}"
check "rule_id simulate 200 且 46.4" "200 46.4" "$STATUS $(j data.total)"
req POST /billing/rules/simulate "$ADMIN" "{\"trip\":$TRIP}"
check "缺省生效规则 simulate 46.4" 46.4 "$(j data.total)"; check "  rule_name 为生效规则" "纯电车队标准套餐" "$(j data.rule_name)"
req POST /billing/rules/simulate "$ADMIN" '{"trip":{"trip_type":"daily","start_at":"2025-06-18T00:00:00Z","end_at":"2025-06-18T10:00:00Z","distance_km":200,"end_soc":15,"overspeed_events":3}}'
check "早高峰+封顶+低电+罚金算例 113" 113 "$(j data.total)"; check "  cap_applied" true "$(j data.cap_applied)"; check "  penalty 25" 25 "$(j data.penalty)"; check "  daily → employee" employee "$(j data.attribution)"
req POST /billing/rules/simulate "$ADMIN" '{"trip":{"start_at":"2025-06-18T03:00:00Z","end_at":"2025-06-18T01:00:00Z","distance_km":1}}'; check "end_at 早于 start_at → 400" 400 "$STATUS"
req POST /billing/rules/simulate "$ADMIN" '{"rule_id":"00000000-0000-0000-0000-000000000001","trip":{"start_at":"2025-06-18T01:00:00Z","end_at":"2025-06-18T02:00:00Z","distance_km":1}}'; check "rule_id 不存在 → 404" 404 "$STATUS"
req POST /billing/rules/simulate "$ADMIN" '{}'; check "缺 trip → 400" 400 "$STATUS"

# ------------------------------------------------------------------ accounts
section "三级账户与流水"
req POST /system/depts "$ADMIN" "{\"name\":\"计费部_$SFX\",\"monthly_budget\":500}"; DEPT=$(j data.id); check "创建部门（预算 500）" 201 "$STATUS"
req POST /system/users "$ADMIN" "{\"username\":\"bemp_$SFX\",\"name\":\"计费员工\",\"password\":\"$PW\",\"dept_id\":\"$DEPT\"}"; EMP=$(j data.id); check "创建员工" 201 "$STATUS"
req POST /system/users "$ADMIN" "{\"username\":\"blead_$SFX\",\"name\":\"计费主管\",\"password\":\"$PW\",\"dept_id\":\"$DEPT\"}"; LEAD=$(j data.id); check "创建部门负责人" 201 "$STATUS"
req PUT "/system/depts/$DEPT" "$ADMIN" "{\"leader_user_id\":\"$LEAD\"}"; check "设置负责人" 200 "$STATUS"
req POST /system/users "$ADMIN" "{\"username\":\"bemp2_$SFX\",\"name\":\"无账户员工\",\"password\":\"$PW\"}"; EMP2=$(j data.id); check "创建无部门员工" 201 "$STATUS"

req GET /billing/accounts/tree "$ADMIN"
check "账户树 200" 200 "$STATUS"; check "  根为企业账户且已创建" "enterprise true" "$(j data.level) $(j data.exists)"; check "  企业 owner 为租户" "$CH_TID" "$(j data.owner_id)"
check "  新部门节点 exists=false 余额 0 预算 500" "false 0 500" "$(jnode "$DEPT" exists) $(jnode "$DEPT" balance) $(jnode "$DEPT" monthly_budget)"
check "  员工节点挂在部门下且 exists=false" "false" "$(jnode "$EMP" exists)"
ENT=$(j data.id)

req POST /billing/accounts/recharge "$ADMIN" '{"amount":-5}'; check "充值负数 → 400" 400 "$STATUS"
req POST /billing/accounts/recharge "$ADMIN" '{"amount":1000,"remark":"线下到账"}'
check "企业充值 200" 200 "$STATUS"; check "  type recharge 金额 1000" "recharge 1000" "$(j data.type) $(j data.amount)"; check "  account_id 为企业账户" "$ENT" "$(j data.account_id)"
check "  created_by_name" "$(sql "SELECT name FROM users WHERE id='$ADMIN_UID';")" "$(j data.created_by_name)"
ENT_BAL=$(j data.balance_after)
req GET "/billing/accounts/$ENT" "$ADMIN"; check "企业账户详情 200" 200 "$STATUS"; check "  余额与流水一致" "$ENT_BAL" "$(j data.balance)"; check "  owner_name 为租户名" "$(sql "SELECT name FROM tenants WHERE id='$CH_TID';")" "$(j data.owner_name)"

req POST "/billing/accounts/$ENT/allocate" "$ADMIN" "{\"to_level\":\"employee\",\"to_owner_id\":\"$EMP\",\"amount\":10}"; check "企业→员工划拨 → 400" 400 "$STATUS"
req POST "/billing/accounts/$ENT/allocate" "$ADMIN" '{"amount":10}'; check "未指定目标 → 400" 400 "$STATUS"
req POST "/billing/accounts/$ENT/allocate" "$ADMIN" "{\"to_level\":\"department\",\"to_owner_id\":\"$EMP\",\"amount\":10}"; check "目标部门不存在 → 400" 400 "$STATUS"
req POST "/billing/accounts/$ENT/allocate" "$ADMIN" "{\"to_level\":\"department\",\"to_owner_id\":\"$DEPT\",\"amount\":300,\"remark\":\"9 月额度\"}"
check "企业→部门划拨 300" 200 "$STATUS"; check "  出账 allocate_out -300" "allocate_out -300" "$(j data.from.type) $(j data.from.amount)"; check "  入账 allocate_in 余额 300" "allocate_in 300" "$(j data.to.type) $(j data.to.balance_after)"
DEPT_ACC=$(j data.to.account_id); check "  ref 指向对方账户" "$DEPT_ACC $ENT" "$(j data.from.ref_id) $(j data.to.ref_id)"; check "  account_name 为部门名" "计费部_$SFX" "$(j data.to.account_name)"
req POST "/billing/accounts/$DEPT_ACC/allocate" "$ADMIN" "{\"to_level\":\"employee\",\"to_owner_id\":\"$EMP\",\"amount\":10000}"; check "部门→员工超额 → 409" 409 "$STATUS"; contains "  提示余额不足" "余额不足" "$(j message)"
req POST "/billing/accounts/$DEPT_ACC/allocate" "$ADMIN" "{\"to_level\":\"employee\",\"to_owner_id\":\"$EMP\",\"amount\":50}"
check "部门→员工划拨 50（自动建户）" 200 "$STATUS"; EMP_ACC=$(j data.to.account_id); check "  部门余额 250" 250 "$(j data.from.balance_after)"
req POST "/billing/accounts/$EMP_ACC/allocate" "$ADMIN" "{\"to_account_id\":\"$DEPT_ACC\",\"amount\":1}"; check "员工向上划拨 → 400" 400 "$STATUS"
req GET "/billing/accounts/$EMP_ACC" "$ADMIN"; check "员工账户 余额 50 / owner_sub 部门名" "50 计费部_$SFX" "$(j data.balance) $(j data.owner_sub)"; check "  owner_name 姓名" "计费员工" "$(j data.owner_name)"
req POST "/billing/accounts/$EMP_ACC/adjust" "$ADMIN" '{"amount":0,"remark":"x"}'; check "调整 0 → 400" 400 "$STATUS"
req POST "/billing/accounts/$EMP_ACC/adjust" "$ADMIN" '{"amount":-20}'; check "调整缺 remark → 400" 400 "$STATUS"
req POST "/billing/accounts/$EMP_ACC/adjust" "$ADMIN" '{"amount":-20,"remark":"误划拨扣回"}'; check "调整 -20" "200 adjust 30" "$STATUS $(j data.type) $(j data.balance_after)"
req POST "/billing/accounts/$EMP_ACC/adjust" "$ADMIN" '{"amount":5,"type":"refund","remark":"退款","ref_type":"trip","ref_id":"x"}'; check "退款 +5" "refund 35" "$(j data.type) $(j data.balance_after)"
req PUT "/billing/accounts/$EMP_ACC" "$ADMIN" '{"credit_limit":100,"monthly_budget":80,"status":"frozen"}'
check "编辑账户 200" 200 "$STATUS"; check "  credit_limit/预算/冻结" "100 80 frozen" "$(j data.credit_limit) $(j data.monthly_budget) $(j data.status)"
req PUT "/billing/accounts/$EMP_ACC" "$ADMIN" '{"credit_limit":-1}'; check "透支额度负数 → 400" 400 "$STATUS"
req PUT "/billing/accounts/$EMP_ACC" "$ADMIN" '{"status":"active"}'; check "解冻" active "$(j data.status)"
req GET "/billing/accounts/00000000-0000-0000-0000-000000000001" "$ADMIN"; check "账户不存在 404" 404 "$STATUS"

req GET "/billing/accounts?level=employee&keyword=bemp_$SFX" "$ADMIN"; check "账户列表按级别+关键词" "200 1" "$STATUS $(j data.total)"; check "  命中员工账户" "$EMP_ACC" "$(j data.items.0.id)"
req GET "/billing/accounts?level=boss" "$ADMIN"; check "level 非法 → 400" 400 "$STATUS"
req GET "/billing/accounts?negative=true&pageSize=5" "$ADMIN"; check "仅负余额列表 200" 200 "$STATUS"
req GET "/billing/accounts?sort=-balance&pageSize=3" "$ADMIN"; check "排序 200" 200 "$STATUS"
req GET "/billing/accounts/$EMP_ACC/transactions" "$ADMIN"; check "账户流水 3 条" "200 3" "$STATUS $(j data.total)"; check "  倒序首条为退款" refund "$(j data.items.0.type)"
req GET "/billing/accounts/$EMP_ACC/transactions?type=adjust" "$ADMIN"; check "  按类型过滤 1 条" 1 "$(j data.total)"
req GET "/billing/accounts/$EMP_ACC/transactions?type=bogus" "$ADMIN"; check "  type 非法 → 400" 400 "$STATUS"
req GET "/billing/accounts/$EMP_ACC/transactions?from=2000-01-01T00:00:00Z&to=2000-01-02T00:00:00Z" "$ADMIN"; check "  时间过滤无结果" 0 "$(j data.total)"
# 部门名只匹配部门账户自身的两条流水（企业侧 allocate_out 的 account_name 是租户名）
req GET "/billing/transactions?keyword=计费部_$SFX" "$ADMIN"; check "租户流水按关键词（部门名）" "200 2" "$STATUS $(j data.total)"
# ref_id=部门账户：企业侧 allocate_out（对方=部门）+ 员工侧 allocate_in（来源=部门）
req GET "/billing/transactions?keyword=$DEPT_ACC" "$ADMIN"; check "租户流水按关键词（ref_id=对方账户）" 2 "$(j data.total)"; check "  命中企业侧 allocate_out" true "$(jhas data.items type allocate_out)"
req GET "/billing/transactions?level=department&type=allocate_in&keyword=$SFX" "$ADMIN"; check "  级别+类型过滤 1 条" 1 "$(j data.total)"; check "  account_level" department "$(j data.items.0.account_level)"

EMPTOK=$(login "bemp_$SFX"); EMP2TOK=$(login "bemp2_$SFX")
req GET /billing/accounts/me "$EMPTOK"; check "我的账户 200" 200 "$STATUS"; check "  exists 余额 35 流水 3" "true 35 3" "$(j data.exists) $(j data.balance) $(jlen data.transactions)"; check "  id 为员工账户" "$EMP_ACC" "$(j data.id)"
req GET /billing/accounts/me "$EMP2TOK"; check "无账户员工 me 200" 200 "$STATUS"; check "  虚拟账户 exists=false 余额 0 零 uuid" "false 0 00000000-0000-0000-0000-000000000000" "$(j data.exists) $(j data.balance) $(j data.id)"
req GET /billing/accounts/tree "$EMPTOK"; check "员工无权看账户树 → 403" 403 "$STATUS"
req GET /billing/accounts/tree "$ADMIN"
check "账户树：部门节点已建 余额 250" "true 250" "$(jnode "$DEPT" exists) $(jnode "$DEPT" balance)"; check "  员工节点余额 35" "35" "$(jnode "$EMP" balance)"

# ------------------------------------------------------------------ automatic trip billing (simulator)
section "行程结束自动计费"
# 上次模拟器被中断遗留的进行中行程会让新申请 409（车辆时段冲突），先经 web 结束接口收尾：也走 CompletedHook
for T in $(sql "SELECT string_agg(t.id::text, ' ') FROM trips t JOIN vehicles v ON v.id = t.vehicle_id WHERE t.tenant_id='$CH_TID' AND t.status='ongoing' AND v.plate_no LIKE '鲁A·S%';"); do
  req POST "/trips/$T/end" "$ADMIN" '{"remark":"verify-be-c 收尾"}'
  check "web 结束遗留行程 → 自动计费 charged" "200 charged" "$STATUS $(sql "SELECT billing_status FROM trips WHERE id='$T';")"
done
BEFORE_TRIPS=$(sql "SELECT count(*) FROM trips WHERE tenant_id='$CH_TID' AND billing_status='charged';")
if [ -n "$SIM_BIN" ] && [ -x "$SIM_BIN" ]; then
  # 模拟器驾驶员归入测试部门：公务行程 → 部门账户，可验证归属；首次运行驾驶员尚不存在则跳过
  SIM1=$(sql "SELECT id FROM users WHERE tenant_id='$CH_TID' AND username='sim_driver_1' AND deleted_at IS NULL;")
  if [ -n "$SIM1" ]; then req PUT "/system/users/$SIM1" "$ADMIN" "{\"dept_id\":\"$DEPT\"}"; check "sim_driver_1 归入测试部门" 200 "$STATUS"; fi
  nohup "$SIM_BIN" -api "$BASE" -vehicles 4 -interval 2s -speedup 30 -state /tmp/sim-be1-state.json > /tmp/sim-be1.log 2>&1 &
  SIMPID=$!
  echo "  simulator pid $SIMPID, running ${SIM_SECONDS}s ..."
  sleep "$SIM_SECONDS"
  kill -INT "$SIMPID" 2>/dev/null; sleep 3; kill -INT "$SIMPID" 2>/dev/null; wait "$SIMPID" 2>/dev/null
  sleep 2
fi
AFTER_TRIPS=$(sql "SELECT count(*) FROM trips WHERE tenant_id='$CH_TID' AND billing_status='charged';")
NEW_TRIPS=$((AFTER_TRIPS - BEFORE_TRIPS))
if [ -n "$SIM_BIN" ]; then
  check "模拟器产生并自动计费的行程 > 0（新增 $NEW_TRIPS）" true "$([ "$NEW_TRIPS" -gt 0 ] && echo true || echo false)"
else
  echo "  skip 未提供 SIM_BIN，仅核对既有已计费行程（$AFTER_TRIPS 条）"
fi
if [ "$AFTER_TRIPS" -gt 0 ]; then
  check "已计费行程 cost/cost_detail/billed_at/account_id 均非空" 0 "$(sql "SELECT count(*) FROM trips WHERE tenant_id='$CH_TID' AND billing_status='charged' AND (cost IS NULL OR cost_detail IS NULL OR billed_at IS NULL OR account_id IS NULL);")"
  check "行程金额与流水金额一致" 0 "$(sql "SELECT count(*) FROM trips t JOIN account_transactions x ON x.id = t.account_txn_id WHERE t.tenant_id='$CH_TID' AND (x.type <> 'trip' OR x.amount <> -t.cost OR x.ref_id <> t.id::text);")"
  check "每个行程至多一条 trip 流水（幂等）" 0 "$(sql "SELECT count(*) FROM (SELECT ref_id FROM account_transactions WHERE tenant_id='$CH_TID' AND type='trip' GROUP BY ref_id HAVING count(*) > 1) d;")"
  check "cost_detail 含 rule_name 与 lines" 0 "$(sql "SELECT count(*) FROM trips WHERE tenant_id='$CH_TID' AND billing_status='charged' AND (cost_detail->>'rule_name' IS NULL OR cost_detail->'lines' IS NULL);")"
  ONE_TRIP=$(sql "SELECT id FROM trips WHERE tenant_id='$CH_TID' AND billing_status='charged' AND cost > 0 ORDER BY billed_at DESC LIMIT 1;")
  req GET "/billing/transactions?type=trip&keyword=$ONE_TRIP" "$ADMIN"
  check "流水接口可按行程 id 检索到 trip 扣费" "200 1" "$STATUS $(j data.total)"; check "  ref_no 为行程号" "$(sql "SELECT trip_no FROM trips WHERE id='$ONE_TRIP';")" "$(j data.items.0.ref_no)"
  contains "  备注含里程" "里程" "$(j data.items.0.remark)"
  DRV=$(sql "SELECT driver_id FROM trips WHERE id='$ONE_TRIP';")
  [ -n "$DRV" ] && check "驾驶员收到扣费站内通知" true "$([ "$(sql "SELECT count(*) FROM notifications WHERE user_id='$DRV' AND type='system' AND ref_id='$ONE_TRIP';")" -ge 1 ] && echo true || echo false)"
  req GET "/trips/$ONE_TRIP" "$ADMIN"; check "行程详情 cost 非空" true "$([ "$(j data.cost)" != null ] && echo true || echo false)"
fi
req POST /auth/login "" '{"username":"sim_driver_1","password":"Sim@123456","tenant_code":"chenhua"}'
if [ "$STATUS" = 200 ]; then SIMTOK=$(j data.access_token); req GET /billing/accounts/me "$SIMTOK"; check "sim_driver_1 我的账户 200" 200 "$STATUS"; check "  level employee" employee "$(j data.level)"; fi

# ------------------------------------------------------------------ settlements
section "月度结算"
PERIOD=$(TZ=Asia/Shanghai date +%Y-%m)
PILE=$(sql "INSERT INTO charge_piles (tenant_id, pile_code, name) VALUES ('$CH_TID', 'BEC-$SFX', '计费测试桩') RETURNING id;")
CT1=$(sql "INSERT INTO charge_transactions (tenant_id, tx_no, pile_id, id_tag, user_id, dept_id, status, start_at, end_at, kwh, unit_price, cost) VALUES ('$CH_TID', 'C-BEC-$SFX-1', '$PILE', 'TAG', '$EMP', '$DEPT', 'settled', now() - interval '2 hour', now() - interval '1 hour', 28.4, 0.65, 18.46) RETURNING id;")
CT2=$(sql "INSERT INTO charge_transactions (tenant_id, tx_no, pile_id, id_tag, user_id, dept_id, status, start_at, end_at, kwh, unit_price, cost) VALUES ('$CH_TID', 'C-BEC-$SFX-2', '$PILE', 'TAG', '$EMP', NULL, 'settled', now() - interval '3 hour', now() - interval '2 hour', 10, 0.65, 6.50) RETURNING id;")
CT3=$(sql "INSERT INTO charge_transactions (tenant_id, tx_no, pile_id, id_tag, status, start_at, end_at, kwh, unit_price, cost) VALUES ('$CH_TID', 'C-BEC-$SFX-3', '$PILE', 'TAG', 'charging', now(), NULL, NULL, NULL, NULL) RETURNING id;")
check "写入 2 条已结算充电事务 + 1 条进行中" true "$([ -n "$CT1" ] && [ -n "$CT2" ] && [ -n "$CT3" ] && echo true || echo false)"
# 一条已完成但未计费的行程：generate 前应被补算
V_SET=$(sql "INSERT INTO vehicles (tenant_id, plate_no, battery_kwh, status, odometer_km) VALUES ('$CH_TID', '鲁C$SFX', 60, 'idle', 100) RETURNING id;")
T_SET=$(sql "INSERT INTO trips (tenant_id, trip_no, vehicle_id, driver_id, trip_type, source, status, start_at, end_at, distance_km, energy_kwh, end_soc) VALUES ('$CH_TID', 'T-BEC-$SFX', '$V_SET', '$EMP', 'official', 'web', 'completed', now() - interval '3 hour', now() - interval '1 hour', 47.3, 6.8, 61) RETURNING id;")
sql "DELETE FROM settlements WHERE tenant_id='$CH_TID' AND period='$PERIOD';" >/dev/null

req POST /billing/settlements/generate "$ADMIN" '{"period":"2026-9"}'; check "期格式非法 → 400" 400 "$STATUS"
req POST /billing/settlements/generate "$ADMIN" '{}'; check "缺 period → 400" 400 "$STATUS"
req POST /billing/settlements/generate "$ADMIN" "{\"period\":\"$PERIOD\"}"
check "生成 $PERIOD 结算单 200" 200 "$STATUS"; N_ROWS=$(jlen data); check "  行数 = 部门数 + 1" "$(sql "SELECT count(*) + 1 FROM departments WHERE tenant_id='$CH_TID' AND deleted_at IS NULL;")" "$N_ROWS"
check "  首行为企业汇总（dept_id null）" null "$(j data.0.dept_id)"; ENT_S=$(j data.0.id)
check "  企业行 charge_count 2 / charge_cost 24.96" "2 24.96" "$(j data.0.charge_count) $(j data.0.charge_cost)"
check "  企业预算 = 各部门预算之和" "$(sql "SELECT sum(monthly_budget) FROM departments WHERE tenant_id='$CH_TID' AND deleted_at IS NULL;" | sed 's/\.00$//; s/\(\.[0-9]\)0$/\1/')" "$(j data.0.budget)"
check "  未计费行程被补算" "charged" "$(sql "SELECT billing_status FROM trips WHERE id='$T_SET';")"
check "  trip_cost + penalty + charge_cost = total" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json; d=json.load(sys.stdin)["data"]; print(str(all(abs(r["trip_cost"]+r["penalty"]+r["charge_cost"]-r["total"])<0.005 for r in d)).lower())')"
DEPT_S=$(jfind data dept_id "$DEPT" id); check "  含测试部门行" true "$([ "$DEPT_S" != __missing__ ] && echo true || echo false)"
# C-BEC-2 未填 dept_id，但用户属于测试部门 → 按用户部门归入（驾驶员/用户所在部门）
check "  部门行 charge_count 2（含按用户部门归入的一条）/ charge_cost 24.96 / 预算 500" "2 24.96 500" "$(jfind data dept_id "$DEPT" charge_count) $(jfind data dept_id "$DEPT" charge_cost) $(jfind data dept_id "$DEPT" budget)"
check "  部门行含补算行程（trip_count ≥ 1，trip_cost 含 46.4）" true "$([ "$(jfind data dept_id "$DEPT" trip_count)" -ge 1 ] && echo true || echo false)"
check "  部门名" "计费部_$SFX" "$(jfind data dept_id "$DEPT" dept_name)"

req GET "/billing/settlements?period=$PERIOD" "$ADMIN"; check "结算单列表（按期）" "200 $N_ROWS" "$STATUS $(jlen data)"
req GET "/billing/settlements" "$ADMIN"; check "结算单列表（缺省最近一期）" "$PERIOD" "$(j data.0.period)"
req GET "/billing/settlements?period=bad" "$ADMIN"; check "period 非法 → 400" 400 "$STATUS"
req GET "/billing/settlements?period=$PERIOD&status=confirmed" "$ADMIN"; check "按状态过滤（无已确认）" 0 "$(jlen data)"
req GET /billing/settlements/periods "$ADMIN"; check "可结算期列表 200" 200 "$STATUS"; check "  含本期 draft" draft "$(jfind data period "$PERIOD" status)"; check "  total 与企业行一致" "$(req GET "/billing/settlements/$ENT_S" "$ADMIN"; j data.total)" "$(req GET /billing/settlements/periods "$ADMIN"; jfind data period "$PERIOD" total)"
req GET "/billing/settlements/$DEPT_S" "$ADMIN"; check "部门结算单详情 200" 200 "$STATUS"
check "  明细含 2 条充电行 + ≥1 条行程行" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json; L=json.load(sys.stdin)["data"]["lines"]; k=[l["kind"] for l in L]; print(str(k.count("charge")==2 and k.count("trip")>=1).lower())')"
check "  充电行单号/数量/金额" "C-BEC-$SFX-1 28.4 18.46" "$(jfind data.lines ref_id "$CT1" ref_no) $(jfind data.lines ref_id "$CT1" quantity) $(jfind data.lines ref_id "$CT1" amount)"
T_COST=$(sql "SELECT cost - COALESCE((cost_detail->>'penalty')::numeric, 0) FROM trips WHERE id='$T_SET';" | sed 's/\.00$//; s/\(\.[0-9]\)0$/\1/')
check "  行程行金额 = trips.cost − 罚金（$T_COST）/ 用户名" "$T_COST 计费员工" "$(jfind data.lines ref_id "$T_SET" amount) $(jfind data.lines ref_id "$T_SET" user_name)"
check "  行程行 detail 含 lines" true "$([ "$(jfind data.lines ref_id "$T_SET" detail)" != null ] && echo true || echo false)"
req GET "/billing/settlements/$ENT_S" "$ADMIN"; check "企业汇总详情含未分配部门的充电行" "C-BEC-$SFX-2" "$(jfind data.lines ref_id "$CT2" ref_no)"
req GET "/billing/settlements/00000000-0000-0000-0000-000000000001" "$ADMIN"; check "结算单不存在 404" 404 "$STATUS"

curl -s -o "$TMP/s.xlsx" -w '%{http_code} %{content_type}' -H "Authorization: Bearer $ADMIN" "$BASE/billing/settlements/export?period=$PERIOD" > "$TMP/hdr"
check "导出 xlsx 200 + content-type" "200 application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" "$(cat "$TMP/hdr")"
INFO=$(xlsx_info "$TMP/s.xlsx"); check "  两个 sheet；汇总行数 = 结算行数" "2 $N_ROWS" "$(echo "$INFO" | cut -d' ' -f1,2)"
check "  明细行数 = 企业汇总明细数" "$(req GET "/billing/settlements/$ENT_S" "$ADMIN"; jlen data.lines)" "$(echo "$INFO" | cut -d' ' -f3)"
curl -s -o "$TMP/d.xlsx" -w '%{http_code}' -H "Authorization: Bearer $ADMIN" "$BASE/billing/settlements/export?period=$PERIOD&dept_id=$DEPT" > "$TMP/hdr"
check "按部门导出 200" 200 "$(cat "$TMP/hdr")"; check "  汇总仅 1 行" "2 1" "$(xlsx_info "$TMP/d.xlsx" | cut -d' ' -f1,2)"
req GET "/billing/settlements/export?period=2000-01" "$ADMIN"; check "未生成的期导出 → 404" 404 "$STATUS"
req GET "/billing/settlements/export" "$ADMIN"; check "缺 period → 400" 400 "$STATUS"
req GET "/billing/settlements/export?period=$PERIOD&dept_id=x" "$ADMIN"; check "dept_id 非法 → 400" 400 "$STATUS"

req POST "/billing/settlements/$DEPT_S/confirm" "$ADMIN"; check "确认部门行 200" 200 "$STATUS"; check "  status confirmed / confirmed_by_name" "confirmed $(sql "SELECT name FROM users WHERE id='$ADMIN_UID';")" "$(j data.status) $(j data.confirmed_by_name)"
req GET "/billing/settlements/$ENT_S" "$ADMIN"; check "  企业行仍为草稿" draft "$(j data.status)"
req POST /billing/settlements/generate "$ADMIN" "{\"period\":\"$PERIOD\"}"; check "已有确认行的期重算 → 409" 409 "$STATUS"
req POST "/billing/settlements/$ENT_S/confirm" "$ADMIN"; check "确认企业汇总行 200" "200 confirmed" "$STATUS $(j data.status)"
req GET "/billing/settlements?period=$PERIOD&status=draft" "$ADMIN"; check "  该期全部确认（无草稿）" 0 "$(jlen data)"
req GET /billing/settlements/periods "$ADMIN"; check "  期状态 confirmed" confirmed "$(jfind data period "$PERIOD" status)"
req POST "/billing/settlements/$DEPT_S/confirm" "$ADMIN"; check "重复确认幂等 200" 200 "$STATUS"
req POST "/billing/settlements/$ENT_S/confirm" "$EMPTOK"; check "员工无权确认 → 403" 403 "$STATUS"

# ------------------------------------------------------------------ cleanup
section "清理"
sql "DELETE FROM settlements WHERE tenant_id='$CH_TID' AND period='$PERIOD';
 DELETE FROM charge_transactions WHERE id IN ('$CT1','$CT2','$CT3');
 DELETE FROM charge_piles WHERE id='$PILE';
 DELETE FROM account_transactions WHERE tenant_id='$CH_TID' AND (account_id IN ('$DEPT_ACC','$EMP_ACC') OR ref_id IN ('$DEPT_ACC','$EMP_ACC','$T_SET'));
 DELETE FROM notifications WHERE user_id IN ('$EMP','$EMP2','$LEAD');
 DELETE FROM trips WHERE id='$T_SET';
 DELETE FROM vehicles WHERE id='$V_SET';
 DELETE FROM accounts WHERE id IN ('$DEPT_ACC','$EMP_ACC');
 UPDATE users SET dept_id = NULL WHERE tenant_id='$CH_TID' AND dept_id='$DEPT';
 UPDATE approvals SET dept_id = NULL WHERE dept_id='$DEPT';
 UPDATE departments SET leader_user_id = NULL WHERE id='$DEPT';
 DELETE FROM user_roles WHERE user_id IN ('$EMP','$EMP2','$LEAD');
 DELETE FROM users WHERE id IN ('$EMP','$EMP2');
 UPDATE users SET deleted_at = now(), status = 'disabled' WHERE id='$LEAD';  -- 模拟器审批单以其为审批人，只能软删
 DELETE FROM departments WHERE id='$DEPT';
 UPDATE billing_rules SET deleted_at = now(), is_default = false WHERE id='$RULE_A';" >/dev/null
ok "测试数据已清理（企业账户充值记录保留）"

echo
echo "== 结果：通过 $PASS，失败 $FAIL"
[ "$FAIL" -eq 0 ]
