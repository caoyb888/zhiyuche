-- +goose Up
-- 阶段 3：计费规则、三级账户与流水、月度结算；充电桩连接器、充电事务与电表数据

-- ---------- 计费规则 ----------
CREATE TABLE billing_rules (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  name           text NOT NULL,
  rule           jsonb NOT NULL,                       -- 结构见 internal/billing/engine.Rule
  is_default     boolean NOT NULL DEFAULT false,       -- 当前生效规则（每租户至多一个）
  enabled        boolean NOT NULL DEFAULT true,
  effective_from date,
  effective_to   date,
  created_by     uuid,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE UNIQUE INDEX uq_billing_rules_default ON billing_rules(tenant_id) WHERE is_default AND deleted_at IS NULL;
CREATE TRIGGER trg_billing_rules_updated BEFORE UPDATE ON billing_rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- 账户 ----------
CREATE TABLE accounts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  level          text NOT NULL,                        -- enterprise | department | employee
  owner_id       uuid NOT NULL,                        -- enterprise: tenant_id；department: departments.id；employee: users.id
  balance        numeric(12,2) NOT NULL DEFAULT 0,
  credit_limit   numeric(12,2) NOT NULL DEFAULT 0,     -- 允许透支额度（余额可到 -credit_limit）
  monthly_budget numeric(12,2) NOT NULL DEFAULT 0,     -- 月度预算（部门级由 departments.monthly_budget 初始化）
  status         text NOT NULL DEFAULT 'active',       -- active | frozen
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, level, owner_id)
);
CREATE TRIGGER trg_accounts_updated BEFORE UPDATE ON accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE account_transactions (
  id            bigserial PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  account_id    uuid NOT NULL REFERENCES accounts(id),
  type          text NOT NULL,                         -- recharge | allocate_in | allocate_out | trip | charge | penalty | refund | adjust
  amount        numeric(12,2) NOT NULL,                -- 有符号：入账为正，扣费为负
  balance_after numeric(12,2) NOT NULL,
  ref_type      text,                                  -- trip | charge_transaction | account | settlement
  ref_id        text,
  remark        text,
  created_by    uuid,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_account_txn_account ON account_transactions(account_id, created_at DESC);
CREATE INDEX idx_account_txn_tenant ON account_transactions(tenant_id, created_at DESC);
CREATE INDEX idx_account_txn_ref ON account_transactions(ref_type, ref_id);

-- 行程计费状态
ALTER TABLE trips
  ADD COLUMN billing_status text NOT NULL DEFAULT 'pending',   -- pending | charged | skipped | failed
  ADD COLUMN billing_rule_id uuid,
  ADD COLUMN account_id uuid,
  ADD COLUMN account_txn_id bigint,
  ADD COLUMN billed_at timestamptz;
CREATE INDEX idx_trips_billing ON trips(tenant_id, billing_status) WHERE status = 'completed';

-- ---------- 月度结算 ----------
CREATE TABLE settlements (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id),
  period       text NOT NULL,                          -- YYYY-MM
  dept_id      uuid REFERENCES departments(id),        -- NULL = 企业汇总行
  trip_count   int NOT NULL DEFAULT 0,
  trip_cost    numeric(12,2) NOT NULL DEFAULT 0,
  charge_count int NOT NULL DEFAULT 0,
  charge_cost  numeric(12,2) NOT NULL DEFAULT 0,
  penalty      numeric(12,2) NOT NULL DEFAULT 0,
  total        numeric(12,2) NOT NULL DEFAULT 0,
  budget       numeric(12,2) NOT NULL DEFAULT 0,       -- 该期预算（部门月度预算）
  status       text NOT NULL DEFAULT 'draft',          -- draft | confirmed
  generated_at timestamptz NOT NULL DEFAULT now(),
  confirmed_at timestamptz,
  confirmed_by uuid,
  file_url     text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_settlements_period_dept
  ON settlements(tenant_id, period, COALESCE(dept_id, '00000000-0000-0000-0000-000000000000'::uuid));
CREATE TRIGGER trg_settlements_updated BEFORE UPDATE ON settlements FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE settlement_lines (
  id            bigserial PRIMARY KEY,
  settlement_id uuid NOT NULL REFERENCES settlements(id) ON DELETE CASCADE,
  kind          text NOT NULL,                         -- trip | charge | penalty
  ref_id        text NOT NULL,                         -- trips.id / charge_transactions.id
  ref_no        text,                                  -- 行程号 / 充电事务号
  user_id       uuid,
  user_name     text,
  vehicle_plate text,
  occurred_at   timestamptz NOT NULL,
  quantity      numeric(12,2),                         -- 里程 km / 电量 kWh
  amount        numeric(12,2) NOT NULL,
  detail        jsonb
);
CREATE INDEX idx_settlement_lines_settlement ON settlement_lines(settlement_id);

-- ---------- 充电 ----------
CREATE TABLE pile_connectors (
  pile_id      uuid NOT NULL REFERENCES charge_piles(id) ON DELETE CASCADE,
  connector_id int  NOT NULL,                          -- OCPP connectorId（0 表示整桩）
  status       text NOT NULL DEFAULT 'Unavailable',    -- OCPP ChargePointStatus：Available/Preparing/Charging/SuspendedEV/SuspendedEVSE/Finishing/Reserved/Unavailable/Faulted
  error_code   text,
  info         text,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (pile_id, connector_id)
);

CREATE SEQUENCE charge_tx_ocpp_seq START 1000;

CREATE TABLE charge_transactions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  tx_no          text NOT NULL,                        -- C-YYYYMMDD-NNN
  pile_id        uuid NOT NULL REFERENCES charge_piles(id),
  connector_id   int  NOT NULL DEFAULT 1,
  ocpp_tx_id     int  NOT NULL DEFAULT nextval('charge_tx_ocpp_seq'),  -- 返回给桩的 transactionId
  id_tag         text NOT NULL,                        -- 鉴权卡号（NFC 卡 UID）
  card_id        uuid REFERENCES nfc_cards(id),
  user_id        uuid REFERENCES users(id),
  dept_id        uuid REFERENCES departments(id),
  vehicle_id     uuid REFERENCES vehicles(id),
  bind_method    text,                                 -- card | location | recent_trip | manual | none
  status         text NOT NULL DEFAULT 'charging',     -- charging | ended | settled | cancelled
  start_at       timestamptz NOT NULL,
  end_at         timestamptz,
  meter_start    numeric(12,3) NOT NULL DEFAULT 0,     -- Wh
  meter_stop     numeric(12,3),
  kwh            numeric(10,3),
  unit_price     numeric(8,4),                         -- 元/kWh，来自计费规则
  cost           numeric(12,2),
  stop_reason    text,
  bms_soc_start  numeric(5,2),
  bms_soc_end    numeric(5,2),
  bms_kwh_est    numeric(10,3),
  deviation_pct  numeric(6,2),
  review_status  text NOT NULL DEFAULT 'none',         -- none | pending | approved | rejected
  review_note    text,
  reviewed_by    uuid,
  reviewed_at    timestamptz,
  attribution    text,                                 -- employee | department | enterprise
  account_id     uuid,
  account_txn_id bigint,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_charge_tx_no ON charge_transactions(tx_no);
CREATE UNIQUE INDEX uq_charge_tx_ocpp ON charge_transactions(pile_id, ocpp_tx_id);
CREATE UNIQUE INDEX uq_charge_tx_active ON charge_transactions(pile_id, connector_id) WHERE status = 'charging';
CREATE INDEX idx_charge_tx_tenant ON charge_transactions(tenant_id, start_at DESC);
CREATE INDEX idx_charge_tx_vehicle ON charge_transactions(vehicle_id, start_at DESC);
CREATE INDEX idx_charge_tx_review ON charge_transactions(tenant_id) WHERE review_status = 'pending';
CREATE TRIGGER trg_charge_tx_updated BEFORE UPDATE ON charge_transactions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE charge_meter_values (
  ts       timestamptz NOT NULL,
  tx_id    uuid NOT NULL,
  pile_id  uuid NOT NULL,
  wh       numeric(12,3),                              -- Energy.Active.Import.Register 累计
  voltage  numeric(8,2),
  current  numeric(8,2),
  power_kw numeric(8,3),
  soc      numeric(5,2),                               -- 桩侧上报的 SoC（若支持）
  raw      jsonb
);
SELECT create_hypertable('charge_meter_values', 'ts', chunk_time_interval => INTERVAL '30 days');
CREATE INDEX idx_charge_meter_tx ON charge_meter_values(tx_id, ts);

-- +goose Down
DROP TABLE IF EXISTS charge_meter_values;
DROP TABLE IF EXISTS charge_transactions;
DROP SEQUENCE IF EXISTS charge_tx_ocpp_seq;
DROP TABLE IF EXISTS pile_connectors;
DROP TABLE IF EXISTS settlement_lines;
DROP TABLE IF EXISTS settlements;
ALTER TABLE trips DROP COLUMN IF EXISTS billing_status, DROP COLUMN IF EXISTS billing_rule_id, DROP COLUMN IF EXISTS account_id,
  DROP COLUMN IF EXISTS account_txn_id, DROP COLUMN IF EXISTS billed_at;
DROP TABLE IF EXISTS account_transactions;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS billing_rules;
