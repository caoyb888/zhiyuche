#!/usr/bin/env bash
# 端到端验证：字典 / 系统参数 / 审计日志 / 通知模板 / 用户 Excel 导入导出。
#
# 前置：api 已启动并完成引导（admin / chenhua_admin，密码 Admin@123456）。
# 用法：BASE=http://localhost:20091/api/v1 bash apps/api/scripts/verify-be2.sh
# 可选：PSQL="docker exec -i zhiyuche-postgres psql -U zhiyuche -d zhiyuche_be2 -tAq"
#       有 PSQL 时额外覆盖"系统字典不可删除"与"按部门名称导入"两个用例。
# 需要 curl 与 python3（生成/解析 xlsx 只用标准库 zipfile）。
set -u
BASE=${BASE:-http://localhost:20091/api/v1}
PSQL=${PSQL:-}
SFX=$(date +%s | tail -c 6)$RANDOM
TMP=${TMPDIR:-/tmp}/zy-be2-$$
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "  ok   $1"; }
bad() { FAIL=$((FAIL+1)); echo "  FAIL $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want [$2], got [$3])"; fi; }
section() { echo; echo "== $1"; }

# req METHOD PATH TOKEN [BODY] [EXTRA_HEADER] → 设置 STATUS / BODY
req() {
  local m=$1 p=$2 t=$3 b=${4:-} h=${5:-}
  local args=(-s -o "$TMP/body" -w '%{http_code}' -X "$m" -H "Authorization: Bearer $t" -H 'Content-Type: application/json')
  [ -n "$h" ] && args+=(-H "$h")
  [ -n "$b" ] && args+=(--data "$b")
  STATUS=$(curl "${args[@]}" "$BASE$p")
  BODY=$(cat "$TMP/body")
}
# j PATH  — 从 $BODY 取 JSON 路径值（点分，数组下标用数字）；bool 输出 true/false，null 输出 null
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
# jlen PATH — 数组长度
jlen() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d[int(k)] if isinstance(d, list) else d.get(k)
print(len(d))' "$1"; }
# jfind ARRAYPATH FIELD VALUE OUTFIELD — 在数组里找 FIELD==VALUE 的元素，输出 OUTFIELD
jfind() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d.get(k)
for it in d:
    if str(it.get(sys.argv[2])) == sys.argv[3]:
        v = it.get(sys.argv[4]); print("null" if v is None else (str(v).lower() if isinstance(v, bool) else v)); sys.exit(0)
print("__missing__")' "$1" "$2" "$3" "$4"; }
# jall ARRAYPATH FIELD VALUE — 数组中每个元素的 FIELD 是否都等于 VALUE
jall() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d.get(k)
print("true" if d and all(str(it.get(sys.argv[2])) == sys.argv[3] for it in d) else "false")' "$1" "$2" "$3"; }

# make_xlsx OUT JSON_ROWS — 用 zipfile 生成最简 xlsx（inlineStr 单元格）
make_xlsx() { python3 - "$1" "$2" <<'PY'
import sys, json, zipfile
out, rows = sys.argv[1], json.loads(sys.argv[2])
def esc(s): return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
def col(i):
    s = ""; i += 1
    while i: i, r = divmod(i - 1, 26); s = chr(65 + r) + s
    return s
sheet = '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>'
for ri, row in enumerate(rows):
    sheet += '<row r="%d">' % (ri + 1)
    for ci, v in enumerate(row):
        if v is None or v == "": continue
        sheet += '<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>' % (col(ci), ri + 1, esc(str(v)))
    sheet += "</row>"
sheet += "</sheetData></worksheet>"
z = zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED)
z.writestr("[Content_Types].xml", '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>')
z.writestr("_rels/.rels", '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>')
z.writestr("xl/workbook.xml", '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="用户" sheetId="1" r:id="rId1"/></sheets></workbook>')
z.writestr("xl/_rels/workbook.xml.rels", '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>')
z.writestr("xl/worksheets/sheet1.xml", sheet)
z.close()
PY
}
# xlsx_rows FILE — 校验 zip/xlsx 结构并输出第一张表的数据行数（不含表头）
xlsx_rows() { python3 - "$1" <<'PY'
import sys, zipfile, re
z = zipfile.ZipFile(sys.argv[1])
names = z.namelist()
assert "xl/workbook.xml" in names, names
sheet = [n for n in names if n.startswith("xl/worksheets/sheet")][0]
xml = z.read(sheet).decode()
print(len(re.findall(r"<row[ >]", xml)) - 1)
PY
}
# upload PATH TOKEN FILE [FIELD] — multipart 上传
upload() {
  local field=${4:-file}
  STATUS=$(curl -s -o "$TMP/body" -w '%{http_code}' -H "Authorization: Bearer $2" -F "$field=@$3" "$BASE$1")
  BODY=$(cat "$TMP/body")
}
sql() { [ -n "$PSQL" ] && printf '%s' "$1" | $PSQL | head -n1; }

