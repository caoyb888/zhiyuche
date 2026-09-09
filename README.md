# 智御系统（ZhiYuChe）

智御车辆智能租赁与全生命周期管理系统。面向工业园区 B2B 场景的纯电动车队管理平台：公务用车审批、行程与轨迹、计费与账户、充电桩接入、车辆健康与 AI 诊断、系统管理。

- 技术方案：`docs/zhiyuche_v1_1.md`
- 实施计划：`docs/implementation_plan_v1.md`

## 仓库结构

```
apps/
  web/        Web 管理端：React 18 + TypeScript + Vite 5 + Tailwind 3 + Recharts
  api/        后端：Go 1.25 + Gin，模块化单体；cmd/{api,iot,ocpp,worker,simulator}
  mobile/     员工移动端（uni-app，阶段 5）
packages/     前端共享包
deploy/       docker-compose（PostgreSQL+TimescaleDB / Redis / EMQX / NATS / MinIO）、K3s 清单
scripts/      开发辅助脚本（同步到远程编译机、远程执行）
docs/         方案、计划、接口文档、ADR
.github/      CI
```

## 开发流程

本仓库采用 **本机写代码、远程机器编译运行** 的方式（本机不装 Go / Docker）。

```powershell
# 1. 把本地工作区同步到远程编译机（无需先 git 提交）
.\scripts\sync-remote.ps1

# 2. 在远程项目目录执行命令
.\scripts\remote.ps1 "make check"                 # vet + test + 前端 typecheck/lint
.\scripts\remote.ps1 "make infra-up"              # 启动基础设施容器
.\scripts\remote.ps1 "make api-migrate CMD=up"    # 数据库迁移
.\scripts\remote.ps1 "make api-run"               # 前台运行 api
.\scripts\remote.ps1 "cd apps/web && npm run dev" # 前端 dev server（端口 20173）
```

远程机器是共享环境，本项目固定使用 **20000–20999** 端口段：

| 服务 | 端口 |
|---|---|
| Web dev server | 20173 |
| API | 20080 |
| OCPP（WebSocket） | 20081 |
| PostgreSQL | 20432 |
| Redis | 20379 |
| EMQX MQTT / WS / Dashboard | 20883 / 20084 / 20183 |
| NATS / 监控 | 20222 / 20822 |
| MinIO API / Console | 20900 / 20901 |

在 Linux 机器上直接开发时，`make help` 列出全部目标；`make env` 从各 `.env.example` 生成缺省 `.env`。

## 车辆模拟器

没有真实车载网关时，用模拟器产生车辆、设备、驾驶员、申请、行程与遥测：

```bash
make api-start                                   # 后端跑在 20080
./bin/zhiyuche-simulator -vehicles 6 -interval 5s -speedup 10   # Ctrl+C 退出并保存状态
./bin/zhiyuche-simulator -telemetry-only         # 只上报遥测，不创建申请/行程
```

模拟器以演示租户管理员登录，首次运行创建 6 辆车（鲁A·S0001…）、对应网关（SIM-0001…）、3 名驾驶员（`sim_driver_1..3`，密码 `Sim@123456`，员工角色，可用来体验员工视角）与 NFC 卡、2 个充电桩，之后按"申请→审批→刷卡取车→沿路线行驶→还车→低电充电"循环运行。参数见 `-help`，状态保存在 `simulator-state.json`。

## 后端约定

- 路由前缀 `/api/v1`，JWT Bearer 鉴权
- 统一响应信封 `{code, message, data, request_id}`，`code` 为 0 表示成功
- 列表参数 `page` / `pageSize` / `sort`（`field` 或 `-field`）
- 配置全部来自环境变量（`ZY_` 前缀），可用 `.env` 文件本地覆盖
- 迁移用 goose，SQL 文件嵌入二进制：`zhiyuche-api migrate up|down|status`
- 接口契约：`apps/api/api/openapi.yaml`

## 前端约定

- npm workspaces：依赖在仓库根安装（`npm install`），lockfile 只有根目录一份；Vercel 从仓库根构建（见 `vercel.json`）
- 全量 TypeScript，`npm run typecheck` 与 `npm run lint` 零错误
- 请求层 `src/api/`，dev server 将 `/api` 代理到 `http://localhost:20080`
- 环境变量 `VITE_API_BASE`、`VITE_WS_URL`、`VITE_AMAP_KEY`

## License

MIT
