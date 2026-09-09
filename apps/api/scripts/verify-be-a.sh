#!/usr/bin/env bash
# 端到端验证：资产四模块（车辆 / 网关设备 / NFC 卡 / 充电桩）与网关接入 /ingest
# （对照 openapi.yaml 的 vehicles、devices、cards、piles、ingest）。
# 前提：API 已启动并完成引导（admin/Admin@123456、chenhua_admin/Admin@123456）；机器上有 curl 与 python3。
# 用法：BASE=http://localhost:20090/api/v1 bash apps/api/scripts/verify-be-a.sh
set -u
BASE=${BASE:-http://localhost:20090/api/v1}
ADMIN_PW=${ADMIN_PW:-Admin@123456}
SUF=$(date +%s)              # 每次运行使用不同的车牌/序列号/卡号，可重复执行
PASS=0; FAIL=0
TMP=$(mktemp); trap 'rm -f "$TMP"' EXIT
STATUS=""; BODY=""

# req METHOD PATH [TOKEN] [JSON_BODY] [EXTRA_HEADER]
req() {
  local m=$1 p=$2 tok=${3:-} body=${4:-} hdr=${5:-}
  local args=(-s -o "$TMP" -w '%{http_code}' -X "$m" "$BASE$p" -H 'Content-Type: application/json')
  [ -n "$tok" ] && args+=(-H "Authorization: Bearer $tok")
  [ -n "$hdr" ] && args+=(-H "$hdr")
  [ -n "$body" ] && args+=(--data-binary "$body")
  STATUS=$(curl "${args[@]}")
  BODY=$(cat "$TMP")
}
# dreq SERIAL KEY METHOD PATH [JSON_BODY] —— 网关设备鉴权请求
dreq() {
  local serial=$1 key=$2 m=$3 p=$4 body=${5:-}
  local args=(-s -o "$TMP" -w '%{http_code}' -X "$m" "$BASE$p" -H 'Content-Type: application/json')
  [ -n "$serial" ] && args+=(-H "X-Device-Serial: $serial")
  [ -n "$key" ] && args+=(-H "X-Device-Key: $key")
  [ -n "$body" ] && args+=(--data-binary "$body")
  STATUS=$(curl "${args[@]}")
  BODY=$(cat "$TMP")
}
# j PY_EXPR —— 用 python 从 $BODY 取值，d 为解析后的 JSON
j() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$1" <<<"$BODY" 2>/dev/null; }
check() { # check NAME WANT GOT
  if [ "$2" == "$3" ]; then PASS=$((PASS+1)); echo "  PASS  $1"
  else FAIL=$((FAIL+1)); echo "  FAIL  $1: want [$2] got [$3]"; echo "        body: ${BODY:0:300}"; fi
}
section() { echo; echo "== $1"; }
TOKEN=""
login() { # login USER PW [TENANT_CODE] -> sets TOKEN
  local tc=${3:-}
  req POST /auth/login "" "{\"username\":\"$1\",\"password\":\"$2\",\"tenant_code\":\"$tc\"}"
  TOKEN=$(j "d['data']['access_token']")
}
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
T1=$(date -u -d '-10 seconds' +%Y-%m-%dT%H:%M:%SZ)
FROM=$(date -u -d '-1 hour' +%Y-%m-%dT%H:%M:%SZ)
TO=$(date -u -d '+1 hour' +%Y-%m-%dT%H:%M:%SZ)
OLD=$(date -u -d '-8 days' +%Y-%m-%dT%H:%M:%SZ)
NIL=00000000-0000-0000-0000-000000000001

# ---------------------------------------------------------------- 登录
section "登录"
login admin "$ADMIN_PW" platform;        ADMIN=$TOKEN; check "超级管理员登录" 200 "$STATUS"
login chenhua_admin "$ADMIN_PW" chenhua; CH=$TOKEN;    check "租户管理员登录" 200 "$STATUS"
CH_TID=$(j "d['data']['user']['tenant']['id']"); CH_UID=$(j "d['data']['user']['id']")
echo "  chenhua tenant=$CH_TID admin user=$CH_UID"

# ---------------------------------------------------------------- 车辆
section "车辆：创建 / 列表 / 详情"
req POST /assets/vehicles "$CH" "{\"plate_no\":\"鲁A·T$SUF\",\"vin\":\"VIN$SUF\",\"brand\":\"比亚迪\",\"model\":\"e6\",\"battery_kwh\":60,\"range_km_full\":400,\"purchase_date\":\"2024-05-01\"}"
check "创建车辆 201" 201 "$STATUS"; V1=$(j "d['data']['id']")
check "初始 status idle" idle "$(j "d['data']['status']")"
check "purchase_date 为 YYYY-MM-DD" 2024-05-01 "$(j "d['data']['purchase_date']")"
check "创建即带 live（vehicle_status 行）" "$V1" "$(j "d['data']['live']['vehicle_id']")"
check "live.plate_no" "鲁A·T$SUF" "$(j "d['data']['live']['plate_no']")"
check "live.online 初始 False" False "$(j "d['data']['live']['online']")"
check "trip_count 0" 0 "$(j "d['data']['trip_count']")"
req POST /assets/vehicles "$CH" "{\"plate_no\":\"鲁A·T$SUF\"}"
check "车牌重复 409" 409 "$STATUS"
req POST /assets/vehicles "$CH" "{\"plate_no\":\"鲁A·U$SUF\",\"vin\":\"VIN$SUF\"}"
check "VIN 重复 409" 409 "$STATUS"
req POST /assets/vehicles "$CH" '{"plate_no":"X"}'
check "车牌过短 400" 400 "$STATUS"
req POST /assets/vehicles "$CH" "{\"plate_no\":\"鲁A·U$SUF\",\"home_dept_id\":\"$NIL\"}"
check "归属部门不存在 400" 400 "$STATUS"
req POST /assets/vehicles "$CH" "{\"plate_no\":\"鲁A·U$SUF\",\"purchase_date\":\"05/01/2024\"}"
check "日期格式错误 400" 400 "$STATUS"
req POST /assets/vehicles "$CH" "{\"plate_no\":\"鲁A·U$SUF\",\"seat_count\":7}"
check "创建第二辆车 201" 201 "$STATUS"; V2=$(j "d['data']['id']")
req GET "/assets/vehicles?keyword=T$SUF" "$CH"
check "列表 keyword 200" 200 "$STATUS"
check "keyword 命中 1" 1 "$(j "d['data']['total']")"
check "列表项带 live" "$V1" "$(j "d['data']['items'][0]['live']['vehicle_id']")"
req GET "/assets/vehicles?status=bogus" "$CH"
check "status 非法 400" 400 "$STATUS"
req GET "/assets/vehicles?sort=-plate_no&pageSize=5&keyword=$SUF" "$CH"
check "排序 200" 200 "$STATUS"
check "sort=-plate_no 降序（U 在 T 前）" "鲁A·U$SUF" "$(j "d['data']['items'][0]['plate_no']")"
req GET "/assets/vehicles/$V1" "$CH"
check "详情 200" 200 "$STATUS"
check "详情 device_id 为空" None "$(j "d['data']['device_id']")"
req GET "/assets/vehicles/$NIL" "$CH"
check "详情不存在 404" 404 "$STATUS"
req GET "/assets/vehicles/$V1" "$ADMIN"
check "跨租户：平台管理员未带 X-Tenant-ID 404" 404 "$STATUS"
req GET "/assets/vehicles/$V1" "$ADMIN" "" "X-Tenant-ID: $CH_TID"
check "平台管理员带 X-Tenant-ID 200" 200 "$STATUS"

section "车辆：状态切换 / 编辑 / options"
req PUT "/assets/vehicles/$V1" "$CH" '{"status":"in_use"}'
check "手动置 in_use 400" 400 "$STATUS"
req PUT "/assets/vehicles/$V1" "$CH" '{"status":"maintenance"}'
check "置维保 200" 200 "$STATUS"
check "status=maintenance" maintenance "$(j "d['data']['status']")"
check "live.status 同步 maintenance" maintenance "$(j "d['data']['live']['status']")"
req GET "/assets/vehicles/options?status=maintenance" "$CH"
check "options 200" 200 "$STATUS"
check "options 含该车" True "$(j "any(x['id']=='$V1' for x in d['data'])")"
req GET "/assets/vehicles/options" "$CH"
check "options 按车牌排序" True "$(j "[x['plate_no'] for x in d['data']]==sorted(x['plate_no'] for x in d['data'])")"
req PUT "/assets/vehicles/$V1" "$CH" '{"status":"idle","remark":"备注","clear_home_dept":true}'
check "恢复 idle 200" 200 "$STATUS"
check "status=idle" idle "$(j "d['data']['status']")"
check "remark 已写入" 备注 "$(j "d['data']['remark']")"
req PUT "/assets/vehicles/$V1" "$CH" "{\"plate_no\":\"鲁A·U$SUF\"}"
check "改车牌与他车重复 409" 409 "$STATUS"
req PUT "/assets/vehicles/$V1" "$CH" "{\"vin\":\"VINNEW$SUF\",\"color\":\"蓝\"}"
check "编辑 VIN/颜色 200" 200 "$STATUS"
check "vin 更新" "VINNEW$SUF" "$(j "d['data']['vin']")"
req PUT "/assets/vehicles/$V1" "$CH" '{"seat_count":0}'
check "seat_count 越界 400" 400 "$STATUS"

# ---------------------------------------------------------------- 设备
section "设备：创建 / 绑定 / 解绑"
req POST /assets/devices "$CH" "{\"serial_no\":\"VG-$SUF\",\"vehicle_id\":\"$V1\",\"firmware\":\"1.0.0\"}"
check "创建设备（同时绑定）201" 201 "$STATUS"; D1=$(j "d['data']['id']"); D1KEY=$(j "d['data']['api_key']")
check "api_key 以 vig_ 开头" True "$(j "d['data']['api_key'].startswith('vig_')")"
check "vehicle_plate 关联" "鲁A·T$SUF" "$(j "d['data']['vehicle_plate']")"
check "model 缺省 VIG-100E" VIG-100E "$(j "d['data']['model']")"
check "online 初始 False" False "$(j "d['data']['online']")"
req GET "/assets/devices/$D1" "$CH"
check "详情 200" 200 "$STATUS"
check "详情不含 api_key" False "$(j "'api_key' in d['data']")"
req POST /assets/devices "$CH" "{\"serial_no\":\"VG-$SUF\"}"
check "序列号重复 409" 409 "$STATUS"
req POST /assets/devices "$CH" '{"serial_no":"ab"}'
check "序列号不合规 400" 400 "$STATUS"
req POST /assets/devices "$CH" "{\"serial_no\":\"VG2-$SUF\",\"vehicle_id\":\"$V1\"}"
check "创建时绑定已有设备的车 409" 409 "$STATUS"
req POST /assets/devices "$CH" "{\"serial_no\":\"VG2-$SUF\",\"vehicle_id\":\"$NIL\"}"
check "创建时绑定不存在车辆 400" 400 "$STATUS"
req POST /assets/devices "$CH" "{\"serial_no\":\"VG2-$SUF\"}"
check "创建未绑定设备 201" 201 "$STATUS"; D2=$(j "d['data']['id']"); D2KEY=$(j "d['data']['api_key']")
req POST "/assets/devices/$D2/bind" "$CH" "{\"vehicle_id\":\"$V1\"}"
check "绑定已有设备的车 409" 409 "$STATUS"
req POST "/assets/devices/$D2/bind" "$CH" "{\"vehicle_id\":\"$V2\"}"
check "绑定 V2 200" 200 "$STATUS"
check "vehicle_id=V2" "$V2" "$(j "d['data']['vehicle_id']")"
req POST "/assets/devices/$D2/bind" "$CH" "{\"vehicle_id\":\"$V2\"}"
check "重复绑定同一车幂等 200" 200 "$STATUS"
req POST "/assets/devices/$D2/bind" "$CH" "{\"vehicle_id\":\"$V1\"}"
check "已绑他车的设备改绑 409（须先解绑）" 409 "$STATUS"
req POST "/assets/devices/$D2/bind" "$CH" "{\"vehicle_id\":\"$NIL\"}"
check "绑定不存在车辆 400" 400 "$STATUS"
req POST "/assets/devices/$D2/unbind" "$CH"
check "解绑 200" 200 "$STATUS"
check "解绑后 vehicle_id 为空" None "$(j "d['data']['vehicle_id']")"
req POST "/assets/devices/$D2/unbind" "$CH"
check "重复解绑 400" 400 "$STATUS"
req GET "/assets/devices?bound=true&keyword=VG-$SUF" "$CH"
check "列表 bound=true 命中 D1" "['$D1']" "$(j "[x['id'] for x in d['data']['items']]")"
req GET "/assets/devices?bound=false&keyword=VG2-$SUF" "$CH"
check "列表 bound=false 命中 D2" "['$D2']" "$(j "[x['id'] for x in d['data']['items']]")"
req GET "/assets/devices?keyword=T$SUF" "$CH"
check "列表 keyword 按车牌命中 D1" "['$D1']" "$(j "[x['id'] for x in d['data']['items']]")"
req GET "/assets/devices?status=bogus" "$CH"
check "status 非法 400" 400 "$STATUS"
req GET "/assets/vehicles/$V1" "$CH"
check "车辆详情 device_id" "$D1" "$(j "d['data']['device_id']")"
check "车辆详情 device_serial" "VG-$SUF" "$(j "d['data']['device_serial']")"

# ---------------------------------------------------------------- 网关接入
section "网关鉴权与遥测入库"
dreq "VG-$SUF" "wrong-key" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "错误 key 401" 401 "$STATUS"
dreq "" "" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "缺少设备头 401" 401 "$STATUS"
dreq "NOPE-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "未知序列号 401" 401 "$STATUS"
dreq "VG2-$SUF" "$D2KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "未绑定车辆 409" 409 "$STATUS"
check "409 message" "device not bound to a vehicle" "$(j "d['message']")"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry '{"points":[]}'
check "空 points 400" 400 "$STATUS"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry '{"points":[{"lng":117.1}]}'
check "缺 ts 400" 400 "$STATUS"
# 两点乱序上报：NOW 在前、T1（更早）在后，服务端应按 ts 升序处理并以 NOW 点更新实时状态
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\",\"lng\":117.13,\"lat\":36.66,\"speed\":42.5,\"heading\":90,\"soc\":80,\"odometer_km\":1200.5,\"acc_on\":true,\"locked\":false},{\"ts\":\"$T1\",\"lng\":117.12,\"lat\":36.65,\"speed\":30,\"soc\":81,\"odometer_km\":1199.5,\"acc_on\":true,\"locked\":false}]}"
check "上报 2 点 200" 200 "$STATUS"
check "accepted=2" 2 "$(j "d['data']['accepted']")"
check "trip_id 为空（无进行中行程）" None "$(j "d['data']['trip_id']")"
req GET "/assets/vehicles/$V1/status" "$CH"
check "单车实时状态 200" 200 "$STATUS"
check "online True" True "$(j "d['data']['online']")"
check "soc=80（取最后一点）" True "$(j "abs(float(d['data']['soc'])-80)<0.01")"
check "range_km=320（80%×400）" True "$(j "abs(float(d['data']['range_km'])-320)<0.01")"
check "lng=117.13" True "$(j "abs(float(d['data']['lng'])-117.13)<1e-6")"
check "speed=42.5" True "$(j "abs(float(d['data']['speed'])-42.5)<0.01")"
check "locked False" False "$(j "d['data']['locked']")"
check "status 仍 idle" idle "$(j "d['data']['status']")"
req GET "/assets/vehicles/$V1" "$CH"
check "档案 soc 同步 80" True "$(j "abs(float(d['data']['soc'])-80)<0.01")"
check "档案 odometer 取最大 1200.5" True "$(j "abs(float(d['data']['odometer_km'])-1200.5)<0.01")"
req GET "/assets/vehicles/$V1/telemetry?from=$FROM&to=$TO" "$CH"
check "历史遥测 200" 200 "$STATUS"
check "2 个点" 2 "$(j "len(d['data'])")"
check "按 ts 升序（首点 speed=30）" True "$(j "abs(float(d['data'][0]['speed'])-30)<0.01")"
check "点带 acc_on" True "$(j "d['data'][0]['acc_on']")"
req GET "/assets/vehicles/$V1/telemetry?from=$FROM&to=$TO&limit=1" "$CH"
check "limit=1" 1 "$(j "len(d['data'])")"
req GET "/assets/vehicles/$V1/telemetry?to=$TO" "$CH"
check "缺 from 400" 400 "$STATUS"
req GET "/assets/vehicles/$V1/telemetry?from=$TO&to=$FROM" "$CH"
check "from>to 400" 400 "$STATUS"
req GET "/assets/vehicles/$V1/telemetry?from=$OLD&to=$TO" "$CH"
check "区间超 7 天 400" 400 "$STATUS"
req GET "/assets/vehicles/$V1/telemetry?from=$FROM&to=$TO&limit=0" "$CH"
check "limit=0 400" 400 "$STATUS"
req GET "/assets/vehicles/$V2/telemetry?from=$FROM&to=$TO" "$CH"
check "无遥测车辆返回空数组" "[]" "$(j "d['data']")"
req GET "/assets/vehicles?online=true&keyword=$SUF" "$CH"
check "列表 online=true 只含 V1" "['$V1']" "$(j "[x['id'] for x in d['data']['items']]")"
req GET "/assets/vehicles?online=false&keyword=$SUF" "$CH"
check "列表 online=false 只含 V2" "['$V2']" "$(j "[x['id'] for x in d['data']['items']]")"
req GET "/assets/vehicles/status" "$CH"
check "全部实时状态 200" 200 "$STATUS"
check "含 V1 且 online" True "$(j "any(x['vehicle_id']=='$V1' and x['online'] for x in d['data'])")"
check "含 V2 且 offline" True "$(j "any(x['vehicle_id']=='$V2' and not x['online'] for x in d['data'])")"
req GET "/assets/devices/$D1" "$CH"
check "设备 online True" True "$(j "d['data']['online']")"
check "设备 last_ip 已记录" True "$(j "d['data']['last_ip'] is not None")"
req GET "/assets/devices?online=true&keyword=VG-$SUF" "$CH"
check "设备列表 online=true 命中" "['$D1']" "$(j "[x['id'] for x in d['data']['items']]")"

section "充电状态与手动状态的优先级"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\",\"charging\":true,\"soc\":82,\"speed\":0}]}"
check "上报 charging=true 200" 200 "$STATUS"
req GET "/assets/vehicles/$V1/status" "$CH"
check "实时 status=charging" charging "$(j "d['data']['status']")"
check "charging True" True "$(j "d['data']['charging']")"
req GET "/assets/vehicles/$V1" "$CH"
check "档案 status=charging" charging "$(j "d['data']['status']")"
req PUT "/assets/vehicles/$V1" "$CH" '{"status":"maintenance"}'
check "充电中可置维保 200" 200 "$STATUS"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\",\"charging\":false}]}"
req GET "/assets/vehicles/$V1/status" "$CH"
check "维保不被遥测覆盖" maintenance "$(j "d['data']['status']")"
req PUT "/assets/vehicles/$V1" "$CH" '{"status":"idle"}'
req GET "/assets/vehicles/$V1/status" "$CH"
check "恢复 idle" idle "$(j "d['data']['status']")"

section "设备密钥轮换 / 停用"
req POST "/assets/devices/$D1/rotate-key" "$CH"
check "rotate-key 200" 200 "$STATUS"; D1KEY2=$(j "d['data']['api_key']")
check "新 key 不同于旧 key" True "$([ "$D1KEY" != "$D1KEY2" ] && echo True || echo False)"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "旧 key 立即失效 401" 401 "$STATUS"
dreq "VG-$SUF" "$D1KEY2" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "新 key 可用 200" 200 "$STATUS"
D1KEY=$D1KEY2
req PUT "/assets/devices/$D1" "$CH" '{"status":"disabled","firmware":"1.0.1"}'
check "停用设备 200" 200 "$STATUS"
check "firmware 更新" 1.0.1 "$(j "d['data']['firmware']")"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "停用后上报 403" 403 "$STATUS"
req PUT "/assets/devices/$D1" "$CH" '{"status":"active"}'
check "恢复启用 200" 200 "$STATUS"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "恢复后上报 200" 200 "$STATUS"
req PUT "/assets/devices/$D1" "$CH" '{"status":"bogus"}'
check "status 非法 400" 400 "$STATUS"

section "网关事件（行程模块占位：hooks 的错误原样透传）"
# 行程模块未实现时其占位 hooks 返回 httpx.Internal(501 AppError)，经 httpx.Fail 呈现为 500；实现后应为 4xx/200。
# 这里只验证 ingest 层把 hooks 的错误原样交给 httpx.Fail（不吞、不改写为 400/409）。
dreq "VG-$SUF" "$D1KEY" POST /ingest/events "{\"type\":\"trip_start\",\"trip_start\":{\"ts\":\"$NOW\",\"card_uid\":\"abcdef12\"}}"
check "trip_start → hooks 错误透传（未知卡 403）" True "$([ "$STATUS" == 403 ] && echo True || echo "$STATUS")"
dreq "VG-$SUF" "$D1KEY" POST /ingest/events "{\"type\":\"trip_end\",\"trip_end\":{\"ts\":\"$NOW\"}}"
check "trip_end → hooks 错误透传（无进行中行程 409）" True "$([ "$STATUS" == 409 ] && echo True || echo "$STATUS")"
dreq "VG-$SUF" "$D1KEY" POST /ingest/events '{"type":"trip_start"}'
check "trip_start 缺 body 400" 400 "$STATUS"
dreq "VG-$SUF" "$D1KEY" POST /ingest/events "{\"type\":\"trip_start\",\"trip_start\":{\"ts\":\"$NOW\"}}"
check "trip_start 缺 card_uid/user_id 400" 400 "$STATUS"
dreq "VG-$SUF" "$D1KEY" POST /ingest/events '{"type":"bogus"}'
check "未知事件类型 400" 400 "$STATUS"
dreq "VG2-$SUF" "$D2KEY" POST /ingest/events "{\"type\":\"trip_end\",\"trip_end\":{\"ts\":\"$NOW\"}}"
check "未绑定设备事件 409" 409 "$STATUS"
dreq "VG-$SUF" "bad" POST /ingest/events "{\"type\":\"trip_end\",\"trip_end\":{\"ts\":\"$NOW\"}}"
check "事件错误 key 401" 401 "$STATUS"

# ---------------------------------------------------------------- NFC 卡
section "NFC 卡"
req POST /assets/nfc-cards "$CH" "{\"card_uid\":\"ab$SUF\"}"
check "发卡 201" 201 "$STATUS"; C1=$(j "d['data']['id']")
check "card_uid 统一大写" "AB$SUF" "$(j "d['data']['card_uid']")"
check "status active" active "$(j "d['data']['status']")"
check "issued_at 缺省为现在" True "$(j "d['data']['issued_at'] is not None")"
check "未绑定 user_id 为空" None "$(j "d['data']['user_id']")"
req POST /assets/nfc-cards "$CH" "{\"card_uid\":\"AB$SUF\"}"
check "卡号重复（大小写不敏感）409" 409 "$STATUS"
req POST /assets/nfc-cards "$CH" '{"card_uid":"xyz"}'
check "卡号不合规 400" 400 "$STATUS"
req POST /assets/nfc-cards "$CH" "{\"card_uid\":\"cd$SUF\",\"user_id\":\"$NIL\"}"
check "持卡人不存在 400" 400 "$STATUS"
req POST /assets/nfc-cards "$CH" "{\"card_uid\":\"cd$SUF\",\"user_id\":\"$CH_UID\",\"issued_at\":\"$NOW\"}"
check "发卡并绑定 201" 201 "$STATUS"; C2=$(j "d['data']['id']")
check "user_name 关联" True "$(j "d['data']['user_name'] is not None")"
check "username 关联" chenhua_admin "$(j "d['data']['username']")"
req GET "/assets/nfc-cards?keyword=chenhua_admin" "$CH"
check "列表 keyword=用户名 命中 C2" True "$(j "any(x['id']=='$C2' for x in d['data']['items'])")"
req GET "/assets/nfc-cards?bound=true&keyword=$SUF" "$CH"
check "bound=true 只含 C2" "['$C2']" "$(j "[x['id'] for x in d['data']['items']]")"
req GET "/assets/nfc-cards?bound=false&keyword=$SUF" "$CH"
check "bound=false 只含 C1" "['$C1']" "$(j "[x['id'] for x in d['data']['items']]")"
req GET "/assets/nfc-cards?status=bogus" "$CH"
check "status 非法 400" 400 "$STATUS"
req POST "/assets/nfc-cards/$C1/bind" "$CH" "{\"user_id\":\"$NIL\"}"
check "绑定不存在用户 400" 400 "$STATUS"
req POST "/assets/nfc-cards/$C1/bind" "$CH" "{\"user_id\":\"$CH_UID\"}"
check "绑定持卡人 200" 200 "$STATUS"
check "绑定后 username" chenhua_admin "$(j "d['data']['username']")"
req POST "/assets/nfc-cards/$C1/unbind" "$CH"
check "解绑 200" 200 "$STATUS"
check "解绑后 user_id 为空" None "$(j "d['data']['user_id']")"
req POST "/assets/nfc-cards/$C1/report-loss" "$CH"
check "挂失 200" 200 "$STATUS"
check "status=lost" lost "$(j "d['data']['status']")"
req GET "/assets/nfc-cards?status=lost&keyword=$SUF" "$CH"
check "列表 status=lost 命中" "['$C1']" "$(j "[x['id'] for x in d['data']['items']]")"
req PUT "/assets/nfc-cards/$C1" "$CH" '{"status":"active","remark":"补办"}'
check "恢复 active 200" 200 "$STATUS"
check "status=active" active "$(j "d['data']['status']")"
req PUT "/assets/nfc-cards/$C1" "$CH" '{"status":"bogus"}'
check "status 非法 400" 400 "$STATUS"
req DELETE "/assets/nfc-cards/$C2" "$CH"
check "删除卡 200" 200 "$STATUS"
req GET "/assets/nfc-cards/$C2" "$CH"
check "删除后 404" 404 "$STATUS"
req POST /assets/nfc-cards "$CH" "{\"card_uid\":\"CD$SUF\"}"
check "软删后卡号可重用 201" 201 "$STATUS"; C3=$(j "d['data']['id']")

# ---------------------------------------------------------------- 充电桩
section "充电桩"
req POST /assets/charge-piles "$CH" "{\"pile_code\":\"P-$SUF\",\"name\":\"桩A\",\"lng\":117.1,\"lat\":36.6}"
check "创建 201" 201 "$STATUS"; P1=$(j "d['data']['id']")
check "status 初始 offline" offline "$(j "d['data']['status']")"
check "type 缺省 slow" slow "$(j "d['data']['type']")"
check "power_kw 缺省 7" True "$(j "abs(float(d['data']['power_kw'])-7)<0.01")"
check "connector_count 缺省 1" 1 "$(j "d['data']['connector_count']")"
req POST /assets/charge-piles "$CH" "{\"pile_code\":\"P-$SUF\",\"name\":\"桩B\"}"
check "编号重复 409" 409 "$STATUS"
req POST /assets/charge-piles "$CH" '{"pile_code":"!","name":"桩B"}'
check "编号不合规 400" 400 "$STATUS"
req POST /assets/charge-piles "$CH" "{\"pile_code\":\"Q-$SUF\"}"
check "缺 name 400" 400 "$STATUS"
req POST /assets/charge-piles "$CH" "{\"pile_code\":\"Q-$SUF\",\"name\":\"桩B\",\"type\":\"medium\"}"
check "type 非法 400" 400 "$STATUS"
req PUT "/assets/charge-piles/$P1" "$CH" '{"status":"available"}'
check "手动置 available 400" 400 "$STATUS"
req PUT "/assets/charge-piles/$P1" "$CH" '{"status":"charging"}'
check "手动置 charging 400" 400 "$STATUS"
req PUT "/assets/charge-piles/$P1" "$CH" '{"status":"disabled"}'
check "手动置 disabled 200" 200 "$STATUS"
check "status=disabled" disabled "$(j "d['data']['status']")"
req PUT "/assets/charge-piles/$P1" "$CH" '{"status":"offline","type":"fast","power_kw":60,"location":"地下车库"}'
check "恢复 offline 并编辑 200" 200 "$STATUS"
check "status=offline" offline "$(j "d['data']['status']")"
check "type=fast" fast "$(j "d['data']['type']")"
req GET "/assets/charge-piles?keyword=P-$SUF" "$CH"
check "列表 keyword 命中 1" 1 "$(j "d['data']['total']")"
req GET "/assets/charge-piles?keyword=车库&type=fast" "$CH"
check "列表 keyword=位置 & type 命中" True "$(j "any(x['id']=='$P1' for x in d['data']['items'])")"
req GET "/assets/charge-piles?status=bogus" "$CH"
check "status 非法 400" 400 "$STATUS"
req DELETE "/assets/charge-piles/$P1" "$CH"
check "删除 200" 200 "$STATUS"
req GET "/assets/charge-piles/$P1" "$CH"
check "删除后 404" 404 "$STATUS"

# ---------------------------------------------------------------- 删除车辆
section "删除车辆（软删 + 自动解绑设备）"
req DELETE "/assets/vehicles/$V1" "$CH"
check "删除 V1 200" 200 "$STATUS"
req GET "/assets/vehicles/$V1" "$CH"
check "删除后 404" 404 "$STATUS"
req GET "/assets/devices/$D1" "$CH"
check "设备 D1 自动解绑" None "$(j "d['data']['vehicle_id']")"
req GET "/assets/vehicles/status" "$CH"
check "实时状态列表不再含 V1" False "$(j "any(x['vehicle_id']=='$V1' for x in d['data'])")"
dreq "VG-$SUF" "$D1KEY" POST /ingest/telemetry "{\"points\":[{\"ts\":\"$NOW\"}]}"
check "解绑后网关上报 409" 409 "$STATUS"
req POST /assets/vehicles "$CH" "{\"plate_no\":\"鲁A·T$SUF\"}"
check "软删后车牌可重用 201" 201 "$STATUS"; V3=$(j "d['data']['id']")
req DELETE "/assets/vehicles/$V3" "$CH";  check "删除 V3 200" 200 "$STATUS"
req DELETE "/assets/vehicles/$V2" "$CH";  check "删除 V2 200" 200 "$STATUS"
req DELETE "/assets/devices/$D1" "$CH";   check "删除 D1 200" 200 "$STATUS"
req GET "/assets/devices/$D1" "$CH";      check "D1 删除后 404" 404 "$STATUS"
req POST /assets/devices "$CH" "{\"serial_no\":\"VG-$SUF\"}"
check "软删后序列号可重用 201" 201 "$STATUS"; D3=$(j "d['data']['id']")
req DELETE "/assets/devices/$D3" "$CH";   check "删除 D3 200" 200 "$STATUS"
req DELETE "/assets/devices/$D2" "$CH";   check "删除 D2 200" 200 "$STATUS"
req DELETE "/assets/nfc-cards/$C1" "$CH"; check "删除 C1 200" 200 "$STATUS"
req DELETE "/assets/nfc-cards/$C3" "$CH"; check "删除 C3 200" 200 "$STATUS"

echo
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