# ------------------------------------------------------------------ login
section "登录"
req POST /auth/login "" '{"username":"admin","password":"Admin@123456"}'
SUPER=$(j data.access_token); check "超级管理员登录" 200 "$STATUS"
req POST /auth/login "" '{"username":"chenhua_admin","password":"Admin@123456","tenant_code":"chenhua"}'
TENANT=$(j data.access_token); check "租户管理员登录" 200 "$STATUS"
req GET /auth/me "$SUPER";  PLATFORM_TID=$(j data.tenant.id)
req GET /auth/me "$TENANT"; CH_TID=$(j data.tenant.id); CH_UID=$(j data.id)
HDR="X-Tenant-ID: $CH_TID"
echo "  platform tenant=$PLATFORM_TID  chenhua tenant=$CH_TID"

# ------------------------------------------------------------------ dict
section "字典 dict"
CODE="vt_$SFX"
req POST /system/dict-types "$SUPER" "{\"code\":\"$CODE\",\"name\":\"车辆类型\",\"description\":\"全局\",\"items\":[{\"label\":\"轿车\",\"value\":\"sedan\",\"sort\":2},{\"label\":\"SUV\",\"value\":\"suv\",\"sort\":1,\"color\":\"#0f0\",\"extra\":{\"seats\":5}}]}"
check "超管无 X-Tenant-ID 创建 → 201 全局字典" 201 "$STATUS"
G_DICT=$(j data.id)
check "  is_global=true" true "$(j data.is_global)"
check "  tenant_id=null" null "$(j data.tenant_id)"
check "  is_system=false（接口创建一律 false）" false "$(j data.is_system)"
check "  条目按 sort 排序" suv "$(j data.items.0.value)"
check "  extra 原样返回" '{"seats": 5}' "$(j data.items.0.extra)"
check "  未给 extra 时为 {}" '{}' "$(j data.items.1.extra)"
req POST /system/dict-types "$SUPER" '{"code":"Bad-Code","name":"x"}';           check "code 不合法 → 400" 400 "$STATUS"
req POST /system/dict-types "$SUPER" "{\"code\":\"$CODE\",\"name\":\"dup\"}";       check "全局 code 重复 → 409" 409 "$STATUS"
req POST /system/dict-types "$SUPER" '{"code":"dupv","name":"x","items":[{"label":"a","value":"v"},{"label":"b","value":"v"}]}'
check "请求内条目值重复 → 400" 400 "$STATUS"
req POST /system/dict-types "$SUPER" '{"name":"x"}';                              check "缺 code → 400" 400 "$STATUS"

req GET "/system/dict-types?keyword=$CODE" "$TENANT"
check "租户列表可见全局字典" 200 "$STATUS"
check "  total=1" 1 "$(j data.total)"
check "  列表项 items 为空数组" '[]' "$(j data.items.0.items)"
check "  列表项 is_global=true" true "$(j data.items.0.is_global)"
req GET "/system/dict-types/$G_DICT" "$TENANT"
check "租户查看全局详情 → 200 含条目" 200 "$STATUS"; check "  条目数=2" 2 "$(jlen data.items)"
req PUT "/system/dict-types/$G_DICT" "$TENANT" '{"name":"改名"}';                 check "租户改全局字典 → 403" 403 "$STATUS"
req DELETE "/system/dict-types/$G_DICT" "$TENANT";                                check "租户删全局字典 → 403" 403 "$STATUS"
req POST "/system/dict-types/$G_DICT/items" "$TENANT" '{"label":"x","value":"x"}'; check "租户向全局字典加条目 → 403" 403 "$STATUS"
req PUT "/system/dict-types/$G_DICT" "$SUPER" '{"name":"车辆类型2","description":"改过"}'
check "超管改全局字典 → 200" 200 "$STATUS"; check "  name 已更新" "车辆类型2" "$(j data.name)"

