-- +goose Up
-- 阶段 1：租户、部门、用户、角色权限、字典、参数、审计、通知模板

CREATE TABLE tenants (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code          text NOT NULL UNIQUE,
  name          text NOT NULL,
  license_no    text,
  contact_name  text,
  contact_phone text,
  status        text NOT NULL DEFAULT 'active',      -- active | disabled
  is_platform   boolean NOT NULL DEFAULT false,      -- 平台租户（超级管理员所属）
  expires_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_tenants_updated BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE departments (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  parent_id      uuid REFERENCES departments(id),
  name           text NOT NULL,
  code           text,
  path           text NOT NULL DEFAULT '',            -- 祖先链 '/id1/id2/'，含自身
  sort           int  NOT NULL DEFAULT 0,
  leader_user_id uuid,                                -- 部门负责人（一级审批人）
  monthly_budget numeric(12,2) NOT NULL DEFAULT 0,
  status         text NOT NULL DEFAULT 'active',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE INDEX idx_departments_tenant ON departments(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_departments_parent ON departments(parent_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_departments_updated BEFORE UPDATE ON departments FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE users (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           uuid NOT NULL REFERENCES tenants(id),
  dept_id             uuid REFERENCES departments(id),
  username            text NOT NULL,
  password_hash       text NOT NULL,
  name                text NOT NULL,
  phone               text,
  email               text,
  employee_no         text,
  avatar_url          text,
  status              text NOT NULL DEFAULT 'active',  -- active | disabled | locked
  is_super            boolean NOT NULL DEFAULT false,  -- 平台超级管理员
  password_changed_at timestamptz,
  last_login_at       timestamptz,
  last_login_ip       text,
  created_by          uuid,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz
);
CREATE UNIQUE INDEX uq_users_tenant_username ON users(tenant_id, username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_tenant_phone ON users(tenant_id, phone) WHERE deleted_at IS NULL AND phone IS NOT NULL;
CREATE INDEX idx_users_dept ON users(dept_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE departments
  ADD CONSTRAINT fk_departments_leader FOREIGN KEY (leader_user_id) REFERENCES users(id);

CREATE TABLE roles (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid REFERENCES tenants(id),             -- NULL = 平台级角色
  code        text NOT NULL,
  name        text NOT NULL,
  description text,
  is_system   boolean NOT NULL DEFAULT false,          -- 内置角色不可删除
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX uq_roles_tenant_code
  ON roles(COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_roles_updated BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 权限点由代码注册表定义，启动时同步到此表（便于查询与前端展示）
CREATE TABLE permissions (
  code        text PRIMARY KEY,                       -- module:resource:action 或菜单 code
  name        text NOT NULL,
  type        text NOT NULL,                          -- menu | action
  parent_code text REFERENCES permissions(code),
  path        text,                                   -- 菜单路由
  icon        text,
  sort        int NOT NULL DEFAULT 0,
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
  role_id         uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_code text NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_code)
);

CREATE TABLE user_roles (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);
CREATE INDEX idx_user_roles_role ON user_roles(role_id);

CREATE TABLE dict_types (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid REFERENCES tenants(id),             -- NULL = 全局字典
  code        text NOT NULL,
  name        text NOT NULL,
  description text,
  is_system   boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_dict_types_code
  ON dict_types(COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code);
CREATE TRIGGER trg_dict_types_updated BEFORE UPDATE ON dict_types FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE dict_items (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  dict_type_id uuid NOT NULL REFERENCES dict_types(id) ON DELETE CASCADE,
  label        text NOT NULL,
  value        text NOT NULL,
  sort         int  NOT NULL DEFAULT 0,
  color        text,
  status       text NOT NULL DEFAULT 'active',
  extra        jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (dict_type_id, value)
);
CREATE TRIGGER trg_dict_items_updated BEFORE UPDATE ON dict_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE sys_params (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid REFERENCES tenants(id),             -- NULL = 全局缺省，租户行覆盖
  key         text NOT NULL,
  value       text NOT NULL,
  value_type  text NOT NULL DEFAULT 'string',          -- string | int | float | bool | json
  description text,
  is_public   boolean NOT NULL DEFAULT false,          -- 是否允许未登录/前端直接读取
  updated_by  uuid,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_sys_params_key
  ON sys_params(COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), key);
CREATE TRIGGER trg_sys_params_updated BEFORE UPDATE ON sys_params FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 审计日志只写不改不删
CREATE TABLE audit_logs (
  id          bigserial PRIMARY KEY,
  tenant_id   uuid,
  user_id     uuid,
  username    text,
  action      text NOT NULL,                           -- create | update | delete | login | logout | ...
  module      text NOT NULL,                           -- system.user / auth / ...
  target_type text,
  target_id   text,
  summary     text,
  before      jsonb,
  after       jsonb,
  method      text,
  path        text,
  status      int,
  ip          text,
  user_agent  text,
  request_id  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_tenant_time ON audit_logs(tenant_id, created_at DESC);
CREATE INDEX idx_audit_logs_user ON audit_logs(user_id, created_at DESC);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id);

CREATE TABLE notification_templates (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid REFERENCES tenants(id),              -- NULL = 全局缺省
  code       text NOT NULL,                            -- 事件码，如 approval.submitted
  channel    text NOT NULL,                            -- inapp | wechat | sms
  title      text NOT NULL,
  content    text NOT NULL,                            -- 支持 {{变量}}
  enabled    boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_notification_templates
  ON notification_templates(COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code, channel);
CREATE TRIGGER trg_notification_templates_updated BEFORE UPDATE ON notification_templates FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS sys_params;
DROP TABLE IF EXISTS dict_items;
DROP TABLE IF EXISTS dict_types;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
ALTER TABLE departments DROP CONSTRAINT IF EXISTS fk_departments_leader;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS tenants;
