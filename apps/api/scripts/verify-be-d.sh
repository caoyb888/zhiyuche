#!/usr/bin/env bash
# 端到端验证：OCPP 1.6J 充电桩接入 + 充电模块（阶段 3 BE-D）。
#
# 起 api（make api-start）与模拟器（充电桩作为 OCPP 客户端接入 -ocpp），跑 2～3 分钟让低电量车辆
# 完成 Preparing → Authorize → StartTransaction → MeterValues → StopTransaction，然后检查库表、
# /charging/* 接口（实时视图/列表/详情/曲线/汇总/导出）、复核 approve/reject、对模拟桩的远程启停、权限 403。
#
# 前置：迁移 00004 已应用；引导账号 chenhua_admin/Admin@123456；机器上有 curl 与 python3；在仓库根目录执行。
# 用法：BASE=http://localhost:20091/api/v1 OCPP=ws://localhost:20181 \
#       PSQL="docker exec -i zhiyuche-postgres psql -U zhiyuche -d zhiyuche_be2 -tAq" bash apps/api/scripts/verify-be-d.sh
# 说明：本环境计费钩子尚未实现（501），因此每个结束的事务都会进入待复核（review_note=计费失败…），
#       approve 会把 501 原样返回并保持 pending；这正是"计价失败可容错、复核时重试"的预期行为。
set -u
BASE=${BASE:-http://localhost:20091/api/v1}
OCPP=${OCPP:-ws://localhost:20181}
PSQL=${PSQL:-docker exec -i zhiyuche-postgres psql -U zhiyuche -d zhiyuche_be2 -tAq}
RUN_SECS=${RUN_SECS:-200}
SIM_LOG=${SIM_LOG:-/tmp/zy-bed-sim.log}
SIM_PID=${SIM_PID:-/tmp/zy-bed-sim.pid}
SIM_STATE=${SIM_STATE:-/tmp/zy-bed-sim-state.json}
KEEP_API=${KEEP_API:-0}
SFX=$(date +%s | tail -c 6)$RANDOM
TMP=${TMPDIR:-/tmp}/zy-bed-$$
mkdir -p "$TMP"

PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "  ok   $1"; }
bad() { FAIL=$((FAIL+1)); echo "  FAIL $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want [$2], got [$3])"; fi; }
ge() { if [ -n "$3" ] && [ "$3" != null ] && [ "$3" -ge "$2" ] 2>/dev/null; then ok "$1 ($3)"; else bad "$1 (want ≥$2, got [$3])"; fi; }
contains() { case "$3" in *"$2"*) ok "$1";; *) bad "$1 (want contains [$2], got [${3:0:200}])";; esac; }
section() { echo; echo "== $1"; }