req POST /system/dict-types "$TENANT" "{\"code\":\"$CODE\",\"name\":\"租户车辆类型\",\"items\":[{\"label\":\"货车\",\"value\":\"truck\"}]}"
check "租户创建同 code 字典 → 201（租户覆盖）" 201 "$STATUS"
T_DICT=$(j data.id)
check "  is_global=false" false "$(j data.is_global)"; check "  tenant_id=租户" "$CH_TID" "$(j data.tenant_id)"
req POST /system/dict-types "$TENANT" "{\"code\":\"$CODE\",\"name\":\"dup\"}";     check "租户内 code 重复 → 409" 409 "$STATUS"
req GET "/system/dict-types?keyword=$CODE" "$TENANT"
check "租户列表：同 code 只剩租户行" 1 "$(j data.total)"; check "  即租户字典" "$T_DICT" "$(j data.items.0.id)"
req GET "/system/dict-types?keyword=$CODE" "$SUPER"
check "超管无头列表：只看到全局行" "$G_DICT" "$(j data.items.0.id)"
req GET "/system/dict-types?keyword=$CODE" "$SUPER" "" "$HDR"
check "超管带 X-Tenant-ID 列表：看到租户行" "$T_DICT" "$(j data.items.0.id)"

req GET "/system/dicts/$CODE" "$TENANT";  check "GET /dicts/{code} 租户 → 租户条目优先" truck "$(j data.0.value)"
req GET "/system/dicts/$CODE" "$SUPER";   check "GET /dicts/{code} 超管无头 → 全局条目" suv "$(j data.0.value)"
req GET "/system/dicts/$CODE" "$SUPER" "" "$HDR"; check "GET /dicts/{code} 超管带头 → 租户条目" truck "$(j data.0.value)"
req GET "/system/dicts/no_such_$SFX" "$TENANT"; check "GET /dicts/不存在 → 404" 404 "$STATUS"

req POST "/system/dict-types/$T_DICT/items" "$TENANT" '{"label":"客车","value":"bus","sort":0,"status":"disabled"}'
check "租户加条目 → 201" 201 "$STATUS"; ITEM=$(j data.id); check "  dict_type_id 正确" "$T_DICT" "$(j data.dict_type_id)"
req POST "/system/dict-types/$T_DICT/items" "$TENANT" '{"label":"x","value":"bus"}'; check "同字典条目值重复 → 409" 409 "$STATUS"
req GET "/system/dicts/$CODE" "$TENANT"; check "禁用条目不出现在 /dicts/{code}" 1 "$(jlen data)"
req PUT "/system/dict-items/$ITEM" "$TENANT" '{"status":"active","sort":-1,"extra":{"k":"v"}}'
check "改条目 → 200" 200 "$STATUS"; check "  status=active" active "$(j data.status)"
req GET "/system/dicts/$CODE" "$TENANT"; check "启用后按 sort 排在前" bus "$(j data.0.value)"
req PUT "/system/dict-items/$ITEM" "$TENANT" '{"value":"truck"}'; check "改条目值与已有重复 → 409" 409 "$STATUS"
req PUT "/system/dict-items/$ITEM" "$SUPER" '{"label":"x"}';       check "超管无头改租户条目 → 404（不可见）" 404 "$STATUS"
req PUT "/system/dict-items/$ITEM" "$SUPER" '{"label":"超管改的"}' "$HDR"; check "超管带头改租户条目 → 200" 200 "$STATUS"
req DELETE "/system/dict-items/$ITEM" "$TENANT"; check "删条目 → 200" 200 "$STATUS"
req DELETE "/system/dict-items/$ITEM" "$TENANT"; check "再删 → 404" 404 "$STATUS"
req GET "/system/dict-types/$T_DICT" "$SUPER";   check "超管无头看租户字典 → 404" 404 "$STATUS"
req GET "/system/dict-types/$T_DICT" "$SUPER" "" "$HDR"; check "超管带头看租户字典 → 200" 200 "$STATUS"
req PUT "/system/dict-types/$T_DICT" "$SUPER" '{"name":"超管代改"}' "$HDR"; check "超管带头改租户字典 → 200" 200 "$STATUS"
req GET "/system/dict-types/not-a-uuid" "$TENANT"; check "id 非 uuid → 400" 400 "$STATUS"
req GET "/system/dict-types/00000000-0000-0000-0000-000000000001" "$TENANT"; check "不存在 → 404" 404 "$STATUS"
if [ -n "$PSQL" ]; then
  sql "UPDATE dict_types SET is_system = true WHERE id = '$G_DICT';" >/dev/null
  req DELETE "/system/dict-types/$G_DICT" "$SUPER"; check "系统字典删除 → 409" 409 "$STATUS"
  req PUT "/system/dict-types/$G_DICT" "$SUPER" '{"description":"系统字典可改描述"}'; check "系统字典改描述 → 200" 200 "$STATUS"
  sql "UPDATE dict_types SET is_system = false WHERE id = '$G_DICT';" >/dev/null
