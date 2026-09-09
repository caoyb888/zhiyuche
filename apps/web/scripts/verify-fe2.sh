#!/usr/bin/env bash
# FE-2 联调：用 curl 按页面逐个跑 Web 管理端各系统页面用到的接口（含超管带 X-Tenant-ID），
# 覆盖：角色 / 权限树 / 字典 / 参数 / 通知模板 / 审计日志 / 租户 / 用户导入导出。
#
# 前置：已 `make api-build`（bin/zhiyuche-api）且 apps/api/.env 指向本脚本专用的库与端口；
#       admin / chenhua_admin 密码 Admin@123456；机器上有 curl 与 python3。
# 用法：PORT=20092 bash apps/web/scripts/verify-fe2.sh   （在仓库根目录执行）
# 脚本自行启动 api（pid 写 /tmp/zy-fe2-api.pid），结束时按 pid 停止。
set -u
ROOT=$(pwd)
PORT=${PORT:-20092}
BASE=http://localhost:$PORT/api/v1
PIDF=${PIDF:-/tmp/zy-fe2-api.pid}
LOG=${LOG:-/tmp/zy-fe2-api.log}
SFX=$(date +%s | tail -c 6)$RANDOM
TMP=${TMPDIR:-/tmp}/zy-fe2-$$
mkdir -p "$TMP"

stop_api() {
  if [ -f "$PIDF" ]; then
    kill "$(cat "$PIDF")" 2>/dev/null && echo "api stopped (pid $(cat "$PIDF"))"
    rm -f "$PIDF"
  fi
}
trap 'stop_api; rm -rf "$TMP"' EXIT

# ---------------------------------------------------------------- start api
# 注意用 `;` 而不是 `&&`：`cd && cmd &` 会把整个列表放到子 shell 后台，$! 就成了子 shell 的 pid
( cd "$ROOT/apps/api"; nohup ../../bin/zhiyuche-api > "$LOG" 2>&1 & echo $! > "$PIDF" )
echo "api started pid $(cat "$PIDF")"
for i in $(seq 1 30); do
  if curl -s -o /dev/null "$BASE/health"; then break; fi
  sleep 0.5
done
curl -s "$BASE/health"; echo

PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "  ok   $1"; }
bad() { FAIL=$((FAIL+1)); echo "  FAIL $1"; echo "       body: ${BODY:0:240}"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want [$2], got [$3])"; fi; }
section() { echo; echo "== $1"; }

