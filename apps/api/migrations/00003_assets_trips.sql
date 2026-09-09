-- +goose Up
-- 阶段 2：车辆资产、设备、NFC 卡、充电桩档案；车辆实时状态与遥测；审批工作流；行程与轨迹；站内通知

-- ---------- 资产 ----------
CREATE TABLE vehicles (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id),
  plate_no         text NOT NULL,
  vin              text,
  brand            text,
  model            text,
  color            text,
  seat_count       int  NOT NULL DEFAULT 5,
  battery_kwh      numeric(6,2) NOT NULL DEFAULT 60,   -- 电池容量，用于 SOC→kWh 估算
  range_km_full    int  NOT NULL DEFAULT 400,          -- 满电标称续航
  status           text NOT NULL DEFAULT 'idle',       -- idle | in_use | charging | maintenance | disabled
  odometer_km      numeric(10,1) NOT NULL DEFAULT 0,
  soc              numeric(5,2),
  soh              numeric(5,2),
  purchase_date    date,
  insurance_expire date,
  inspection_expire date,
  home_dept_id     uuid REFERENCES departments(id),
  remark           text,
  created_by       uuid,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz
);
CREATE UNIQUE INDEX uq_vehicles_tenant_plate ON vehicles(tenant_id, plate_no) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_vehicles_vin ON vehicles(vin) WHERE deleted_at IS NULL AND vin IS NOT NULL;
CREATE INDEX idx_vehicles_tenant_status ON vehicles(tenant_id, status) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_vehicles_updated BEFORE UPDATE ON vehicles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE devices (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id),
  serial_no       text NOT NULL,                       -- VIG-100E 网关序列号，全局唯一
  vehicle_id      uuid REFERENCES vehicles(id),        -- 绑定车辆（一车一网关）
  api_key_hash    text NOT NULL,                       -- sha256(api_key)，设备上报鉴权
  model           text NOT NULL DEFAULT 'VIG-100E',
  firmware        text,
  iccid           text,
  status          text NOT NULL DEFAULT 'active',      -- active | disabled
  last_online_at  timestamptz,
  last_ip         text,
  remark          text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE UNIQUE INDEX uq_devices_serial ON devices(serial_no) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_devices_vehicle ON devices(vehicle_id) WHERE deleted_at IS NULL AND vehicle_id IS NOT NULL;
