#!/usr/bin/env bash
# 端到端验证：部门 / 角色与权限 / 租户 三个模块（对照 openapi.yaml 的 depts、roles、tenants）。
# 前提：API 已启动并完成引导（admin/Admin@123456、chenhua_admin/Admin@123456）；机器上有 curl 与 python3。
# 用法：BASE=http://localhost:20090/api/v1 bash apps/api/scripts/verify-be1.sh
set -u
BASE=${BASE:-http://localhost:20090/api/v1}
ADMIN_PW=${ADMIN_PW:-Admin@123456}
SUF=$(date +%s)              # 每次运行使用不同的代码/名称，可重复执行
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
# j PY_EXPR —— 用 python 从 $BODY 取值，d 为解析后的 JSON
j() { python3 -c 'import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$1" <<<"$BODY" 2>/dev/null; }
check() { # check NAME WANT GOT
  if [ "$2" == "$3" ]; then PASS=$((PASS+1)); echo "  PASS  $1"
  else FAIL=$((FAIL+1)); echo "  FAIL  $1: want [$2] got [$3]"; echo "        body: ${BODY:0:300}"; fi
}
section() { echo; echo "== $1"; }
TOKEN=""; RT=""
login() { # login USER PW [TENANT_CODE] -> sets TOKEN / RT（不能放进 $(...)，否则 STATUS/BODY 丢失）
  local tc=${3:-}
  req POST /auth/login "" "{\"username\":\"$1\",\"password\":\"$2\",\"tenant_code\":\"$tc\"}"
  TOKEN=$(j "d['data']['access_token']"); RT=$(j "d['data']['refresh_token']")
}

# ---------------------------------------------------------------- 登录
section "登录"
login admin "$ADMIN_PW" platform;        ADMIN=$TOKEN; check "超级管理员登录" 200 "$STATUS"
PLATFORM_TID=$(j "d['data']['user']['tenant']['id']")
login chenhua_admin "$ADMIN_PW" chenhua; CH=$TOKEN;    check "租户管理员登录" 200 "$STATUS"
CH_TID=$(j "d['data']['user']['tenant']['id']")
echo "  platform=$PLATFORM_TID chenhua=$CH_TID"

# ---------------------------------------------------------------- 部门
section "部门：创建 / 树 / 列表 / 详情"
req POST /system/depts "$CH" "{\"name\":\"总部$SUF\",\"sort\":1,\"monthly_budget\":1000.5}"
check "创建根部门 201" 201 "$STATUS"; HQ=$(j "d['data']['id']"); HQ_PATH=$(j "d['data']['path']")
check "根部门 path=/id/" "/$HQ/" "$HQ_PATH"
check "根部门 parent_id null" None "$(j "d['data']['parent_id']")"
check "monthly_budget 数值" 1000.5 "$(j "d['data']['monthly_budget']")"
req POST /system/depts "$CH" "{\"name\":\"研发部\",\"parent_id\":\"$HQ\",\"sort\":2,\"code\":\"RD\"}"
check "创建子部门 201" 201 "$STATUS"; RD=$(j "d['data']['id']")
check "子部门 path=父path+id/" "$HQ_PATH$RD/" "$(j "d['data']['path']")"
req POST /system/depts "$CH" "{\"name\":\"研发一组\",\"parent_id\":\"$RD\",\"sort\":1}"
check "创建孙部门 201" 201 "$STATUS"; RD1=$(j "d['data']['id']")
req POST /system/depts "$CH" "{\"name\":\"销售部\",\"parent_id\":\"$HQ\",\"sort\":1}"
check "创建第二个子部门 201" 201 "$STATUS"; SALES=$(j "d['data']['id']")
req POST /system/depts "$CH" "{\"name\":\"研发部\",\"parent_id\":\"$HQ\"}"
check "同级重名 409" 409 "$STATUS"
req POST /system/depts "$CH" "{\"name\":\"X\",\"parent_id\":\"00000000-0000-0000-0000-000000000001\"}"
check "上级不存在 400" 400 "$STATUS"
req POST /system/depts "$CH" "{\"name\":\"X\",\"leader_user_id\":\"00000000-0000-0000-0000-000000000001\"}"
check "负责人不存在 400" 400 "$STATUS"
req POST /system/depts "$CH" "{\"name\":\"\"}"
check "name 必填 400" 400 "$STATUS"
req POST /system/depts "$ADMIN" "{\"name\":\"X\",\"parent_id\":\"$HQ\"}"
check "跨租户：平台管理员未指定 X-Tenant-ID 看不到 chenhua 的部门 400" 400 "$STATUS"
req POST /system/depts "$ADMIN" "{\"name\":\"平台侧建$SUF\",\"parent_id\":\"$HQ\",\"sort\":3}" "X-Tenant-ID: $CH_TID"
check "平台管理员带 X-Tenant-ID 可在 chenhua 下建部门 201" 201 "$STATUS"; PLAT_MADE=$(j "d['data']['id']")

req GET /system/depts/tree "$CH"
check "部门树 200" 200 "$STATUS"
check "树：总部在根" True "$(j "any(n['id']=='$HQ' for n in d['data'])")"
check "树：总部子节点按 sort 排序(销售部,研发部,平台侧建)" "['销售部', '研发部', '平台侧建$SUF']" "$(j "[c['name'] for n in d['data'] if n['id']=='$HQ' for c in n['children']]")"
check "树：研发部含研发一组" "['研发一组']" "$(j "[g['name'] for n in d['data'] if n['id']=='$HQ' for c in n['children'] if c['id']=='$RD' for g in c['children']]")"
check "树：叶子 children 为空数组" "[]" "$(j "[g['children'] for n in d['data'] if n['id']=='$HQ' for c in n['children'] if c['id']=='$RD' for g in c['children']][0]")"
check "树：节点带 user_count" 0 "$(j "[n['user_count'] for n in d['data'] if n['id']=='$HQ'][0]")"
req GET "/system/depts?keyword=%E7%A0%94%E5%8F%91" "$CH"   # 研发
check "列表 keyword 200" 200 "$STATUS"
check "列表 keyword=研发 命中 2" 2 "$(j "len([x for x in d['data'] if x['id'] in ('$RD','$RD1')])")"
req GET "/system/depts?status=disabled" "$CH"
check "列表 status 过滤 200" 200 "$STATUS"
req GET "/system/depts?status=bogus" "$CH"
check "列表 status 非法 400" 400 "$STATUS"
req GET "/system/depts/$RD" "$CH"
check "详情 200" 200 "$STATUS"; check "详情 code" RD "$(j "d['data']['code']")"
req GET /system/depts/00000000-0000-0000-0000-000000000001 "$CH"
check "详情不存在 404" 404 "$STATUS"
req GET /system/depts/not-a-uuid "$CH"
check "详情 id 非 uuid 400" 400 "$STATUS"

section "部门：移动 / 编辑"
req PUT "/system/depts/$RD" "$CH" "{\"parent_id\":\"$RD1\"}"
check "移到自己的子部门下 400" 400 "$STATUS"
req PUT "/system/depts/$RD" "$CH" "{\"parent_id\":\"$RD\"}"
check "移到自身下 400" 400 "$STATUS"
req PUT "/system/depts/$RD1" "$CH" "{\"parent_id\":\"$SALES\"}"
check "研发一组移到销售部 200" 200 "$STATUS"
check "移动后 path 重算" "$HQ_PATH$SALES/$RD1/" "$(j "d['data']['path']")"
check "移动后 parent_id" "$SALES" "$(j "d['data']['parent_id']")"
req PUT "/system/depts/$SALES" "$CH" "{\"clear_parent\":true,\"name\":\"销售中心$SUF\"}"
check "销售部 clear_parent 置根 200" 200 "$STATUS"
check "置根后 path=/id/" "/$SALES/" "$(j "d['data']['path']")"
req GET "/system/depts/$RD1" "$CH"
check "子树 path 随之更新" "/$SALES/$RD1/" "$(j "d['data']['path']")"
req PUT "/system/depts/$RD1" "$CH" "{\"parent_id\":\"$RD\"}"
check "研发一组移回研发部 200" 200 "$STATUS"
check "移回后 path" "$HQ_PATH$RD/$RD1/" "$(j "d['data']['path']")"
req PUT "/system/depts/$RD" "$CH" "{\"name\":\"平台侧建$SUF\"}"
check "改名撞同级重名 409" 409 "$STATUS"
req PUT "/system/depts/$RD" "$CH" "{\"sort\":9,\"status\":\"disabled\",\"monthly_budget\":88}"
check "改 sort/status/budget 200" 200 "$STATUS"
check "status=disabled" disabled "$(j "d['data']['status']")"; check "sort=9" 9 "$(j "d['data']['sort']")"

# 部门里放一个用户：user_count / leader / 删除拒绝
req POST /system/users "$CH" "{\"username\":\"dept_u_$SUF\",\"name\":\"部门用户\",\"dept_id\":\"$RD1\"}"
check "新建部门用户 201" 201 "$STATUS"; DU=$(j "d['data']['id']")
req PUT "/system/depts/$RD1" "$CH" "{\"leader_user_id\":\"$DU\"}"
check "设置负责人 200" 200 "$STATUS"; check "leader_name" "部门用户" "$(j "d['data']['leader_name']")"
req GET /system/depts/tree "$CH"
check "树：研发一组 user_count=1" 1 "$(j "[g['user_count'] for n in d['data'] if n['id']=='$HQ' for c in n['children'] if c['id']=='$RD' for g in c['children'] if g['id']=='$RD1'][0]")"
check "树：研发一组 leader_name" "部门用户" "$(j "[g['leader_name'] for n in d['data'] if n['id']=='$HQ' for c in n['children'] if c['id']=='$RD' for g in c['children'] if g['id']=='$RD1'][0]")"
req PUT "/system/depts/$RD1" "$CH" "{\"clear_leader\":true}"
check "clear_leader 200" 200 "$STATUS"; check "leader 清空" None "$(j "d['data']['leader_user_id']")"

section "部门：删除"
req DELETE "/system/depts/$RD" "$CH"
check "有子部门 409" 409 "$STATUS"
req DELETE "/system/depts/$RD1" "$CH"
check "有在职用户 409" 409 "$STATUS"
req PUT "/system/users/$DU" "$CH" '{"clear_dept":true}'
check "用户移出部门 200" 200 "$STATUS"
req DELETE "/system/depts/$RD1" "$CH"
check "无用户后删除 200" 200 "$STATUS"
req GET "/system/depts/$RD1" "$CH"
check "删除后 404" 404 "$STATUS"
req DELETE "/system/depts/$RD" "$CH"
check "子部门删掉后可删 200" 200 "$STATUS"
req DELETE "/system/depts/$PLAT_MADE" "$CH";  check "清理平台侧建部门 200" 200 "$STATUS"
req DELETE "/system/depts/$SALES" "$CH";      check "清理销售中心 200" 200 "$STATUS"
req DELETE "/system/depts/$HQ" "$CH";         check "清理总部 200" 200 "$STATUS"
req DELETE "/system/users/$DU" "$CH";         check "清理部门用户 200" 200 "$STATUS"

# ---------------------------------------------------------------- 角色与权限
section "权限树"
req GET /system/permissions/tree "$CH"
check "租户权限树 200" 200 "$STATUS"
check "非平台租户不含 system:tenant:*" False "$(j "'system:tenant:view' in json.dumps(d['data'])")"
check "非平台租户不含 system.tenant 菜单" False "$(j "'system.tenant' in json.dumps(d['data'])")"
check "根为菜单" menu "$(j "d['data'][0]['type']")"
check "system 菜单含 system.user 子菜单，其子为 action" action "$(j "[c for m in d['data'] if m['code']=='system' for c in m['children'] if c['code']=='system.user'][0]['children'][0]['type']")"
req GET /system/permissions/tree "$ADMIN"
check "平台租户权限树含 system:tenant:view" True "$(j "'system:tenant:view' in json.dumps(d['data'])")"

section "角色：列表 / 简表 / 创建"
req GET "/system/roles?pageSize=50" "$CH"
check "角色列表 200" 200 "$STATUS"
check "内置角色 5 个" 5 "$(j "len([r for r in d['data']['items'] if r['is_system']])")"
check "tenant_admin 持有者>=1" True "$(j "[r['user_count'] for r in d['data']['items'] if r['code']=='tenant_admin'][0] >= 1")"
check "tenant_admin 权限非空" True "$(j "len([r for r in d['data']['items'] if r['code']=='tenant_admin'][0]['permissions']) > 10")"
check "租户列表不含平台级角色" 0 "$(j "len([r for r in d['data']['items'] if r['tenant_id'] is None])")"
TA=$(j "[r['id'] for r in d['data']['items'] if r['code']=='tenant_admin'][0]")
req GET "/system/roles?keyword=fleet" "$CH"
check "keyword=fleet 命中 1" 1 "$(j "d['data']['total']")"
req GET "/system/roles?pageSize=2&page=2&sort=code" "$CH"
check "分页 page=2 pageSize=2" "[2, 2]" "$(j "[d['data']['page'], d['data']['pageSize']]")"
check "sort=code 升序" True "$(j "[r['code'] for r in d['data']['items']] == sorted(r['code'] for r in d['data']['items'])")"
req GET "/system/roles?pageSize=50" "$ADMIN"
check "平台租户列表含 super_admin(tenant_id null)" 1 "$(j "len([r for r in d['data']['items'] if r['code']=='super_admin' and r['tenant_id'] is None])")"
req GET "/system/roles?pageSize=50" "$ADMIN" "" "X-Tenant-ID: $CH_TID"
check "平台管理员切到 chenhua 不含平台级角色" 0 "$(j "len([r for r in d['data']['items'] if r['tenant_id'] is None])")"
req GET /system/roles/options "$CH"
check "角色简表 200" 200 "$STATUS"
check "简表按 code 排序" True "$(j "[r['code'] for r in d['data']] == sorted(r['code'] for r in d['data'])")"
check "简表只有 id/code/name" "['code', 'id', 'name']" "$(j "sorted(d['data'][0].keys())")"

ROLE=tester_$SUF
req POST /system/roles "$CH" "{\"code\":\"Bad-Code\",\"name\":\"x\"}";                            check "code 不合法 400" 400 "$STATUS"
req POST /system/roles "$CH" "{\"code\":\"$ROLE\",\"name\":\"x\",\"permissions\":[\"system:user:nope\"]}"; check "未知权限码 400" 400 "$STATUS"
req POST /system/roles "$CH" "{\"code\":\"$ROLE\",\"name\":\"x\",\"permissions\":[\"system:tenant:view\"]}"; check "非平台租户含平台专属 400" 400 "$STATUS"
req POST /system/roles "$CH" "{\"code\":\"tenant_admin\",\"name\":\"x\"}";                          check "code 重复 409" 409 "$STATUS"
req POST /system/roles "$CH" "{\"code\":\"$ROLE\",\"name\":\"测试员\",\"description\":\"e2e\",\"permissions\":[\"dashboard:view\",\"dashboard:view\"]}"
check "创建角色 201" 201 "$STATUS"; RID=$(j "d['data']['id']")
check "权限去重写入" "['dashboard:view']" "$(j "d['data']['permissions']")"
check "user_count=0" 0 "$(j "d['data']['user_count']")"; check "is_system=false" False "$(j "d['data']['is_system']")"
req POST /system/roles "$ADMIN" "{\"code\":\"plat_$SUF\",\"name\":\"平台角色\",\"permissions\":[\"system:tenant:view\"]}"
check "平台租户可含平台专属权限 201" 201 "$STATUS"; PRID=$(j "d['data']['id']")
req GET "/system/roles/$RID" "$CH";     check "角色详情 200" 200 "$STATUS"
req GET "/system/roles/$PRID" "$CH";    check "看不到别的租户的角色 404" 404 "$STATUS"

section "角色：编辑 / 权限设置（持有者立即生效）"
req PUT "/system/roles/$RID" "$CH" '{"name":"测试员2","description":""}'
check "改名 200" 200 "$STATUS"; check "name 已改" 测试员2 "$(j "d['data']['name']")"; check "description 清空为 null" None "$(j "d['data']['description']")"
req PUT "/system/roles/$TA" "$CH" '{"description":"内置角色可改描述"}'
check "内置角色改描述 200" 200 "$STATUS"; check "code 不变" tenant_admin "$(j "d['data']['code']")"

req POST /system/users "$CH" "{\"username\":\"tester_$SUF\",\"name\":\"测试用户\",\"password\":\"Tester@12345\",\"role_ids\":[\"$RID\"]}"
check "创建持有该角色的用户 201" 201 "$STATUS"; TU=$(j "d['data']['id']")
login "tester_$SUF" Tester@12345 chenhua; TT=$TOKEN; check "测试用户登录" 200 "$STATUS"
req GET /system/users "$TT";      check "无 system:user:view → 403" 403 "$STATUS"
req PUT "/system/roles/$RID/permissions" "$CH" '{"permissions":["dashboard:view","system:user:view","system:dept:view"]}'
check "设置权限 200" 200 "$STATUS"
check "权限已替换" "['dashboard:view', 'system:dept:view', 'system:user:view']" "$(j "d['data']['permissions']")"
req GET /system/users "$TT";      check "同一 token 立即拥有 system:user:view → 200" 200 "$STATUS"
req GET /auth/me "$TT";           check "/auth/me 权限含 system:dept:view" True "$(j "'system:dept:view' in d['data']['permissions']")"
req PUT "/system/roles/$RID/permissions" "$CH" '{"permissions":["system:tenant:create"]}'
check "设置平台专属 400" 400 "$STATUS"
req PUT "/system/roles/$RID/permissions" "$CH" '{"permissions":["nope"]}'
check "设置未知码 400" 400 "$STATUS"
req PUT "/system/roles/$RID/permissions" "$CH" '{}'
check "permissions 必填 400" 400 "$STATUS"
req PUT "/system/roles/$RID/permissions" "$CH" '{"permissions":[]}'
check "清空权限 200" 200 "$STATUS"
req GET /system/users "$TT";      check "权限收回后立即 403" 403 "$STATUS"
req PUT "/system/roles/$RID/permissions" "$TT" '{"permissions":[]}'
check "无 assign-perms 权限的用户设置权限 403" 403 "$STATUS"

section "角色：删除"
req DELETE "/system/roles/$TA" "$CH";   check "内置角色 409" 409 "$STATUS"
req DELETE "/system/roles/$RID" "$CH";  check "仍有持有者 409" 409 "$STATUS"
req PUT "/system/users/$TU/roles" "$CH" '{"role_ids":[]}';  check "解除用户角色 200" 200 "$STATUS"
req DELETE "/system/roles/$RID" "$CH";  check "无持有者后删除 200" 200 "$STATUS"
req GET "/system/roles/$RID" "$CH";     check "删除后 404" 404 "$STATUS"
req GET /system/roles/options "$CH";    check "简表不再包含已删角色" 0 "$(j "len([r for r in d['data'] if r['id']=='$RID'])")"
req POST /system/roles "$CH" "{\"code\":\"$ROLE\",\"name\":\"重建\"}"; check "软删后 code 可重建 201" 201 "$STATUS"; RID2=$(j "d['data']['id']")
req DELETE "/system/roles/$RID2" "$CH"; check "清理重建角色 200" 200 "$STATUS"
req DELETE "/system/roles/$PRID" "$ADMIN"; check "清理平台角色 200" 200 "$STATUS"
req DELETE "/system/users/$TU" "$CH";   check "清理测试用户 200" 200 "$STATUS"

# ---------------------------------------------------------------- 租户
section "租户：权限 / 列表"
req GET /system/tenants "$CH";           check "租户管理员访问租户列表 403" 403 "$STATUS"
req GET /system/tenants "$ADMIN";        check "租户列表 200" 200 "$STATUS"
check "含平台与 chenhua" True "$(j "d['data']['total'] >= 2")"
check "平台租户 is_platform" True "$(j "[t['is_platform'] for t in d['data']['items'] if t['code']=='platform'][0]")"
check "chenhua user_count>=1" True "$(j "[t['user_count'] for t in d['data']['items'] if t['code']=='chenhua'][0] >= 1")"
check "项含 dept_count" True "$(j "'dept_count' in d['data']['items'][0]")"
req GET "/system/tenants?keyword=chen" "$ADMIN";     check "keyword 命中 1" 1 "$(j "d['data']['total']")"
req GET "/system/tenants?status=bogus" "$ADMIN";     check "status 非法 400" 400 "$STATUS"
req GET "/system/tenants?sort=-code&pageSize=1" "$ADMIN"; check "sort=-code pageSize=1" 1 "$(j "len(d['data']['items'])")"

section "租户：创建"
TC=e2e-$SUF
req POST /system/tenants "$ADMIN" "{\"code\":\"Bad_Code\",\"name\":\"x\",\"admin_username\":\"a_admin\"}"; check "code 不合法 400" 400 "$STATUS"
req POST /system/tenants "$ADMIN" "{\"code\":\"$TC\",\"name\":\"x\"}";                                  check "admin_username 必填 400" 400 "$STATUS"
req POST /system/tenants "$ADMIN" "{\"code\":\"$TC\",\"name\":\"x\",\"admin_username\":\"a_admin\",\"admin_password\":\"weak\"}"; check "弱密码 400" 400 "$STATUS"
req POST /system/tenants "$ADMIN" "{\"code\":\"chenhua\",\"name\":\"x\",\"admin_username\":\"a_admin\"}"; check "code 重复 409" 409 "$STATUS"
req POST /system/tenants "$ADMIN" "{\"code\":\"$TC\",\"name\":\"E2E租户\",\"contact_name\":\"张三\",\"admin_username\":\"e2e_admin\",\"admin_password\":\"E2e@12345\",\"expires_at\":\"2030-01-01T00:00:00Z\"}"
check "创建租户 201" 201 "$STATUS"; TID=$(j "d['data']['id']")
check "user_count=1（管理员）" 1 "$(j "d['data']['user_count']")"
check "status=active" active "$(j "d['data']['status']")"; check "is_platform=false" False "$(j "d['data']['is_platform']")"
check "expires_at 已设" True "$(j "d['data']['expires_at'] is not None")"
req POST /system/tenants "$CH" "{\"code\":\"x-$SUF\",\"name\":\"x\",\"admin_username\":\"a_admin\"}";   check "租户管理员创建租户 403" 403 "$STATUS"
req GET "/system/tenants/$TID" "$ADMIN"; check "租户详情 200" 200 "$STATUS"; check "详情 contact_name" 张三 "$(j "d['data']['contact_name']")"
req GET "/system/tenants/00000000-0000-0000-0000-000000000001" "$ADMIN"; check "详情不存在 404" 404 "$STATUS"

login e2e_admin E2e@12345 "$TC"; E2E=$TOKEN; E2E_RT=$RT; check "新租户管理员可登录" 200 "$STATUS"
check "绑定 tenant_admin" True "$(j "'tenant_admin' in [r['code'] for r in d['data']['user']['roles']]")"
check "拥有本租户权限但无平台专属" "[True, False]" "$(j "['system:user:view' in d['data']['user']['permissions'], 'system:tenant:view' in d['data']['user']['permissions']]")"
req GET "/system/roles?pageSize=50" "$E2E"; check "新租户内置角色 5 个" 5 "$(j "d['data']['total']")"
req GET /system/depts/tree "$E2E";          check "新租户部门树为空数组" "[]" "$(j "d['data']")"
# 缺省密码路径：不传 admin_password
req POST /system/tenants "$ADMIN" "{\"code\":\"$TC-d\",\"name\":\"缺省密码租户\",\"admin_username\":\"d_admin\"}"
check "不传密码创建 201" 201 "$STATUS"; TID2=$(j "d['data']['id']")
login d_admin Zy@123456 "$TC-d"; check "缺省密码 Zy@123456 可登录" 200 "$STATUS"

section "租户：编辑 / 停用 / 恢复"
req PUT "/system/tenants/$TID" "$ADMIN" '{"name":"E2E租户改","license_no":"L-1","clear_expires":true}'
check "编辑 200" 200 "$STATUS"; check "name 已改" E2E租户改 "$(j "d['data']['name']")"
check "clear_expires 置空" None "$(j "d['data']['expires_at']")"; check "license_no" L-1 "$(j "d['data']['license_no']")"
req PUT "/system/tenants/$PLATFORM_TID" "$ADMIN" '{"status":"disabled"}';  check "停用平台租户 400" 400 "$STATUS"
req DELETE "/system/tenants/$PLATFORM_TID" "$ADMIN";                        check "DELETE 平台租户 400" 400 "$STATUS"
req GET /auth/me "$E2E";  check "停用前 e2e_admin 正常 200" 200 "$STATUS"
req DELETE "/system/tenants/$TID" "$ADMIN"
check "DELETE=停用 200" 200 "$STATUS"
req GET "/system/tenants/$TID" "$ADMIN"; check "状态 disabled" disabled "$(j "d['data']['status']")"
req GET /auth/me "$E2E";                 check "停用后原 access token 立即 403" 403 "$STATUS"
req POST /auth/refresh "" "{\"refresh_token\":\"$E2E_RT\"}"; check "停用后 refresh token 401" 401 "$STATUS"
login e2e_admin E2e@12345 "$TC"; check "停用后登录 403" 403 "$STATUS"
req PUT "/system/tenants/$TID" "$ADMIN" '{"status":"active"}'
check "恢复 active 200" 200 "$STATUS"; check "status=active" active "$(j "d['data']['status']")"
login e2e_admin E2e@12345 "$TC"; check "恢复后可登录" 200 "$STATUS"
req PUT "/system/tenants/$TID" "$ADMIN" '{"status":"disabled"}';  check "PUT status=disabled 200" 200 "$STATUS"
req PUT "/system/tenants/$TID2" "$ADMIN" '{"status":"disabled"}'; check "停用缺省密码租户 200" 200 "$STATUS"
req GET "/system/tenants?status=disabled" "$ADMIN"; check "status=disabled 过滤含 e2e" True "$(j "'$TID' in [t['id'] for t in d['data']['items']]")"

# ---------------------------------------------------------------- 汇总
echo
echo "== 结果：PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
