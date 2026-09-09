-- +goose Up
-- 预约派车：调度台按电话（source=phone）或当面/系统（source=direct）录入的用车预约。
-- 与 approvals 的区别：不走审批流，录入即占用车辆；两张表共同参与车辆时段冲突检查。

CREATE TABLE bookings (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  booking_no     text NOT NULL,                        -- YY-20260909-0001
  source         text NOT NULL DEFAULT 'phone',        -- phone 电话预约 | direct 直接预约
  contact_name   text NOT NULL,                        -- 来电人 / 预约人姓名（未必是系统用户）
  contact_phone  text,
  passenger_id   uuid REFERENCES users(id),            -- 用车人（系统用户，可选）
  dept_id        uuid REFERENCES departments(id),      -- 用车人部门（创建时快照）
  reserve_start  timestamptz NOT NULL,
  reserve_end    timestamptz NOT NULL,
  vehicle_id     uuid REFERENCES vehicles(id),         -- 派车；为空表示待派车
  origin         text NOT NULL,
  origin_lng     double precision,
  origin_lat     double precision,
  destination    text NOT NULL,
  dest_lng       double precision,
  dest_lat       double precision,
  purpose        text,                                 -- 用车事由
  remark         text,
  status         text NOT NULL DEFAULT 'reserved',     -- reserved | departed | completed | cancelled
  cancel_reason  text,
  departed_at    timestamptz,
  completed_at   timestamptz,
  created_by     uuid REFERENCES users(id),            -- 记录人（当前登录账号）
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_bookings_no ON bookings(booking_no);
CREATE INDEX idx_bookings_tenant_status ON bookings(tenant_id, status, reserve_start DESC);
CREATE INDEX idx_bookings_tenant_source ON bookings(tenant_id, source, created_at DESC);
-- 冲突检查只看未结束的预约
CREATE INDEX idx_bookings_vehicle_window ON bookings(vehicle_id, reserve_start, reserve_end) WHERE status IN ('reserved','departed');
CREATE TRIGGER trg_bookings_updated BEFORE UPDATE ON bookings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS bookings;