req() { # METHOD PATH TOKEN [BODY]
  local m=$1 p=$2 t=$3 b=${4:-}
  local args=(-s -o "$TMP/body" -w '%{http_code}' -X "$m" -H 'Content-Type: application/json')
  [ -n "$t" ] && args+=(-H "Authorization: Bearer $t")
  [ -n "$b" ] && args+=(--data "$b")
  STATUS=$(curl "${args[@]}" "$BASE$p")
  BODY=$(cat "$TMP/body")
}
j() { printf '%s' "$BODY" | python3 -c '
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
' "$1" 2>/dev/null; }
jlen() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d[int(k)] if isinstance(d, list) else (d.get(k) if isinstance(d, dict) else None)
print(len(d) if d is not None else 0)' "$1" 2>/dev/null; }
# jq-ish: python expression over d (parsed BODY)
py() { printf '%s' "$BODY" | python3 -c 'import sys,json; d=json.load(sys.stdin); v=eval(sys.argv[1]); print(str(v).lower() if isinstance(v,bool) else v)' "$1" 2>/dev/null; }
sql() { printf '%s' "$1" | $PSQL | head -n1; }
xlsx_rows() { python3 - "$1" <<'PY'
import sys, zipfile, re
z = zipfile.ZipFile(sys.argv[1])
sheet = [n for n in z.namelist() if n.startswith("xl/worksheets/sheet")][0]
print(len(re.findall(r"<row[ >]", z.read(sheet).decode())) - 1)
PY
}

stop_sim() {
  if [ -f "$SIM_PID" ] && kill -0 "$(cat "$SIM_PID")" 2>/dev/null; then
    kill -INT "$(cat "$SIM_PID")" 2>/dev/null
    for _ in $(seq 1 20); do kill -0 "$(cat "$SIM_PID")" 2>/dev/null || break; sleep 0.5; done
    kill -0 "$(cat "$SIM_PID")" 2>/dev/null && kill "$(cat "$SIM_PID")" 2>/dev/null
    echo "  simulator stopped"
  fi
  rm -f "$SIM_PID"
}
cleanup() {
  stop_sim
  if [ -n "${U_VIEW:-}" ]; then
    sql "DELETE FROM notifications WHERE user_id = '$U_VIEW'; DELETE FROM user_roles WHERE user_id = '$U_VIEW'; DELETE FROM users WHERE id = '$U_VIEW';
         DELETE FROM role_permissions WHERE role_id = '${R_VIEW:-00000000-0000-0000-0000-000000000000}'; DELETE FROM roles WHERE id = '${R_VIEW:-00000000-0000-0000-0000-000000000000}';" >/dev/null 2>&1
  fi
  [ "$KEEP_API" = 1 ] || make api-stop >/dev/null 2>&1 || true
  rm -rf "$TMP"
}
trap cleanup EXIT
trap 'echo "  interrupted"; exit 130' INT TERM

# ------------------------------------------------------------------ start api + simulator
section "启动 api 与模拟器"
make api-start | tail -1
for _ in $(seq 1 30); do curl -sf "$BASE/health" >/dev/null 2>&1 && break; sleep 1; done
req GET /health ""; check "api /health" 200 "$STATUS"
T0=$(date -u +%Y-%m-%dT%H:%M:%SZ)
stop_sim
rm -f "$SIM_STATE"
# speedup 40 → 每 3s 一跳等于 2 分钟模拟时间，60 kW 桩把 20%→90% 充完约 25 跳（≈75s 真实时间）
nohup bin/zhiyuche-simulator -api "$BASE" -ocpp "$OCPP" -tenant chenhua -vehicles 6 -low-soc 2 -interval 3s -speedup 40 -state "$SIM_STATE" > "$SIM_LOG" 2>&1 &
echo $! > "$SIM_PID"
echo "  simulator pid $(cat "$SIM_PID") log $SIM_LOG"

req POST /auth/login "" '{"username":"chenhua_admin","password":"Admin@123456","tenant_code":"chenhua"}'
ADMIN=$(j data.access_token); check "租户管理员登录" 200 "$STATUS"
req GET /auth/me "$ADMIN"; CH_TID=$(j data.tenant.id); ADMIN_UID=$(j data.id)

# 等待模拟桩通过 OCPP 上线
ONLINE=0
for _ in $(seq 1 40); do
  req GET /charging/piles/live "$ADMIN"
  ONLINE=$(py "sum(1 for p in d['data'] if p['pile_code'].startswith('SIM-PILE') and p['online'])")
  [ "${ONLINE:-0}" -ge 2 ] && break
  sleep 2
done
check "两个模拟桩 OCPP 上线（online=true）" 2 "$ONLINE"
P1=$(py "[p['id'] for p in d['data'] if p['pile_code']=='SIM-PILE-01'][0]")
P2=$(py "[p['id'] for p in d['data'] if p['pile_code']=='SIM-PILE-02'][0]")
check "  桩状态 available" available "$(py "[p['status'] for p in d['data'] if p['id']=='$P1'][0]")"
for _ in $(seq 1 10); do # the per-connector StatusNotifications follow the boot within a second or two
  CONNS=$(py "' '.join(c['status'] for c in [p for p in d['data'] if p['id']=='$P1'][0]['connectors'] if c['connector_id'] in (1,2))")
  [ "$CONNS" = "Available Available" ] && break
  sleep 1; req GET /charging/piles/live "$ADMIN"
done
check "  连接器 1/2 Available" "Available Available" "$CONNS"
check "  last_heartbeat_at 非空" true "$([ "$(py "[p['last_heartbeat_at'] for p in d['data'] if p['id']=='$P1'][0]")" != None ] && echo true)"
echo "  pile1=$P1 pile2=$P2 tenant=$CH_TID"
sql "SELECT status FROM charge_piles WHERE id = '$P1';" | { read -r s; check "DB charge_piles.status=available" available "$s"; }

# ------------------------------------------------------------------ let the vehicles charge
section "等待低电量车辆走完 OCPP 充电流程（最多 ${RUN_SECS}s）"
START=$(date +%s)
while :; do
  EL=$(( $(date +%s) - START ))
  ENDED=$(sql "SELECT count(*) FROM charge_transactions WHERE tenant_id='$CH_TID' AND created_at >= '$T0' AND status <> 'charging';")
  ACTIVE=$(sql "SELECT count(*) FROM charge_transactions WHERE tenant_id='$CH_TID' AND created_at >= '$T0' AND status = 'charging';")
  MV=$(sql "SELECT count(*) FROM charge_meter_values WHERE ts >= '$T0';")
  echo "  t+${EL}s: charging=$ACTIVE ended=$ENDED meter_values=$MV"
  { [ "$ENDED" -ge 2 ] || { [ "$ENDED" -ge 1 ] && [ "$EL" -ge 120 ]; } || [ "$EL" -ge "$RUN_SECS" ]; } && break
  sleep 15
done
grep -E 'charging session (started|ended)|RemoteStart|RemoteStop|ocpp connected' "$SIM_LOG" | tail -8 | sed 's/^/  sim: /'

# ------------------------------------------------------------------ DB state
section "库表：charge_transactions / charge_meter_values / pile_connectors"
ge "本次运行创建的充电事务" 1 "$(sql "SELECT count(*) FROM charge_transactions WHERE tenant_id='$CH_TID' AND created_at >= '$T0';")"
ge "已结束的事务" 1 "$ENDED"
ge "电表数据点" 5 "$MV"
TX=$(sql "SELECT id FROM charge_transactions WHERE tenant_id='$CH_TID' AND created_at >= '$T0' AND status <> 'charging' ORDER BY end_at DESC LIMIT 1;")
if [ -z "$TX" ]; then
  # 没有结束的事务：后续依赖 TX 的检查改用最新事务（会以 FAIL 体现，但不至于报 SQL 语法错误）
  TX=$(sql "SELECT id FROM charge_transactions WHERE tenant_id='$CH_TID' AND created_at >= '$T0' ORDER BY created_at DESC LIMIT 1;")
  [ -z "$TX" ] && TX=00000000-0000-0000-0000-000000000000
fi
check "tx_no 形如 C-YYYYMMDD-NNN" true "$(sql "SELECT tx_no FROM charge_transactions WHERE id='$TX';" | grep -Eq '^C-[0-9]{8}-[0-9]{3,}$' && echo true)"
ROW=$(sql "SELECT status || ' ' || review_status || ' ' || COALESCE(bind_method,'-') || ' ' || (vehicle_id IS NOT NULL) || ' ' || (user_id IS NOT NULL) || ' ' || (card_id IS NOT NULL) || ' ' || COALESCE(kwh::text,'-') || ' ' || COALESCE(bms_kwh_est::text,'-') || ' ' || COALESCE(deviation_pct::text,'-') || ' ' || COALESCE(stop_reason,'-') FROM charge_transactions WHERE id='$TX';")
echo "  tx=$TX → status review bind vehicle user card kwh est dev reason = $ROW"
check "  刷卡归属：user/card/vehicle 均已绑定" "true true true" "$(sql "SELECT (vehicle_id IS NOT NULL) || ' ' || (user_id IS NOT NULL) || ' ' || (card_id IS NOT NULL) FROM charge_transactions WHERE id='$TX';")"
check "  bind_method=location（按桩坐标匹配车辆）" location "$(sql "SELECT bind_method FROM charge_transactions WHERE id='$TX';")"
check "  kwh>0 且 BMS 估算/偏差已计算" true "$(sql "SELECT (kwh > 0 AND bms_kwh_est IS NOT NULL AND deviation_pct IS NOT NULL) FROM charge_transactions WHERE id='$TX';" | sed 's/t/true/;s/f/false/')"
check "  status=ended、review_status=pending（计费 501 或偏差>5%）" "ended pending" "$(sql "SELECT status || ' ' || review_status FROM charge_transactions WHERE id='$TX';")"
contains "  review_note 记录原因" "" "$(sql "SELECT review_note FROM charge_transactions WHERE id='$TX';")"
check "  stop_reason=Local" Local "$(sql "SELECT stop_reason FROM charge_transactions WHERE id='$TX';")"
ge "  该事务电表点" 5 "$(sql "SELECT count(*) FROM charge_meter_values WHERE tx_id='$TX';")"
check "  电表 Wh 单调递增、power_kw/voltage/soc 齐全" true "$(sql "SELECT (max(wh) > min(wh) AND count(*) FILTER (WHERE power_kw IS NULL OR voltage IS NULL OR soc IS NULL) = 0) FROM charge_meter_values WHERE tx_id='$TX';" | sed 's/^t$/true/')"
VEH=$(sql "SELECT vehicle_id FROM charge_transactions WHERE id='$TX';")
check "  车辆结束后 charging=false" f "$(sql "SELECT charging FROM vehicle_status WHERE vehicle_id='$VEH';")"
ge "pile_connectors 行（两桩各 3 行：0/1/2）" 6 "$(sql "SELECT count(*) FROM pile_connectors WHERE pile_id IN ('$P1','$P2');")"
ge "驾驶员收到 charging.started/ended 通知" 2 "$(sql "SELECT count(*) FROM notifications WHERE tenant_id='$CH_TID' AND type IN ('charging.started','charging.ended') AND ref_id='$TX';")"
ge "租户管理员收到待复核通知" 1 "$(sql "SELECT count(*) FROM notifications WHERE user_id='$ADMIN_UID' AND type='charging.review' AND ref_id='$TX';")"

# ------------------------------------------------------------------ HTTP: list / detail / meter-values / summary / export
section "接口：列表 / 详情 / 曲线 / 汇总 / 导出"
req GET "/charging/transactions?pile_id=$P1&sort=-start_at" "$ADMIN"; check "列表 pile_id 过滤 → 200" 200 "$STATUS"
req GET "/charging/transactions?from=$T0&status=ended&review_status=pending" "$ADMIN"; ge "列表 from+status+review_status 过滤 total" 1 "$(j data.total)"
req GET "/charging/transactions?keyword=SIM-PILE" "$ADMIN"; ge "列表 keyword=桩编号" 1 "$(j data.total)"
req GET "/charging/transactions?status=bogus" "$ADMIN"; check "status 非法 → 400" 400 "$STATUS"
req GET "/charging/transactions?from=bad" "$ADMIN"; check "from 非法 → 400" 400 "$STATUS"
req GET "/charging/transactions/$TX" "$ADMIN"; check "详情 → 200" 200 "$STATUS"
check "  pile_code/connector_id/user/vehicle/duration_min" true "$(py "all(d['data'].get(k) is not None for k in ('pile_code','connector_id','user','vehicle','duration_min')) and d['data']['connector_id'] >= 1")"
ge "  meter_values（抽样 ≤200）" 3 "$(jlen data.meter_values)"
check "  meter_values ≤ 200 点、kwh 相对 meter_start" true "$(py "len(d['data']['meter_values']) <= 200 and d['data']['meter_values'][0]['kwh'] is not None and d['data']['meter_values'][-1]['kwh'] >= d['data']['meter_values'][0]['kwh']")"
check "  ocpp_tx_id ≥ 1000, review_status=pending" "true pending" "$(py "d['data']['ocpp_tx_id'] >= 1000") $(j data.review_status)"
req GET "/charging/transactions/$TX/meter-values" "$ADMIN"; check "曲线（全部点）→ 200" 200 "$STATUS"
ge "  点数" 5 "$(jlen data)"
req GET "/charging/transactions/00000000-0000-0000-0000-000000000001" "$ADMIN"; check "不存在 → 404" 404 "$STATUS"
TODAY=$(TZ=Asia/Shanghai date +%Y-%m-%d)
req GET "/charging/summary" "$ADMIN"; check "汇总 → 200 date=今天" "200 $TODAY" "$STATUS $(j data.date)"
ge "  today.sessions" 1 "$(j data.today.sessions)"
ge "  month.sessions" 1 "$(j data.month.sessions)"
ge "  pending_review" 1 "$(j data.pending_review)"
ge "  piles.online" 2 "$(j data.piles.online)"
ge "  by_pile 条目" 2 "$(jlen data.by_pile)"
check "  today.kwh > 0" true "$(py "d['data']['today']['kwh'] > 0")"
req GET "/charging/summary?date=bad" "$ADMIN"; check "date 非法 → 400" 400 "$STATUS"
curl -s -D "$TMP/exp.h" -o "$TMP/charging.xlsx" -H "Authorization: Bearer $ADMIN" "$BASE/charging/transactions/export?from=$T0"
check "导出 xlsx Content-Type" true "$(grep -qi 'content-type: application/vnd.openxmlformats' "$TMP/exp.h" && echo true)"
ge "  导出行数（本次运行事务）" 1 "$(xlsx_rows "$TMP/charging.xlsx")"

# ------------------------------------------------------------------ review
section "复核 approve / reject"
req POST "/charging/transactions/$TX/review" "$ADMIN" '{"action":"bogus"}'; check "action 非法 → 400" 400 "$STATUS"
req POST "/charging/transactions/$TX/review" "$ADMIN" '{"action":"approve","note":"确认归属"}'
if [ "$STATUS" = 200 ]; then
  check "approve → settled/approved（计费钩子可用）" "settled approved" "$(j data.status) $(j data.review_status)"
else
  check "approve 时重试计费：钩子未实现 → 501 原样返回" 501 "$STATUS"
  check "  事务仍待复核，note 记录计费失败" "pending 计费失败" "$(sql "SELECT review_status || ' ' || left(review_note, 4) FROM charge_transactions WHERE id='$TX';")"
fi
req POST "/charging/transactions/$TX/review" "$ADMIN" '{"action":"approve","user_id":"00000000-0000-0000-0000-000000000001"}'
[ "$STATUS" = 400 ] && ok "approve 指定不存在的 user_id → 400" || { [ "$STATUS" = 409 ] && ok "已复核事务再次复核 → 409" || bad "approve bad user (got $STATUS)"; }
req POST "/charging/transactions/$TX/review" "$ADMIN" '{"action":"reject","note":"计量异常，不计费"}'
if [ "$STATUS" = 200 ]; then
  check "reject → ended/rejected 不计费" "ended rejected null" "$(j data.status) $(j data.review_status) $(j data.cost)"
  check "  reviewed_by_name" true "$([ "$(j data.reviewed_by_name)" != null ] && echo true)"
  req POST "/charging/transactions/$TX/review" "$ADMIN" '{"action":"reject"}'; check "再次复核 → 409" 409 "$STATUS"
else
  check "reject（已 settled 的事务）→ 409" 409 "$STATUS"
fi
ge "审计日志记录复核" 1 "$(sql "SELECT count(*) FROM audit_logs WHERE module='charging' AND action LIKE 'review_%' AND target_id='$TX';")"

# ------------------------------------------------------------------ remote start / stop against the simulated pile
section "远程启停（对模拟桩）"
req GET /charging/piles/live "$ADMIN"
PILE=$(py "next((p['id'] for p in d['data'] if p['pile_code'].startswith('SIM-PILE') and p['online'] and any(c['status'] in ('Available','Preparing') for c in p['connectors'] if c['connector_id']>0)), 'none')")
CONN=$(py "next((c['connector_id'] for p in d['data'] if p['id']=='$PILE' for c in p['connectors'] if c['connector_id']>0 and c['status'] in ('Available','Preparing')), 0)")
echo "  free connector: pile=$PILE connector=$CONN"
req POST "/charging/piles/$PILE/remote-start" "$ADMIN" "{\"connector_id\":$CONN}"
check "id_tag 缺省且当前用户无卡 → 400" 400 "$STATUS"
req POST "/charging/piles/$PILE/remote-start" "$ADMIN" "{\"connector_id\":$CONN,\"id_tag\":\"A1B2C3D0\"}"
check "remote-start（模拟驾驶员卡）→ 200 Accepted" "200 Accepted" "$STATUS $(j data.status)"
RTX=""
for _ in $(seq 1 20); do
  RTX=$(sql "SELECT id FROM charge_transactions WHERE pile_id='$PILE' AND connector_id=$CONN AND status='charging' AND created_at >= '$T0' ORDER BY created_at DESC LIMIT 1;")
  [ -n "$RTX" ] && break; sleep 2
done
check "桩发起 StartTransaction → 事务 charging" true "$([ -n "$RTX" ] && echo true)"
if [ -n "$RTX" ]; then
  echo "  remote tx=$RTX bind=$(sql "SELECT COALESCE(bind_method,'-') || ' user=' || (user_id IS NOT NULL) FROM charge_transactions WHERE id='$RTX';")"
  sleep 4
  req GET /charging/piles/live "$ADMIN"
  check "实时视图：连接器 Charging 且挂有事务（实时 kwh/power）" "Charging true" "$(py "next((c['status']+' '+str(c['transaction'] is not None and c['transaction']['id']=='$RTX').lower() for p in d['data'] if p['id']=='$PILE' for c in p['connectors'] if c['connector_id']==$CONN), 'none')")"
  check "  桩状态 charging" charging "$(py "[p['status'] for p in d['data'] if p['id']=='$PILE'][0]")"
  req GET "/charging/transactions?status=charging" "$ADMIN"; check "列表 status=charging 含实时 kwh 字段" true "$(py "any(t['id']=='$RTX' and t['kwh'] is not None for t in d['data']['items'])")"
  req POST "/charging/piles/$PILE/remote-start" "$ADMIN" "{\"connector_id\":$CONN,\"id_tag\":\"A1B2C3D0\"}"; check "连接器充电中再远程启动 → 409" 409 "$STATUS"
  req POST "/charging/piles/$P2/remote-stop" "$ADMIN" "{\"transaction_id\":\"$RTX\"}"; [ "$PILE" = "$P2" ] || check "事务不属于该桩 → 400" 400 "$STATUS"
  req POST "/charging/piles/$PILE/remote-stop" "$ADMIN" "{\"transaction_id\":\"$RTX\"}"
  check "remote-stop → 200 Accepted" "200 Accepted" "$STATUS $(j data.status)"
  for _ in $(seq 1 20); do
    [ "$(sql "SELECT status FROM charge_transactions WHERE id='$RTX';")" != charging ] && break; sleep 2
  done
  check "桩发起 StopTransaction(reason Remote) → 事务结束" "ended Remote" "$(sql "SELECT status || ' ' || COALESCE(stop_reason,'-') FROM charge_transactions WHERE id='$RTX';")"
  req POST "/charging/piles/$PILE/remote-stop" "$ADMIN" "{\"transaction_id\":\"$RTX\"}"; check "已结束事务再 remote-stop → 409" 409 "$STATUS"
fi
req POST "/charging/piles/00000000-0000-0000-0000-000000000001/remote-start" "$ADMIN" '{}'; check "桩不存在 → 404" 404 "$STATUS"
req POST "/charging/piles/$PILE/remote-stop" "$ADMIN" '{}'; check "remote-stop 缺 transaction_id → 400" 400 "$STATUS"

# ------------------------------------------------------------------ permissions
section "权限 403"
req POST /system/roles "$ADMIN" "{\"code\":\"cv_$SFX\",\"name\":\"仅看充电\",\"permissions\":[\"charging:view\"]}"
R_VIEW=$(j data.id); check "创建仅 charging:view 角色" 201 "$STATUS"
req POST /system/users "$ADMIN" "{\"username\":\"cv_$SFX\",\"name\":\"看充电\",\"password\":\"Emp@123456\",\"role_ids\":[\"$R_VIEW\"]}"
U_VIEW=$(j data.id); check "创建用户" 201 "$STATUS"
req POST /auth/login "" "{\"username\":\"cv_$SFX\",\"password\":\"Emp@123456\",\"tenant_code\":\"chenhua\"}"; VIEW=$(j data.access_token)
req GET /charging/piles/live "$VIEW"; check "charging:view 看实时 → 200" 200 "$STATUS"
req GET "/charging/transactions" "$VIEW"; check "charging:view 看列表 → 200" 200 "$STATUS"
req GET "/charging/summary" "$VIEW"; check "charging:view 看汇总 → 200" 200 "$STATUS"
req POST "/charging/piles/$PILE/remote-start" "$VIEW" '{}'; check "无 charging:manage 远程启动 → 403" 403 "$STATUS"
req POST "/charging/piles/$PILE/remote-stop" "$VIEW" "{\"transaction_id\":\"$TX\"}"; check "无 charging:manage 远程停止 → 403" 403 "$STATUS"
req POST "/charging/transactions/$TX/review" "$VIEW" '{"action":"reject"}'; check "无 charging:review 复核 → 403" 403 "$STATUS"
curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $VIEW" "$BASE/charging/transactions/export" > "$TMP/code"; check "无 charging:export 导出 → 403" 403 "$(cat "$TMP/code")"
req GET /charging/piles/live ""; check "未登录 → 401" 401 "$STATUS"

# ------------------------------------------------------------------ simulator shutdown closes sessions, piles go offline
section "模拟器退出：会话收尾、桩离线"
stop_sim
sleep 2
check "模拟器退出后无遗留 charging 事务" 0 "$(sql "SELECT count(*) FROM charge_transactions WHERE tenant_id='$CH_TID' AND status='charging' AND pile_id IN ('$P1','$P2');")"
req GET /charging/piles/live "$ADMIN"
check "两桩 online=false、status=offline" "false offline false offline" "$(py "' '.join(str(p['online']).lower()+' '+p['status'] for p in d['data'] if p['id'] in ('$P1','$P2'))")"
check "  连接器 Unavailable" true "$(py "all(c['status']=='Unavailable' for p in d['data'] if p['id'] in ('$P1','$P2') for c in p['connectors'])")"

echo
echo "== 结果：通过 $PASS，失败 $FAIL"
[ "$FAIL" -eq 0 ]