fi
req DELETE "/system/dict-types/$T_DICT" "$TENANT"; check "租户删自己的字典 → 200" 200 "$STATUS"
req GET "/system/dicts/$CODE" "$TENANT"; check "删除后 /dicts/{code} 回退到全局" suv "$(j data.0.value)"
req DELETE "/system/dict-types/$G_DICT" "$SUPER";  check "超管删全局字典 → 200（级联条目）" 200 "$STATUS"
req GET "/system/dict-types/$G_DICT" "$SUPER";     check "删除后 → 404" 404 "$STATUS"

# ------------------------------------------------------------------ param
section "系统参数 param"
KEY=security.login_max_failures
req GET /system/params "$TENANT"
check "租户参数列表 → 200" 200 "$STATUS"; check "  ≥4 个全局键" true "$([ "$(jlen data)" -ge 4 ] && echo true)"
check "  初始 source=global" global "$(jfind data key $KEY source)"
req GET "/system/params?keyword=security" "$TENANT"; check "keyword 过滤" 2 "$(jlen data)"
req PUT "/system/params/$KEY" "$TENANT" '{"value":"abc"}';     check "int 参数给非整数 → 400" 400 "$STATUS"
req PUT "/system/params/$KEY" "$TENANT" '{}';                  check "缺 value → 400" 400 "$STATUS"
req PUT "/system/params/no.such.key" "$TENANT" '{"value":"1"}'; check "未知 key → 404（不允许新增）" 404 "$STATUS"
req PUT "/system/params/$KEY" "$TENANT" '{"value":"7","description":"租户自定义"}'
check "租户 PUT → 200 写租户覆盖" 200 "$STATUS"
check "  source=tenant" tenant "$(j data.source)"; check "  value=7" 7 "$(j data.value)"; check "  global_value=5" 5 "$(j data.global_value)"
check "  description 覆盖" "租户自定义" "$(j data.description)"
req PUT "/system/params/$KEY" "$TENANT" '{"value":"9"}'; check "租户再次 PUT（upsert）" 9 "$(j data.value)"
req GET /system/params "$SUPER"; check "超管无头列表：该键仍为 global" global "$(jfind data key $KEY source)"
req PUT "/system/params/$KEY" "$SUPER" '{"value":"6"}'
check "超管无头 PUT → 写全局缺省" 200 "$STATUS"; check "  source=global" global "$(j data.source)"; check "  value=6" 6 "$(j data.value)"
req GET /system/params "$TENANT"
check "租户仍看到自己的覆盖 9" 9 "$(jfind data key $KEY value)"; check "  global_value 变为 6" 6 "$(jfind data key $KEY global_value)"
req PUT "/system/params/$KEY" "$SUPER" '{"value":"8"}' "$HDR"
check "超管带头 PUT → 写目标租户覆盖" tenant "$(j data.source)"
req GET /system/params "$TENANT"; check "租户看到 8" 8 "$(jfind data key $KEY value)"
req DELETE "/system/params/$KEY" "$SUPER"; check "超管无头 DELETE 全局 → 400" 400 "$STATUS"
req DELETE "/system/params/$KEY" "$TENANT"; check "租户 DELETE 覆盖 → 200" 200 "$STATUS"
req GET /system/params "$TENANT"
check "恢复全局值 6" 6 "$(jfind data key $KEY value)"; check "  source=global" global "$(jfind data key $KEY source)"
req DELETE "/system/params/$KEY" "$TENANT"; check "无覆盖再 DELETE → 404" 404 "$STATUS"
req DELETE "/system/params/no.such.key" "$TENANT"; check "未知 key DELETE → 404" 404 "$STATUS"
req PUT "/system/params/$KEY" "$SUPER" '{"value":"5"}'; check "恢复全局缺省 5" 5 "$(j data.value)"