# req METHOD PATH TOKEN [BODY] [EXTRA_HEADER]
req() {
  local m=$1 p=$2 t=$3 b=${4:-} h=${5:-}
  local args=(-s -o "$TMP/body" -w '%{http_code}' -X "$m" -H "Authorization: Bearer $t" -H 'Content-Type: application/json' -H 'Accept: application/json')
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
    if k: d = d[int(k)] if isinstance(d, list) else d.get(k)
print(len(d))' "$1"; }
jfind() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d.get(k)
for it in d:
    if str(it.get(sys.argv[2])) == sys.argv[3]:
        v = it.get(sys.argv[4]); print("null" if v is None else (str(v).lower() if isinstance(v, bool) else v)); sys.exit(0)
print("__missing__")' "$1" "$2" "$3" "$4"; }
jall() { printf '%s' "$BODY" | python3 -c '
import sys, json
d = json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k: d = d.get(k)
print("true" if d and all(str(it.get(sys.argv[2])) == sys.argv[3] for it in d) else "false")' "$1" "$2" "$3"; }

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
xlsx_rows() { python3 - "$1" <<'PY'
import sys, zipfile, re
z = zipfile.ZipFile(sys.argv[1])
sheet = [n for n in z.namelist() if n.startswith("xl/worksheets/sheet")][0]
print(len(re.findall(r"<row[ >]", z.read(sheet).decode())) - 1)
PY
}
upload() {
  STATUS=$(curl -s -o "$TMP/body" -w '%{http_code}' -H "Authorization: Bearer $2" -F "file=@$3" "$BASE$1")
  BODY=$(cat "$TMP/body")
}

# ---------------------------------------------------------------- login
section "登录（Login 页）"
req POST /auth/login "" '{"username":"admin","password":"Admin@123456"}'
SUPER=$(j data.access_token); check "超级管理员登录" 200 "$STATUS"
req POST /auth/login "" '{"username":"chenhua_admin","password":"Admin@123456","tenant_code":"chenhua"}'
TENANT=$(j data.access_token); check "租户管理员登录" 200 "$STATUS"
req GET /auth/me "$SUPER";  PLATFORM_TID=$(j data.tenant.id); check "超管 /auth/me is_super" true "$(j data.is_super)"
req GET /auth/me "$TENANT"; CH_TID=$(j data.tenant.id); CH_UID=$(j data.id)
HDR="X-Tenant-ID: $CH_TID"
echo "  platform=$PLATFORM_TID chenhua=$CH_TID"
check "超管菜单含租户管理（/system/tenants）" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json' >/dev/null; req GET /auth/me "$SUPER"; printf '%s' "$BODY" | python3 -c '
import sys,json
d=json.load(sys.stdin)["data"]["menus"]
paths=[]
def walk(ns):
    for n in ns:
        paths.append(n["path"]); walk(n.get("children") or [])
walk(d); print(str("/system/tenants" in paths and "/system/audit-logs" in paths).lower())')"
req GET /auth/me "$TENANT"
check "租户菜单不含租户管理" true "$(printf '%s' "$BODY" | python3 -c '
import sys,json
d=json.load(sys.stdin)["data"]
perms=d["permissions"]; print(str("system:tenant:view" not in perms and "system:role:view" in perms).lower())')"
check "路由 path 与菜单 path 一致" true "$(printf '%s' "$BODY" | python3 -c '
import sys,json
d=json.load(sys.stdin)["data"]["menus"]
paths=set()
def walk(ns):
    for n in ns:
        paths.add(n["path"]); walk(n.get("children") or [])
walk(d)
want={"/system/roles","/system/permissions","/system/dicts","/system/params","/system/templates","/system/audit-logs"}
print(str(want <= paths).lower())')"

# ---------------------------------------------------------------- roles
section "角色管理页 /system/roles"
req GET "/system/roles?page=1&pageSize=20&sort=code" "$TENANT"
check "列表 → 200" 200 "$STATUS"; check "  内置角色存在 tenant_admin" true "$(jfind data.items code tenant_admin is_system)"
SYS_ROLE=$(jfind data.items code tenant_admin id); EMP_ROLE=$(jfind data.items code employee id)
req GET "/system/roles?keyword=fleet" "$TENANT"; check "keyword 过滤" fleet_manager "$(j data.items.0.code)"
req GET "/system/roles?sort=-created_at" "$TENANT"; check "sort=-created_at → 200" 200 "$STATUS"
req GET /system/permissions/tree "$TENANT"
check "权限树 → 200（分配权限 Modal）" 200 "$STATUS"; check "  根为 menu" menu "$(j data.0.type)"
check "  非平台租户不含 system.tenant" __missing__ "$(printf '%s' "$BODY" | python3 -c '
import sys,json
d=json.load(sys.stdin)["data"]
sysm=[n for n in d if n["code"]=="system"][0]
print(next((c["code"] for c in sysm["children"] if c["code"]=="system.tenant"), "__missing__"))')"
RCODE="fe_role_$SFX"
req POST /system/roles "$TENANT" "{\"code\":\"$RCODE\",\"name\":\"前端测试角色\",\"description\":\"desc\",\"permissions\":[\"dashboard:view\",\"trip:view\"]}"
check "新建角色（含初始权限）→ 201" 201 "$STATUS"; ROLE=$(j data.id); check "  permissions=2" 2 "$(jlen data.permissions)"
req POST /system/roles "$TENANT" '{"code":"Bad Code","name":"x"}'; check "code 不合法 → 400" 400 "$STATUS"
req POST /system/roles "$TENANT" "{\"code\":\"$RCODE\",\"name\":\"dup\"}"; check "code 重复 → 409" 409 "$STATUS"
req PUT "/system/roles/$ROLE" "$TENANT" '{"name":"改名","description":"新描述"}'; check "编辑名称/描述 → 200" 200 "$STATUS"; check "  name" 改名 "$(j data.name)"
req PUT "/system/roles/$ROLE/permissions" "$TENANT" '{"permissions":["dashboard:view","system:user:view","system:user:create"]}'
check "分配权限 → 200" 200 "$STATUS"; check "  permissions=3" 3 "$(jlen data.permissions)"
req PUT "/system/roles/$ROLE/permissions" "$TENANT" '{"permissions":["system:tenant:view"]}'; check "非平台租户含平台权限 → 400" 400 "$STATUS"
req PUT "/system/roles/$ROLE/permissions" "$TENANT" '{"permissions":["no:such:perm"]}'; check "未知权限码 → 400" 400 "$STATUS"
req PUT "/system/roles/$ROLE/permissions" "$TENANT" '{"permissions":[]}'; check "清空权限 → 200" 200 "$STATUS"
req DELETE "/system/roles/$SYS_ROLE" "$TENANT"; check "删除内置角色 → 409（Toast 显示原因）" 409 "$STATUS"; echo "       message: $(j message)"
req POST /system/users "$TENANT" "{\"username\":\"fe_u_$SFX\",\"name\":\"持有者\",\"role_ids\":[\"$ROLE\"]}"; U_HOLDER=$(j data.id)
req DELETE "/system/roles/$ROLE" "$TENANT"; check "删除仍有用户的角色 → 409" 409 "$STATUS"; echo "       message: $(j message)"
req DELETE "/system/users/$U_HOLDER" "$TENANT"
req DELETE "/system/roles/$ROLE" "$TENANT"; check "删除自定义角色 → 200" 200 "$STATUS"
req GET "/system/roles/$ROLE" "$TENANT"; check "删除后详情 → 404" 404 "$STATUS"
req GET "/system/roles?pageSize=200" "$SUPER" "" "$HDR"; check "超管带 X-Tenant-ID 看租户角色" true "$(jall data.items tenant_id "$CH_TID")"
req GET "/system/roles?pageSize=200" "$SUPER"; check "超管无头含平台级角色 super_admin" null "$(jfind data.items code super_admin tenant_id)"
req GET /system/permissions/tree "$SUPER"
check "超管权限树含 system.tenant" system.tenant "$(printf '%s' "$BODY" | python3 -c '
import sys,json
d=json.load(sys.stdin)["data"]
sysm=[n for n in d if n["code"]=="system"][0]
print(next((c["code"] for c in sysm["children"] if c["code"]=="system.tenant"), "__missing__"))')"

# ---------------------------------------------------------------- dicts
section "字典管理页 /system/dicts"
DCODE="fe_dict_$SFX"
req GET "/system/dict-types?page=1&pageSize=20&sort=code" "$TENANT"; check "类型列表 → 200" 200 "$STATUS"
req GET "/system/dict-types?keyword=zzz_none_$SFX" "$TENANT"; check "空态 total=0" 0 "$(j data.total)"
req POST /system/dict-types "$TENANT" "{\"code\":\"$DCODE\",\"name\":\"前端字典\",\"description\":\"d\",\"items\":[{\"label\":\"甲\",\"value\":\"a\",\"sort\":0,\"status\":\"active\"},{\"label\":\"乙\",\"value\":\"b\",\"sort\":1,\"status\":\"active\"}]}"
check "新建类型（含初始条目）→ 201" 201 "$STATUS"; DT=$(j data.id); check "  is_global=false" false "$(j data.is_global)"
req GET "/system/dict-types/$DT" "$TENANT"; check "详情含条目 2" 2 "$(jlen data.items)"
req POST "/system/dict-types/$DT/items" "$TENANT" '{"label":"丙","value":"c","sort":2,"color":"#10b981","status":"disabled","extra":{"k":1}}'
check "新增条目 → 201" 201 "$STATUS"; DI=$(j data.id); check "  color" "#10b981" "$(j data.color)"; check "  extra" '{"k": 1}' "$(j data.extra)"
req POST "/system/dict-types/$DT/items" "$TENANT" '{"label":"x","value":"c","status":"active"}'; check "条目值重复 → 409" 409 "$STATUS"
req PUT "/system/dict-items/$DI" "$TENANT" '{"label":"丙2","status":"active","extra":{}}'; check "编辑条目 → 200" 200 "$STATUS"; check "  label" 丙2 "$(j data.label)"
req PUT "/system/dict-types/$DT" "$TENANT" '{"name":"前端字典2"}'; check "编辑类型名称 → 200" 200 "$STATUS"
req GET "/system/dict-types?pageSize=200" "$SUPER"
G_SYS=$(printf '%s' "$BODY" | python3 -c '
import sys,json
d=json.load(sys.stdin)["data"]["items"]
print(next((i["id"] for i in d if i["is_system"] and i["is_global"]), ""))')
if [ -n "$G_SYS" ]; then
  req DELETE "/system/dict-types/$G_SYS" "$SUPER"; check "删除系统字典 → 409" 409 "$STATUS"; echo "       message: $(j message)"
  req PUT "/system/dict-types/$G_SYS" "$TENANT" '{"name":"x"}'; check "租户改全局字典 → 403（按钮已隐藏）" 403 "$STATUS"
else
  echo "  (skip) 库中没有全局系统字典"
fi
req DELETE "/system/dict-items/$DI" "$TENANT"; check "删除条目 → 200" 200 "$STATUS"
req GET "/system/dict-types/$DT" "$SUPER" "" "$HDR"; check "超管带头看租户字典 → 200" 200 "$STATUS"
req DELETE "/system/dict-types/$DT" "$TENANT"; check "删除类型 → 200" 200 "$STATUS"

# ---------------------------------------------------------------- params
section "参数设置页 /system/params"
KEY=security.login_max_failures
req GET /system/params "$TENANT"; check "列表 → 200" 200 "$STATUS"; N=$(jlen data); check "  ≥4 项" true "$([ "$N" -ge 4 ] && echo true)"
VT=$(jfind data key $KEY value_type); check "  $KEY 类型 int" int "$VT"
req PUT "/system/params/$KEY" "$TENANT" '{"value":"abc"}'; check "int 给非整数 → 400" 400 "$STATUS"; echo "       message: $(j message)"
req PUT "/system/params/$KEY" "$TENANT" '{"value":"7","description":"前端设置"}'; check "租户 PUT → 200 source=tenant" tenant "$(j data.source)"
req DELETE "/system/params/$KEY" "$TENANT"; check "恢复缺省 DELETE → 200" 200 "$STATUS"
req GET /system/params "$TENANT"; check "  恢复后 source=global" global "$(jfind data key $KEY source)"
req PUT "/system/params/$KEY" "$SUPER" '{"value":"5"}'; check "超管无头 PUT → 写全局 source=global" global "$(j data.source)"
req DELETE "/system/params/$KEY" "$SUPER"; check "超管无头 DELETE 全局 → 400（按钮已隐藏）" 400 "$STATUS"
req PUT "/system/params/$KEY" "$SUPER" '{"value":"6"}' "$HDR"; check "超管带头 PUT → 租户覆盖" tenant "$(j data.source)"
req DELETE "/system/params/$KEY" "$SUPER" "" "$HDR"; check "超管带头 DELETE → 200" 200 "$STATUS"
# json/bool 类型若存在，验证校验
JKEY=$(req GET /system/params "$TENANT"; printf '%s' "$BODY" | python3 -c 'import sys,json; d=json.load(sys.stdin)["data"]; print(next((p["key"] for p in d if p["value_type"]=="bool"), ""))')
if [ -n "$JKEY" ]; then
  req PUT "/system/params/$JKEY" "$TENANT" '{"value":"yes"}'; check "bool 给 yes → 400" 400 "$STATUS"
  req PUT "/system/params/$JKEY" "$TENANT" '{"value":"true"}'; check "bool 给 true → 200" 200 "$STATUS"
  req DELETE "/system/params/$JKEY" "$TENANT"
fi

# ---------------------------------------------------------------- templates
section "通知模板页 /system/templates"
TCODE="fe.tpl_$SFX"
req GET "/system/notification-templates?page=1&pageSize=20" "$TENANT"; check "列表 → 200" 200 "$STATUS"
req GET "/system/notification-templates?channel=sms" "$TENANT"; check "channel 过滤 → 200" 200 "$STATUS"
req GET "/system/notification-templates?channel=fax" "$TENANT"; check "channel 非法 → 400" 400 "$STATUS"
req POST /system/notification-templates "$SUPER" "{\"code\":\"$TCODE\",\"channel\":\"inapp\",\"title\":\"全局\",\"content\":\"你好 {{name}}\",\"enabled\":true}"
check "超管创建全局模板 → 201" 201 "$STATUS"; GT=$(j data.id); check "  is_global" true "$(j data.is_global)"
req PUT "/system/notification-templates/$GT" "$TENANT" '{"enabled":false}'; check "租户切换全局模板开关 → 403（Switch 已禁用）" 403 "$STATUS"
req POST /system/notification-templates "$TENANT" "{\"code\":\"$TCODE\",\"channel\":\"inapp\",\"title\":\"租户\",\"content\":\"c {{x}}\",\"enabled\":true}"
check "租户创建同 code/channel 覆盖 → 201" 201 "$STATUS"; TT=$(j data.id)
req POST /system/notification-templates "$TENANT" "{\"code\":\"$TCODE\",\"channel\":\"inapp\",\"title\":\"dup\",\"content\":\"c\",\"enabled\":true}"; check "重复 → 409" 409 "$STATUS"
req PUT "/system/notification-templates/$TT" "$TENANT" '{"enabled":false}'; check "启用 Switch PUT → 200 enabled=false" false "$(j data.enabled)"
req PUT "/system/notification-templates/$TT" "$TENANT" '{"title":"改","content":"新 {{a}} {{b}}"}'; check "编辑标题/内容 → 200" 200 "$STATUS"
req GET "/system/notification-templates?keyword=$TCODE" "$TENANT"; check "租户列表：inapp 为租户行" "$TT" "$(jfind data.items channel inapp id)"
req GET "/system/notification-templates?keyword=$TCODE" "$SUPER" "" "$HDR"; check "超管带头列表同租户视角" "$TT" "$(jfind data.items channel inapp id)"
req DELETE "/system/notification-templates/$TT" "$TENANT"; check "删除租户模板 → 200" 200 "$STATUS"
req DELETE "/system/notification-templates/$GT" "$SUPER"; check "删除全局模板 → 200" 200 "$STATUS"

# ---------------------------------------------------------------- users import/export
section "用户页导入 / 导出 /system/users"
curl -s -D "$TMP/tpl.h" -o "$TMP/tpl.xlsx" -H "Authorization: Bearer $TENANT" -H 'Accept: application/json' "$BASE/system/users/import-template"
check "下载模板 xlsx Content-Type" true "$(grep -qi 'content-type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' "$TMP/tpl.h" && echo true)"
check "  Content-Disposition 文件名（前端 download() 取用）" true "$(grep -qi 'filename=' "$TMP/tpl.h" && echo true)"
U1="fe_imp_$SFX"
make_xlsx "$TMP/import.xlsx" "[[\"用户名*\",\"姓名*\",\"手机号\",\"邮箱\",\"工号\",\"部门名称\",\"角色代码\"],[\"$U1\",\"导入甲\",\"\",\"\",\"\",\"\",\"employee\"],[\"\",\"缺用户名\",\"\",\"\",\"\",\"\",\"\"],[\"$U1\",\"重复\",\"\",\"\",\"\",\"\",\"\"]]"
upload /system/users/import "$TENANT" "$TMP/import.xlsx"
check "导入 → 200" 200 "$STATUS"; check "  total=3 success=1 failed=2（ImportModal 明细表）" "3 1 2" "$(j data.total) $(j data.success) $(j data.failed)"
echo "       errors: $(j data.errors)"
printf 'nope' > "$TMP/bad.xlsx"; upload /system/users/import "$TENANT" "$TMP/bad.xlsx"; check "非法文件 → 400（Toast）" 400 "$STATUS"
curl -s -D "$TMP/exp.h" -o "$TMP/export.xlsx" -H "Authorization: Bearer $TENANT" -H 'Accept: application/json' "$BASE/system/users/export?keyword=$U1&status=active"
check "导出 xlsx Content-Type" true "$(grep -qi 'content-type: application/vnd.openxmlformats' "$TMP/exp.h" && echo true)"
check "  attachment 文件名" true "$(grep -qi 'content-disposition: attachment; filename=' "$TMP/exp.h" && echo true)"
check "  带筛选导出 1 行" 1 "$(xlsx_rows "$TMP/export.xlsx")"
req GET "/system/users?keyword=$U1" "$TENANT"; U1_ID=$(j data.items.0.id); req DELETE "/system/users/$U1_ID" "$TENANT"
# 前端 blob 错误体：无权限时 JSON 信封
curl -s -o "$TMP/err.json" -w '%{http_code}' -H "Authorization: Bearer invalid" "$BASE/system/users/export" > "$TMP/code"
check "导出 401 时返回 JSON 信封（client.readErrorBody 可解析）" true "$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(str("code" in d and "message" in d).lower())' "$TMP/err.json")"

# ---------------------------------------------------------------- tenants
section "租户管理页 /system/tenants（超管）"
req GET "/system/tenants?page=1&pageSize=20" "$TENANT"; check "租户管理员访问 → 403（路由 403 页）" 403 "$STATUS"
req GET "/system/tenants?pageSize=200&sort=code" "$SUPER"; check "顶栏下拉 pageSize=200 → 200" 200 "$STATUS"
check "  含平台租户 is_platform" true "$(jfind data.items id "$PLATFORM_TID" is_platform)"
req GET "/system/tenants?keyword=chenhua&status=active" "$SUPER"; check "keyword+status 过滤" chenhua "$(j data.items.0.code)"
req GET "/system/tenants?status=bogus" "$SUPER"; check "status 非法 → 400" 400 "$STATUS"
TCODE2="fe-t-$SFX"
req POST /system/tenants "$SUPER" "{\"code\":\"$TCODE2\",\"name\":\"前端租户\",\"contact_name\":\"张三\",\"contact_phone\":\"13800000000\",\"expires_at\":\"2030-01-01T00:00:00.000Z\",\"admin_username\":\"fe_t_admin_$SFX\",\"admin_name\":\"管理员\"}"
check "新建租户（expires_at 为 toISOString 格式）→ 201" 201 "$STATUS"; NT=$(j data.id); check "  user_count=1（管理员）" 1 "$(j data.user_count)"
req POST /system/tenants "$SUPER" "{\"code\":\"$TCODE2\",\"name\":\"dup\",\"admin_username\":\"x1\",\"admin_name\":\"管理员\"}"; check "code 重复 → 409" 409 "$STATUS"
req POST /system/tenants "$SUPER" '{"code":"Bad_Code","name":"x","admin_username":"x1","admin_name":"管理员"}'; check "code 不合法 → 400" 400 "$STATUS"
req PUT "/system/tenants/$NT" "$SUPER" '{"name":"前端租户2","clear_expires":true}'; check "编辑 + clear_expires → 200" 200 "$STATUS"; check "  expires_at=null" null "$(j data.expires_at)"
req PUT "/system/tenants/$NT" "$SUPER" '{"status":"disabled"}'; check "停用（PUT status）→ 200" disabled "$(j data.status)"
req PUT "/system/tenants/$NT" "$SUPER" '{"status":"active"}'; check "启用 → 200" active "$(j data.status)"
req PUT "/system/tenants/$PLATFORM_TID" "$SUPER" '{"status":"disabled"}'; check "平台租户停用 → 400（按钮已禁用）" 400 "$STATUS"; echo "       message: $(j message)"
req GET "/system/tenants/$NT" "$SUPER"; check "详情 → 200" 200 "$STATUS"
req GET "/system/tenants?pageSize=200" "$SUPER" "" "$HDR"; check "租户接口带 X-Tenant-ID 仍返回全部（前端 skipTenantHeader）" 200 "$STATUS"
req PUT "/system/tenants/$NT" "$SUPER" '{"status":"disabled"}'

# ---------------------------------------------------------------- super admin viewing tenant
section "超管切换查看租户（X-Tenant-ID）"
req GET "/system/users?pageSize=5" "$SUPER" "" "$HDR"; check "用户列表 → chenhua 用户" true "$(jall data.items tenant_id "$CH_TID")"
req GET /system/depts/tree "$SUPER" "" "$HDR"; check "部门树 → 200" 200 "$STATUS"
req GET "/system/dict-types?pageSize=5" "$SUPER" "" "$HDR"; check "字典列表 → 200" 200 "$STATUS"
req GET /system/params "$SUPER" "" "$HDR"; check "参数列表 → 200" 200 "$STATUS"
req GET "/system/notification-templates?pageSize=5" "$SUPER" "" "$HDR"; check "模板列表 → 200" 200 "$STATUS"
req GET "/system/audit-logs?pageSize=5" "$SUPER" "" "$HDR"; check "审计日志只看目标租户" true "$(jall data.items tenant_id "$CH_TID")"
req GET /auth/me "$SUPER" "" "$HDR"; check "/auth/me 不受头影响（前端不发）" "$PLATFORM_TID" "$(j data.tenant.id)"

# ---------------------------------------------------------------- audit logs
section "审计日志页 /system/audit-logs"
sleep 1
FROM=$(date -u -d '-10 minutes' +%Y-%m-%dT%H:%M:%S.000Z); TO=$(date -u -d '+1 minute' +%Y-%m-%dT%H:%M:%S.000Z)
req GET "/system/audit-logs?page=1&pageSize=20" "$TENANT"; check "列表 → 200" 200 "$STATUS"; check "  列表项 before 为 null" null "$(j data.items.0.before)"
LOG_ID=$(j data.items.0.id)
req GET "/system/audit-logs?module=system.role&action=create&keyword=$RCODE" "$TENANT"; check "module+action+keyword 过滤" 1 "$(j data.total)"
req GET "/system/audit-logs?username=chenhua&target_id=$KEY&action=update" "$TENANT"; check "username+target_id+action" true "$([ "$(j data.total)" -ge 1 ] && echo true)"
req GET "/system/audit-logs?from=$FROM&to=$TO" "$TENANT"; check "from/to（toISOString 毫秒格式）→ 200 有数据" true "$([ "$STATUS" = 200 ] && [ "$(j data.total)" -ge 1 ] && echo true)"
req GET "/system/audit-logs?from=$TO&to=$FROM" "$TENANT"; check "to 早于 from → 400" 400 "$STATUS"
req GET "/system/audit-logs?sort=-created_at" "$TENANT"; check "sort=-created_at → 200" 200 "$STATUS"
req GET "/system/audit-logs?sort=module" "$TENANT"; check "sort=module → 200" 200 "$STATUS"
req GET "/system/audit-logs/$LOG_ID" "$TENANT"; check "详情 → 200" 200 "$STATUS"
req GET "/system/audit-logs?module=system.role&action=assign_perms" "$TENANT"; AP=$(j data.items.0.id)
req GET "/system/audit-logs/$AP" "$TENANT"; check "assign_perms 详情 before/after 为数组（JsonView 并排）" true "$(printf '%s' "$BODY" | python3 -c 'import sys,json; d=json.load(sys.stdin)["data"]; print(str(isinstance(d["before"], list) and isinstance(d["after"], list)).lower())')"
req GET "/system/audit-logs?module=system.tenant" "$TENANT"; check "租户看不到平台租户日志" 0 "$(j data.total)"
req GET "/system/audit-logs?module=system.tenant&keyword=$TCODE2" "$SUPER"; check "超管看到租户模块日志 ≥4" true "$([ "$(j data.total)" -ge 4 ] && echo true)"

echo
echo "== 结果：通过 $PASS，失败 $FAIL"
[ "$FAIL" -eq 0 ]