CREATE TRIGGER trg_devices_updated BEFORE UPDATE ON devices FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE nfc_cards (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid NOT NULL REFERENCES tenants(id),
  card_uid   text NOT NULL,
  user_id    uuid REFERENCES users(id),
  status     text NOT NULL DEFAULT 'active',           -- active | lost | disabled
  issued_at  timestamptz,
  remark     text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_nfc_cards_uid ON nfc_cards(tenant_id, card_uid) WHERE deleted_at IS NULL;
CREATE INDEX idx_nfc_cards_user ON nfc_cards(user_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_nfc_cards_updated BEFORE UPDATE ON nfc_cards FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE charge_piles (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         uuid NOT NULL REFERENCES tenants(id),
  pile_code         text NOT NULL,                    -- OCPP ChargePoint identity
  name              text NOT NULL,
  type              text NOT NULL DEFAULT 'slow',     -- fast | slow
  power_kw          numeric(6,2) NOT NULL DEFAULT 7,
  connector_count   int NOT NULL DEFAULT 1,
  vendor            text,
  location          text,
  lng               double precision,
  lat               double precision,
  status            text NOT NULL DEFAULT 'offline',  -- available | charging | offline | faulted | disabled
  last_heartbeat_at timestamptz,
  remark            text,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz
);
CREATE UNIQUE INDEX uq_charge_piles_code ON charge_piles(tenant_id, pile_code) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_charge_piles_updated BEFORE UPDATE ON charge_piles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- 车辆实时状态与遥测 ----------
CREATE TABLE vehicle_status (
  vehicle_id        uuid PRIMARY KEY REFERENCES vehicles(id) ON DELETE CASCADE,
  tenant_id         uuid NOT NULL,
  status            text NOT NULL DEFAULT 'idle',
  current_trip_id   uuid,
  driver_id         uuid,
  lng               double precision,
  lat               double precision,
  speed             numeric(6,1),
  heading           numeric(5,1),
  soc               numeric(5,2),
  soh               numeric(5,2),
  range_km          numeric(7,1),
  odometer_km       numeric(10,1),
  sign_on           boolean NOT NULL DEFAULT false,
  locked            boolean NOT NULL DEFAULT true,
  charging          boolean NOT NULL DEFAULT false,
  last_telemetry_at timestamptz,
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_vehicle_status_tenant ON vehicle_status(tenant_id);

CREATE TABLE vehicle_telemetry (
  ts          timestamptz NOT NULL,
  tenant_id   uuid NOT NULL,
  vehicle_id  uuid NOT NULL,
  device_id   uuid,
  lng         double precision,
  lat         double precision,
  speed       numeric(6,1),
  heading     numeric(5,1),
  soc         numeric(5,2),
  soh         numeric(5,2),
  cell_temp   numeric(5,1),
  motor_temp  numeric(5,1),
  odometer_km numeric(10,1),
  acc_on      boolean,
  locked      boolean,
  sign_on     boolean,
  charging    boolean,
  raw         jsonb
);
SELECT create_hypertable('vehicle_telemetry', 'ts', chunk_time_interval => INTERVAL '7 days');
CREATE INDEX idx_vehicle_telemetry_vehicle_ts ON vehicle_telemetry(vehicle_id, ts DESC);

-- ---------- 审批 ----------
CREATE TABLE approval_rules (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id              uuid NOT NULL UNIQUE REFERENCES tenants(id),
  enabled                boolean NOT NULL DEFAULT true,
  level2_km              numeric(8,1),                 -- 预计里程 ≥ 此值需二级审批；NULL 不启用
  level2_night           boolean NOT NULL DEFAULT true,-- 计划时段与夜间重叠需二级审批
  night_start            time NOT NULL DEFAULT '22:00',
  night_end              time NOT NULL DEFAULT '06:00',
  level2_cross_dept      boolean NOT NULL DEFAULT false, -- 申请车辆归属部门 ≠ 申请人部门需二级
  level2_trip_types      text[] NOT NULL DEFAULT '{}',   -- 指定用车类型一律二级，如 {official}
  level2_approver_id     uuid REFERENCES users(id),      -- 二级审批人；NULL 则取持有 approver 角色的用户
  fallback_approver_role text NOT NULL DEFAULT 'approver',
  overdue_alert_minutes  int NOT NULL DEFAULT 30,
  updated_by             uuid,
  updated_at             timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_approval_rules_updated BEFORE UPDATE ON approval_rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE approvals (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id),
  apply_no        text NOT NULL,                        -- ZY-20250618-0023
  applicant_id    uuid NOT NULL REFERENCES users(id),
  dept_id         uuid REFERENCES departments(id),
  trip_type       text NOT NULL DEFAULT 'official',     -- official | daily
  purpose_code    text NOT NULL,                        -- 字典 approval_purpose
  purpose_detail  text NOT NULL,
  planned_start   timestamptz NOT NULL,
  planned_end     timestamptz NOT NULL,
  destination     text NOT NULL,
  dest_lng        double precision,
  dest_lat        double precision,
  planned_route   jsonb,                                -- {points:[[lng,lat],...], distance_km, duration_min, summary}
  planned_km      numeric(8,1),
  passengers      jsonb NOT NULL DEFAULT '[]'::jsonb,   -- [{user_id,name}]
  attachments     jsonb NOT NULL DEFAULT '[]'::jsonb,   -- [{name,url}]
  vehicle_id      uuid REFERENCES vehicles(id),
  urgency         text NOT NULL DEFAULT 'normal',       -- normal | urgent
  status          text NOT NULL DEFAULT 'pending_l1',   -- pending_l1 | pending_l2 | approved | rejected | cancelled | in_use | completed | expired
  level_required  int  NOT NULL DEFAULT 1,
  current_step    int  NOT NULL DEFAULT 1,
  reject_reason   text,
  cancel_reason   text,
  approved_at     timestamptz,
  created_by      uuid,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_approvals_no ON approvals(apply_no);
CREATE INDEX idx_approvals_tenant_status ON approvals(tenant_id, status, planned_start DESC);
CREATE INDEX idx_approvals_applicant ON approvals(applicant_id, created_at DESC);
CREATE INDEX idx_approvals_vehicle_window ON approvals(vehicle_id, planned_start, planned_end) WHERE status IN ('pending_l1','pending_l2','approved','in_use');
CREATE TRIGGER trg_approvals_updated BEFORE UPDATE ON approvals FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE approval_steps (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  approval_id uuid NOT NULL REFERENCES approvals(id) ON DELETE CASCADE,
  step_no     int  NOT NULL,
  approver_id uuid NOT NULL REFERENCES users(id),
  action      text NOT NULL DEFAULT 'pending',          -- pending | approved | rejected | skipped
  remark      text,
  acted_at    timestamptz,
  ip          text,
  UNIQUE (approval_id, step_no)
);
CREATE INDEX idx_approval_steps_approver ON approval_steps(approver_id) WHERE action = 'pending';

-- ---------- 行程 ----------
CREATE TABLE trips (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id),
  trip_no          text NOT NULL,                       -- T-20250618-047
  vehicle_id       uuid NOT NULL REFERENCES vehicles(id),
  driver_id        uuid REFERENCES users(id),
  card_id          uuid REFERENCES nfc_cards(id),
  approval_id      uuid REFERENCES approvals(id),
  trip_type        text NOT NULL DEFAULT 'official',
  purpose          text,
  source           text NOT NULL DEFAULT 'device',      -- device | web | simulator
  status           text NOT NULL DEFAULT 'ongoing',     -- ongoing | completed | cancelled
  start_at         timestamptz NOT NULL,
  end_at           timestamptz,
  start_odometer   numeric(10,1),
  end_odometer     numeric(10,1),
  distance_km      numeric(8,1),
  energy_kwh       numeric(8,2),
  start_soc        numeric(5,2),
  end_soc          numeric(5,2),
  avg_speed        numeric(6,1),
  max_speed        numeric(6,1),
  harsh_accel      int NOT NULL DEFAULT 0,
  harsh_brake      int NOT NULL DEFAULT 0,
  point_count      int NOT NULL DEFAULT 0,
  roof_sign_status text NOT NULL DEFAULT 'unknown',     -- on | off | unknown
  deviation_flag   boolean NOT NULL DEFAULT false,
  deviation_max_m  numeric(10,1),
  start_lng        double precision,
  start_lat        double precision,
  end_lng          double precision,
  end_lat          double precision,
  cost             numeric(10,2),                       -- 阶段 3 计费引擎填写
  cost_detail      jsonb,
  remark           text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_trips_no ON trips(trip_no);
CREATE INDEX idx_trips_tenant_start ON trips(tenant_id, start_at DESC);
CREATE INDEX idx_trips_vehicle ON trips(vehicle_id, start_at DESC);
CREATE INDEX idx_trips_driver ON trips(driver_id, start_at DESC);
CREATE UNIQUE INDEX uq_trips_ongoing_vehicle ON trips(vehicle_id) WHERE status = 'ongoing';
CREATE TRIGGER trg_trips_updated BEFORE UPDATE ON trips FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE trip_points (
  ts         timestamptz NOT NULL,
  trip_id    uuid NOT NULL,
  vehicle_id uuid NOT NULL,
  lng        double precision NOT NULL,
  lat        double precision NOT NULL,
  speed      numeric(6,1),
  heading    numeric(5,1),
  soc        numeric(5,2)
);
SELECT create_hypertable('trip_points', 'ts', chunk_time_interval => INTERVAL '7 days');
CREATE INDEX idx_trip_points_trip_ts ON trip_points(trip_id, ts);

CREATE TABLE trip_events (
  id         bigserial PRIMARY KEY,
  tenant_id  uuid NOT NULL,
  trip_id    uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  vehicle_id uuid NOT NULL,
  type       text NOT NULL,                             -- start | end | deviation | overspeed | low_soc | sign_on | sign_off | cancel
  ts         timestamptz NOT NULL DEFAULT now(),
  payload    jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX idx_trip_events_trip ON trip_events(trip_id, ts);

-- ---------- 站内通知 ----------
CREATE TABLE notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid NOT NULL,
  user_id    uuid NOT NULL,
  type       text NOT NULL,                             -- approval.pending | approval.approved | ... | trip.ended | alert
  title      text NOT NULL,
  content    text NOT NULL DEFAULT '',
  ref_type   text,
  ref_id     text,
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_user ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id) WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS trip_events;
DROP TABLE IF EXISTS trip_points;
DROP TABLE IF EXISTS trips;
DROP TABLE IF EXISTS approval_steps;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS approval_rules;
DROP TABLE IF EXISTS vehicle_telemetry;
DROP TABLE IF EXISTS vehicle_status;
DROP TABLE IF EXISTS charge_piles;
DROP TABLE IF EXISTS nfc_cards;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS vehicles;