# ------------------------------------------------------------------ template
section "通知模板 template"
TCODE="evt.test_$SFX"
req POST /system/notification-templates "$SUPER" "{\"code\":\"$TCODE\",\"channel\":\"inapp\",\"title\":\"全局标题\",\"content\":\"你好 {{name}}\"}"
check "超管无头创建 → 201 全局模板" 201 "$STATUS"; G_TPL=$(j data.id)
check "  is_global=true" true "$(j data.is_global)"; check "  enabled 默认 true" true "$(j data.enabled)"
req POST /system/notification-templates "$SUPER" '{"code":"Bad.Code","channel":"inapp","title":"t","content":"c"}'; check "code 不合法 → 400" 400 "$STATUS"
req POST /system/notification-templates "$SUPER" "{\"code\":\"$TCODE\",\"channel\":\"email\",\"title\":\"t\",\"content\":\"c\"}"; check "channel 不合法 → 400" 400 "$STATUS"
req POST /system/notification-templates "$SUPER" "{\"code\":\"$TCODE\",\"channel\":\"inapp\",\"title\":\"t\",\"content\":\"c\"}"; check "(全局,code,channel) 重复 → 409" 409 "$STATUS"
req GET "/system/notification-templates?keyword=$TCODE" "$TENANT"
check "租户列表可见全局模板" 1 "$(j data.total)"; check "  is_global=true" true "$(j data.items.0.is_global)"
req GET "/system/notification-templates?keyword=$TCODE&channel=sms" "$TENANT"; check "channel 过滤" 0 "$(j data.total)"
req GET "/system/notification-templates?channel=fax" "$TENANT"; check "channel 非法 → 400" 400 "$STATUS"
req GET "/system/notification-templates/$G_TPL" "$TENANT"; check "租户看全局详情 → 200" 200 "$STATUS"
req PUT "/system/notification-templates/$G_TPL" "$TENANT" '{"title":"x"}'; check "租户改全局 → 403" 403 "$STATUS"
req DELETE "/system/notification-templates/$G_TPL" "$TENANT";              check "租户删全局 → 403" 403 "$STATUS"
req POST /system/notification-templates "$TENANT" "{\"code\":\"$TCODE\",\"channel\":\"inapp\",\"title\":\"租户标题\",\"content\":\"租户内容\",\"enabled\":false}"
check "租户创建同 code/channel → 201 覆盖" 201 "$STATUS"; T_TPL=$(j data.id)
check "  tenant_id=租户" "$CH_TID" "$(j data.tenant_id)"; check "  enabled=false" false "$(j data.enabled)"
req POST /system/notification-templates "$TENANT" "{\"code\":\"$TCODE\",\"channel\":\"sms\",\"title\":\"短信\",\"content\":\"c\"}"
check "同 code 不同 channel → 201" 201 "$STATUS"; T_TPL_SMS=$(j data.id)
req GET "/system/notification-templates?keyword=$TCODE" "$TENANT"
check "租户列表：inapp 只剩租户行 + sms 行 = 2" 2 "$(j data.total)"
check "  inapp 为租户行" "$T_TPL" "$(jfind data.items channel inapp id)"
req GET "/system/notification-templates?keyword=$TCODE" "$SUPER"; check "超管无头列表只见全局行" "$G_TPL" "$(j data.items.0.id)"
req PUT "/system/notification-templates/$T_TPL" "$TENANT" '{"title":"改过","enabled":true}'
check "租户改自己的 → 200" 200 "$STATUS"; check "  title" 改过 "$(j data.title)"; check "  enabled=true" true "$(j data.enabled)"
req GET "/system/notification-templates/$T_TPL" "$SUPER";          check "超管无头看租户模板 → 404" 404 "$STATUS"
req GET "/system/notification-templates/$T_TPL" "$SUPER" "" "$HDR"; check "超管带头看租户模板 → 200" 200 "$STATUS"
req PUT "/system/notification-templates/$G_TPL" "$SUPER" '{"content":"全局改"}'; check "超管改全局 → 200" 200 "$STATUS"
req DELETE "/system/notification-templates/$T_TPL" "$TENANT";     check "租户删自己的 → 200" 200 "$STATUS"
req DELETE "/system/notification-templates/$T_TPL_SMS" "$TENANT"; check "租户删 sms → 200" 200 "$STATUS"
req DELETE "/system/notification-templates/$G_TPL" "$SUPER";      check "超管删全局 → 200" 200 "$STATUS"
req GET "/system/notification-templates/$G_TPL" "$SUPER";         check "删除后 → 404" 404 "$STATUS"

