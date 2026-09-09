# ADR-0002 接口契约先行（OpenAPI）

日期：2026-09-09 | 状态：已采纳

## 决策

每个阶段开工前先把该阶段全部接口写进 `apps/api/api/openapi.yaml`（路径、参数、字段、语义、权限码 `x-permission`），前端用 `openapi-typescript` 从契约生成类型（`apps/web/src/api/schema.d.ts`），后端按契约实现。契约在实现期间视为只读，不一致之处先改契约再改代码。

## 统一约定（写进契约的 description）

- 前缀 `/api/v1`；JWT Bearer；响应信封 `{code, message, data, request_id}`，`code` 为 0 表示成功
- 列表参数 `page` / `pageSize` / `sort`（`field` 或 `-field`），返回 `{items, total, page, pageSize}`
- 时间 RFC3339；ID 为 UUID；软删除对外不可见
- 多租户：普通用户只能操作自己的租户；超级管理员可用 `X-Tenant-ID` 指定目标租户
- "全局 + 租户覆盖"型数据（字典、参数、通知模板）：读时租户行覆盖全局行；超级管理员不带 `X-Tenant-ID` 时写全局行

## 理由

前后端并行开发的前提是稳定的契约；生成类型消除了手写类型漂移。