# ------------------------------------------------------------------ users import/export
section "用户导入导出"
curl -s -D "$TMP/tpl.h" -o "$TMP/tpl.xlsx" -w '' -H "Authorization: Bearer $TENANT" "$BASE/system/users/import-template"
check "下载模板 Content-Type" true "$(grep -qi 'content-type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' "$TMP/tpl.h" && echo true)"
check "  Content-Disposition 文件名" true "$(grep -qi 'filename="user-import-template.xlsx"' "$TMP/tpl.h" && echo true)"
check "  模板为合法 xlsx，含 1 行示例" 1 "$(xlsx_rows "$TMP/tpl.xlsx")"

DEPT_NAME="导入部_$SFX"
DEPT_ID=$(sql "INSERT INTO departments (tenant_id, name, path) VALUES ('$CH_TID', '$DEPT_NAME', '/') RETURNING id;")
[ -n "$DEPT_ID" ] && sql "UPDATE departments SET path = '/$DEPT_ID/' WHERE id = '$DEPT_ID';" >/dev/null
U1="imp_a_$SFX"; U2="imp_b_$SFX"; U3="imp_c_$SFX"
ROWS="[[\"用户名*\",\"姓名*\",\"手机号\",\"邮箱\",\"工号\",\"部门名称\",\"角色代码\"],
 [\"$U1\",\"导入甲\",\"1390000$RANDOM\",\"a_$SFX@example.com\",\"E$SFX\",\"\",\"employee,approver\"],
 [\"$U2\",\"导入乙\",\"\",\"\",\"\",\"$DEPT_NAME\",\"\"],
 [\"\",\"缺用户名\",\"\",\"\",\"\",\"\",\"\"],
 [\"$U3\",\"\",\"\",\"\",\"\",\"\",\"\"],
 [\"$U3\",\"坏邮箱\",\"\",\"not-an-email\",\"\",\"\",\"\"],
 [\"$U3\",\"坏角色\",\"\",\"\",\"\",\"\",\"no_such_role\"],
 [\"$U3\",\"坏部门\",\"\",\"\",\"\",\"没有的部门\",\"\"],
 [\"$U1\",\"文件内重复\",\"\",\"\",\"\",\"\",\"\"],
 [\"chenhua_admin\",\"库里已存在\",\"\",\"\",\"\",\"\",\"\"]]"
make_xlsx "$TMP/import.xlsx" "$ROWS"
upload /system/users/import "$TENANT" "$TMP/import.xlsx"
check "导入 → 200" 200 "$STATUS"
if [ -n "$DEPT_ID" ]; then
  check "  total=9 success=2 failed=7" "9 2 7" "$(j data.total) $(j data.success) $(j data.failed)"
  check "  失败行号 4,5,6,7,8,9,10" "4,5,6,7,8,9,10" "$(printf '%s' "$BODY" | python3 -c 'import sys,json; print(",".join(str(e["row"]) for e in json.load(sys.stdin)["data"]["errors"]))')"
else
  check "  total=9 success=1 failed=8（无 PSQL：部门行也失败）" "9 1 8" "$(j data.total) $(j data.success) $(j data.failed)"
fi
check "  第 8 行：被拒绝行的用户名不占用，按自身问题报部门不存在" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json; es=json.load(sys.stdin)["data"]["errors"]; print(str(any(e["row"]==8 and "部门不存在" in e["message"] for e in es)).lower())')"
check "  第 9 行：文件内用户名重复" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json; es=json.load(sys.stdin)["data"]["errors"]; print(str(any(e["row"]==9 and "重复" in e["message"] for e in es)).lower())')"
check "  第 10 行：用户名已存在" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json; es=json.load(sys.stdin)["data"]["errors"]; print(str(any(e["row"]==10 and "已存在" in e["message"] for e in es)).lower())')"
echo "  errors: $(j data.errors)"
req GET "/system/users?keyword=$U1" "$TENANT"
check "导入的用户可查到" 1 "$(j data.total)"; check "  角色已分配 2 个" 2 "$(jlen data.items.0.roles)"
U1_ID=$(j data.items.0.id)
req GET "/system/users?keyword=$U2" "$TENANT"; U2_ID=$(j data.items.0.id)
[ -n "$DEPT_ID" ] && check "  按部门名称挂到部门" "$DEPT_NAME" "$(j data.items.0.dept_name)"
upload /system/users/import "$TENANT" "$TMP/import.xlsx"
check "重复导入：全部因用户名已存在失败" "9 0 9" "$(j data.total) $(j data.success) $(j data.failed)"
printf 'not an xlsx' > "$TMP/bad.txt"
upload /system/users/import "$TENANT" "$TMP/bad.txt"; check "非 xlsx → 400" 400 "$STATUS"
upload /system/users/import "$TENANT" "$TMP/import.xlsx" other; check "缺 file 字段 → 400" 400 "$STATUS"
make_xlsx "$TMP/hdr.xlsx" '[["name","user"],["a","b"]]'
upload /system/users/import "$TENANT" "$TMP/hdr.xlsx"; check "表头不符 → 400" 400 "$STATUS"
make_xlsx "$TMP/empty.xlsx" '[["用户名*","姓名*"]]'
upload /system/users/import "$TENANT" "$TMP/empty.xlsx"; check "只有表头 → 200 total=0" "200 0" "$STATUS $(j data.total)"
python3 - "$TMP/big.xlsx" <<'PY'
import sys, zipfile
z = zipfile.ZipFile(sys.argv[1], "w", zipfile.ZIP_STORED); z.writestr("blob", b"0" * (6 * 1024 * 1024)); z.close()
PY
upload /system/users/import "$TENANT" "$TMP/big.xlsx"; check "超过 5MB → 400" 400 "$STATUS"
upload /system/users/import "$SUPER" "$TMP/import.xlsx"
check "超管无头导入到平台租户：用户名不冲突则成功" true "$([ "$(j data.success)" -ge 1 ] && echo true)"
req GET "/system/users?keyword=imp_" "$SUPER"; P_IDS=$(printf '%s' "$BODY" | python3 -c 'import sys,json; print(" ".join(i["id"] for i in json.load(sys.stdin)["data"]["items"]))')

curl -s -D "$TMP/exp.h" -o "$TMP/export.xlsx" -H "Authorization: Bearer $TENANT" "$BASE/system/users/export"
check "导出 → xlsx Content-Type" true "$(grep -qi 'content-type: application/vnd.openxmlformats' "$TMP/exp.h" && echo true)"
check "  Content-Disposition attachment" true "$(grep -qi 'content-disposition: attachment; filename="users-' "$TMP/exp.h" && echo true)"
check "  文件非空" true "$([ -s "$TMP/export.xlsx" ] && echo true)"
EXP_ROWS=$(xlsx_rows "$TMP/export.xlsx")
req GET "/system/users?pageSize=1" "$TENANT"
check "  导出行数 = 租户用户总数" "$(j data.total)" "$EXP_ROWS"
curl -s -o "$TMP/export2.xlsx" -H "Authorization: Bearer $TENANT" "$BASE/system/users/export?keyword=imp_&status=active"
IMPORTED=1; [ -n "$DEPT_ID" ] && IMPORTED=2
check "  带筛选导出行数=导入成功数" "$IMPORTED" "$(xlsx_rows "$TMP/export2.xlsx")"
curl -s -o "$TMP/export3.xlsx" "$BASE/system/users/export?keyword=$U1&access_token=$TENANT"
check "  access_token 查询参数可下载" 1 "$(xlsx_rows "$TMP/export3.xlsx")"
# 把导出文件原样回灌：excelize 能打开并逐行解析，行数一致；全部因用户名已存在失败
upload /system/users/import "$TENANT" "$TMP/export.xlsx"
check "导出文件可被 excelize 打开并解析（回灌 total=行数）" "200 $EXP_ROWS" "$STATUS $(j data.total)"
check "  回灌全部失败（用户已存在/角色列为名称）" "$EXP_ROWS" "$(j data.failed)"
curl -s -o /dev/null -w '%{http_code}' "$BASE/system/users/export" > "$TMP/code"; check "未登录导出 → 401" 401 "$(cat "$TMP/code")"

# cleanup users
for id in $U1_ID $U2_ID; do req DELETE "/system/users/$id" "$TENANT"; done
for id in $P_IDS; do req DELETE "/system/users/$id" "$SUPER"; done
[ -n "$DEPT_ID" ] && sql "UPDATE users SET dept_id = NULL WHERE dept_id = '$DEPT_ID'; DELETE FROM departments WHERE id = '$DEPT_ID';" >/dev/null

# ------------------------------------------------------------------ audit logs
section "审计日志 auditlog"
sleep 1  # 审计写入是异步的
req GET "/system/audit-logs?module=system.dict&action=create&keyword=新建字典%20$CODE" "$TENANT"
check "租户查询字典创建日志 → 200" 200 "$STATUS"
check "  只见本租户 1 条（超管建的全局字典不在本租户）" 1 "$(j data.total)"
check "  列表项 before/after 为 null" "null null" "$(j data.items.0.before) $(j data.items.0.after)"
check "  tenant_id=本租户" "$CH_TID" "$(j data.items.0.tenant_id)"
check "  method/path/status 齐全" "POST /api/v1/system/dict-types 201" "$(j data.items.0.method) $(j data.items.0.path) $(j data.items.0.status)"
LOG_ID=$(j data.items.0.id)
req GET "/system/audit-logs/$LOG_ID" "$TENANT"
check "详情 → 200 含 after" 200 "$STATUS"; check "  after.code" "$CODE" "$(j data.after.code)"
req GET "/system/audit-logs?module=system.dict&keyword=$CODE" "$SUPER"
check "超管无头：看到全部租户（≥3 条：全局创建/改/删 + 租户的）" true "$([ "$(j data.total)" -ge 5 ] && echo true)"
check "  默认按 id 降序" true "$([ "$(j data.items.0.id)" -gt "$(j data.items.1.id)" ] && echo true)"
req GET "/system/audit-logs?module=system.dict&keyword=$CODE" "$SUPER" "" "$HDR"
check "超管带头：只看目标租户" true "$(jall data.items tenant_id "$CH_TID")"
req GET "/system/audit-logs?module=system.dict&keyword=新建字典%20$CODE&action=create" "$SUPER"
check "超管无头：两个租户各 1 条创建" 2 "$(j data.total)"
P_LOG=$(jfind data.items tenant_id "$PLATFORM_TID" id)
req GET "/system/audit-logs/$P_LOG" "$TENANT"; check "租户看平台租户的日志 → 404" 404 "$STATUS"
req GET "/system/audit-logs/$P_LOG" "$SUPER";  check "超管看 → 200 且 after 非空" "200 $CODE" "$STATUS $(j data.after.code)"
req GET "/system/audit-logs?user_id=$CH_UID&module=system.param" "$TENANT"
check "user_id + module 过滤" true "$(jall data.items user_id "$CH_UID")"
check "  ≥3 条（PUT 7 / PUT 9 / DELETE）" true "$([ "$(j data.total)" -ge 3 ] && echo true)"
req GET "/system/audit-logs?username=chenhua&target_id=$KEY&action=delete" "$TENANT"
check "username 模糊 + target_id + action" true "$([ "$(j data.total)" -ge 1 ] && echo true)"
req GET "/system/audit-logs?module=system.user&action=import" "$TENANT"
check "导入审计 summary" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json; d=json.load(sys.stdin)["data"]["items"]; print(str(any("成功 2" in (i["summary"] or "") or "成功 1" in (i["summary"] or "") for i in d)).lower())')"
req GET "/system/audit-logs?from=$(date -u -d '-10 minutes' +%Y-%m-%dT%H:%M:%SZ)&to=$(date -u -d '+1 minute' +%Y-%m-%dT%H:%M:%SZ)" "$TENANT"
check "from/to 区间 → 有数据" true "$([ "$(j data.total)" -ge 1 ] && echo true)"
req GET "/system/audit-logs?to=2000-01-01T00:00:00Z" "$TENANT"; check "to 早于所有记录 → 0" 0 "$(j data.total)"
req GET "/system/audit-logs?from=yesterday" "$TENANT";  check "from 非法 → 400" 400 "$STATUS"
req GET "/system/audit-logs?user_id=abc" "$TENANT";     check "user_id 非 uuid → 400" 400 "$STATUS"
req GET "/system/audit-logs/abc" "$TENANT";             check "id 非整数 → 400" 400 "$STATUS"
req GET "/system/audit-logs/999999999" "$TENANT";       check "不存在 → 404" 404 "$STATUS"
req GET "/system/audit-logs?pageSize=2&page=2" "$TENANT"; check "分页 pageSize=2" "2 2" "$(j data.page) $(j data.pageSize)"

echo
echo "== 结果：通过 $PASS，失败 $FAIL"
[ "$FAIL" -eq 0 ]
