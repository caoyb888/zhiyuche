package trip

// Integration tests against a real PostgreSQL (migrations applied). They run
// only when ZY_TEST_DATABASE_URL is set, e.g.
//
//	ZY_TEST_DATABASE_URL=postgres://zhiyuche:zhiyuche@localhost:20432/zhiyuche_be2?sslmode=disable go test ./internal/trip/
//
// Every test seeds its own tenant and removes it afterwards.

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/telemetry"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

type fixture struct {
	ctx       context.Context
	db        *pgxpool.Pool
	svc       *Service
	tenant    uuid.UUID
	dept      uuid.UUID
	applicant uuid.UUID
	approver  uuid.UUID
	other     uuid.UUID
	vehicle   uuid.UUID
	device    telemetry.Device
	card      string
	lostCard  string
	freeCard  string
	approval  uuid.UUID
	applyNo   string
}

var route = [][]float64{{117.00, 36.60}, {117.05, 36.60}, {117.10, 36.60}}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("ZY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ZY_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	a := &app.App{Log: zerolog.Nop(), DB: db, Hub: ws.NewHub(zerolog.Nop())}
	f := &fixture{ctx: ctx, db: db, svc: NewService(a)}
	sfx := fmt.Sprintf("%d%04d", time.Now().Unix()%100000, rand.Intn(10000))

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	one := func(sql string, args ...any) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := db.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
		return id
	}
	f.tenant = one(`INSERT INTO tenants (code, name) VALUES ($1, $2) RETURNING id`, "it_"+sfx, "集成测试租户")
	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM notifications WHERE tenant_id = $1`,
			`DELETE FROM trip_events WHERE tenant_id = $1`,
			`DELETE FROM trip_points WHERE trip_id IN (SELECT id FROM trips WHERE tenant_id = $1)`,
			`DELETE FROM trips WHERE tenant_id = $1`,
			`DELETE FROM approval_steps WHERE approval_id IN (SELECT id FROM approvals WHERE tenant_id = $1)`,
			`DELETE FROM approvals WHERE tenant_id = $1`,
			`DELETE FROM approval_rules WHERE tenant_id = $1`,
			`DELETE FROM nfc_cards WHERE tenant_id = $1`,
			`DELETE FROM devices WHERE tenant_id = $1`,
			`DELETE FROM vehicle_status WHERE tenant_id = $1`,
			`DELETE FROM vehicles WHERE tenant_id = $1`,
			`UPDATE departments SET leader_user_id = NULL WHERE tenant_id = $1`,
			`DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE tenant_id = $1)`,
			`DELETE FROM users WHERE tenant_id = $1`,
			`DELETE FROM departments WHERE tenant_id = $1`,
			`DELETE FROM audit_logs WHERE tenant_id = $1`,
			`DELETE FROM tenants WHERE id = $1`,
		} {
			if _, err := db.Exec(ctx, sql, f.tenant); err != nil {
				t.Logf("cleanup %s: %v", sql, err)
			}
		}
		db.Close()
	})

	f.dept = one(`INSERT INTO departments (tenant_id, name, path) VALUES ($1, '测试部', '/') RETURNING id`, f.tenant)
	exec(`UPDATE departments SET path = '/' || id || '/' WHERE id = $1`, f.dept)
	user := func(name string) uuid.UUID {
		return one(`INSERT INTO users (tenant_id, dept_id, username, password_hash, name) VALUES ($1, $2, $3, 'x', $4) RETURNING id`, f.tenant, f.dept, name+"_"+sfx, name)
	}
	f.applicant, f.approver, f.other = user("driver"), user("leader"), user("other")
	exec(`UPDATE departments SET leader_user_id = $2 WHERE id = $1`, f.dept, f.approver)

	f.vehicle = one(`INSERT INTO vehicles (tenant_id, plate_no, battery_kwh, status, odometer_km) VALUES ($1, $2, 60, 'idle', 1000) RETURNING id`, f.tenant, "测"+sfx[len(sfx)-6:])
	exec(`INSERT INTO vehicle_status (vehicle_id, tenant_id, status, odometer_km, soc, lng, lat, last_telemetry_at) VALUES ($1, $2, 'idle', 1000, 80, 117.0, 36.6, now())`, f.vehicle, f.tenant)
	devID := one(`INSERT INTO devices (tenant_id, serial_no, vehicle_id, api_key_hash) VALUES ($1, $2, $3, 'hash') RETURNING id`, f.tenant, "DEV"+sfx, f.vehicle)
	f.device = telemetry.Device{ID: devID, TenantID: f.tenant, SerialNo: "DEV" + sfx, VehicleID: &f.vehicle, Status: "active"}

	f.card, f.lostCard, f.freeCard = "AA"+sfx, "BB"+sfx, "CC"+sfx
	exec(`INSERT INTO nfc_cards (tenant_id, card_uid, user_id, status) VALUES ($1, $2, $3, 'active'), ($1, $4, $3, 'lost'), ($1, $5, NULL, 'active')`,
		f.tenant, f.card, f.applicant, f.lostCard, f.freeCard)

	f.approval, f.applyNo = f.newApproval(t, f.applicant, "official", time.Now().Add(-time.Hour), time.Now().Add(3*time.Hour))
	return f
}

// newApproval inserts an already-approved request for the fixture vehicle.
func (f *fixture) newApproval(t *testing.T, applicant uuid.UUID, tripType string, start, end time.Time) (uuid.UUID, string) {
	t.Helper()
	tx, err := f.db.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	no, err := approval.NextNo(f.ctx, tx, "approvals", "apply_no", "ZY", time.Now(), 4)
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	err = tx.QueryRow(f.ctx, `
		INSERT INTO approvals (tenant_id, apply_no, applicant_id, dept_id, trip_type, purpose_code, purpose_detail, planned_start, planned_end,
		  destination, planned_route, planned_km, vehicle_id, status, level_required, current_step, approved_at)
		VALUES ($1, $2, $3, $4, $5, 'official_trip', '集成测试出行', $6, $7, '测试目的地',
		  '{"points":[[117.00,36.60],[117.05,36.60],[117.10,36.60]],"distance_km":9}', 9, $8, 'approved', 1, 1, now()) RETURNING id`,
		f.tenant, no, applicant, f.dept, tripType, start, end, f.vehicle).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(f.ctx, `INSERT INTO approval_steps (approval_id, step_no, approver_id, action, acted_at) VALUES ($1, 1, $2, 'approved', now())`, id, f.approver); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	return id, no
}

func (f *fixture) scan(t *testing.T, sql string, dst any, args ...any) {
	t.Helper()
	if err := f.db.QueryRow(f.ctx, sql, args...).Scan(dst); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	f.scan(t, sql, &n, args...)
	return n
}

func status(err error) int {
	var ae *httpx.AppError
	if errors.As(err, &ae) {
		return ae.Status
	}
	return 0
}

func message(err error) string {
	var ae *httpx.AppError
	if errors.As(err, &ae) {
		return ae.Message
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

func pt(ts time.Time, lng, lat, speed, soc float64) telemetry.Point {
	return telemetry.Point{TS: ts, Lng: &lng, Lat: &lat, Speed: &speed, SOC: &soc}
}

func TestDeviceTripLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := f.ctx
	now := time.Now().Truncate(time.Second)

	t.Run("rejections", func(t *testing.T) {
		cases := []struct {
			name   string
			dev    telemetry.Device
			ev     telemetry.TripStartEvent
			status int
			msg    string
		}{
			{"device not bound", telemetry.Device{ID: f.device.ID, TenantID: f.tenant}, telemetry.TripStartEvent{TS: now, CardUID: f.card}, 409, "设备未绑定车辆"},
			{"no driver identity", f.device, telemetry.TripStartEvent{TS: now}, 400, "缺少驾驶员标识"},
			{"unknown card", f.device, telemetry.TripStartEvent{TS: now, CardUID: "ZZ00"}, 403, "卡不存在"},
			{"lost card", f.device, telemetry.TripStartEvent{TS: now, CardUID: f.lostCard}, 403, "卡已挂失"},
			{"unbound card", f.device, telemetry.TripStartEvent{TS: now, CardUID: f.freeCard}, 403, "卡未绑定用户"},
			{"no approval for this driver", f.device, telemetry.TripStartEvent{TS: now, UserID: &f.other}, 403, "无有效用车申请"},
			{"outside the window", f.device, telemetry.TripStartEvent{TS: now.Add(5 * time.Hour), CardUID: f.card}, 403, "无有效用车申请"},
			{"explicit apply_no unknown", f.device, telemetry.TripStartEvent{TS: now, CardUID: f.card, ApprovalNo: "ZY-19700101-0001"}, 403, "申请不存在"},
			{"explicit apply_no wrong driver", f.device, telemetry.TripStartEvent{TS: now, UserID: &f.other, ApprovalNo: f.applyNo}, 403, "驾驶员不是该申请的申请人或同行人"},
		}
		for _, c := range cases {
			res, err := f.svc.StartFromDevice(ctx, c.dev, c.ev)
			if err == nil {
				t.Errorf("%s: expected rejection, got trip %v", c.name, res.TripNo)
				continue
			}
			if status(err) != c.status || !strings.HasPrefix(message(err), c.msg) {
				t.Errorf("%s: got %d %q, want %d %q", c.name, status(err), message(err), c.status, c.msg)
			}
		}
		if n := f.count(t, `SELECT count(*) FROM trips WHERE tenant_id = $1`, f.tenant); n != 0 {
			t.Errorf("rejections must not create trips, found %d", n)
		}
	})

	var tripID uuid.UUID
	t.Run("start by card", func(t *testing.T) {
		odo, soc := 1000.0, 80.0
		res, err := f.svc.StartFromDevice(ctx, f.device, telemetry.TripStartEvent{TS: now, CardUID: f.card, OdometerKm: &odo, SOC: &soc})
		if err != nil {
			t.Fatalf("start: %v", err)
		}
		tripID = res.TripID
		if !res.SignOn || res.TripType != "official" || res.DriverID != f.applicant || res.TripNo == "" {
			t.Errorf("unexpected result %+v", res)
		}
		var vStatus, aStatus, tStatus, source string
		var signOn bool
		var cardID *uuid.UUID
		f.scan(t, `SELECT status FROM vehicles WHERE id = $1`, &vStatus, f.vehicle)
		f.scan(t, `SELECT status FROM approvals WHERE id = $1`, &aStatus, f.approval)
		f.scan(t, `SELECT status FROM trips WHERE id = $1`, &tStatus, tripID)
		f.scan(t, `SELECT source FROM trips WHERE id = $1`, &source, tripID)
		f.scan(t, `SELECT card_id FROM trips WHERE id = $1`, &cardID, tripID)
		f.scan(t, `SELECT sign_on FROM vehicle_status WHERE vehicle_id = $1`, &signOn, f.vehicle)
		if vStatus != "in_use" || aStatus != "in_use" || tStatus != "ongoing" || source != "device" || cardID == nil || !signOn {
			t.Errorf("state after start: vehicle=%s approval=%s trip=%s source=%s card=%v sign_on=%v", vStatus, aStatus, tStatus, source, cardID, signOn)
		}
		var tripRef *uuid.UUID
		f.scan(t, `SELECT current_trip_id FROM vehicle_status WHERE vehicle_id = $1`, &tripRef, f.vehicle)
		if tripRef == nil || *tripRef != tripID {
			t.Errorf("vehicle_status.current_trip_id = %v", tripRef)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type IN ('start','sign_on')`, tripID); n != 2 {
			t.Errorf("start + sign_on events expected, got %d", n)
		}
		if n := f.count(t, `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'trip.started' AND ref_id = $2`, f.applicant, tripID.String()); n != 1 {
			t.Errorf("driver notification expected, got %d", n)
		}
		// vehicle now in use → second swipe is refused
		_, err = f.svc.StartFromDevice(ctx, f.device, telemetry.TripStartEvent{TS: now, CardUID: f.card})
		if status(err) != 409 || message(err) != "车辆已在使用中" {
			t.Errorf("second start: got %d %q", status(err), message(err))
		}
	})

	t.Run("telemetry rules", func(t *testing.T) {
		t0 := now.Add(time.Minute)
		batch := []telemetry.Point{
			pt(t0, 117.00, 36.60, 0, 78),
			pt(t0.Add(10*time.Second), 117.01, 36.60, 40, 77),
			pt(t0.Add(20*time.Second), 117.02, 36.60, 95, 76), // overspeed (limit 80)
			pt(t0.Add(30*time.Second), 117.03, 36.60, 98, 76), // still overspeed → same batch, no second event
			pt(t0.Add(40*time.Second), 117.04, 36.62, 60, 15), // 2.2 km off route + low soc (< 20)
			pt(t0.Add(50*time.Second), 117.05, 36.60, 50, 15),
			{TS: t0.Add(55 * time.Second)}, // no position → not stored
		}
		if err := f.svc.OnTelemetry(ctx, f.tenant, f.vehicle, batch); err != nil {
			t.Fatalf("telemetry: %v", err)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_points WHERE trip_id = $1`, tripID); n != 6 {
			t.Errorf("points stored = %d, want 6", n)
		}
		var pc int
		f.scan(t, `SELECT point_count FROM trips WHERE id = $1`, &pc, tripID)
		if pc != 6 {
			t.Errorf("point_count = %d, want 6", pc)
		}
		for _, typ := range []string{"overspeed", "low_soc", "deviation"} {
			if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = $2`, tripID, typ); n != 1 {
				t.Errorf("%s events = %d, want exactly 1", typ, n)
			}
		}
		var flag bool
		var maxM *float64
		f.scan(t, `SELECT deviation_flag FROM trips WHERE id = $1`, &flag, tripID)
		f.scan(t, `SELECT deviation_max_m FROM trips WHERE id = $1`, &maxM, tripID)
		if !flag || maxM == nil || *maxM < 2000 {
			t.Errorf("deviation flag=%v max=%v", flag, maxM)
		}
		for _, uid := range []uuid.UUID{f.applicant, f.approver} {
			if n := f.count(t, `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'trip.event' AND ref_id = $2`, uid, tripID.String()); n != 3 {
				t.Errorf("user %s should get 3 trip.event notifications, got %d", uid, n)
			}
		}
		// within the 2-minute cooldown: no new overspeed event
		if err := f.svc.OnTelemetry(ctx, f.tenant, f.vehicle, []telemetry.Point{pt(t0.Add(90*time.Second), 117.06, 36.60, 99, 30)}); err != nil {
			t.Fatal(err)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = 'overspeed'`, tripID); n != 1 {
			t.Errorf("cooldown violated: overspeed events = %d", n)
		}
		// after the cooldown: a new one
		if err := f.svc.OnTelemetry(ctx, f.tenant, f.vehicle, []telemetry.Point{pt(t0.Add(4*time.Minute), 117.08, 36.60, 99, 30)}); err != nil {
			t.Fatal(err)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = 'overspeed'`, tripID); n != 2 {
			t.Errorf("after cooldown: overspeed events = %d, want 2", n)
		}
		// roof sign turned off by the gateway → sign_off event once
		off := false
		if err := f.svc.OnTelemetry(ctx, f.tenant, f.vehicle, []telemetry.Point{{TS: t0.Add(5 * time.Minute), Lng: fp(117.09), Lat: fp(36.60), SignOn: &off}}); err != nil {
			t.Fatal(err)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = 'sign_off'`, tripID); n != 1 {
			t.Errorf("sign_off events = %d, want 1", n)
		}
		// telemetry for a vehicle without an ongoing trip is ignored
		other := uuid.New()
		if err := f.svc.OnTelemetry(ctx, f.tenant, other, batch); err != nil {
			t.Errorf("no trip → must be a no-op, got %v", err)
		}
	})

	t.Run("end from device", func(t *testing.T) {
		endAt := now.Add(30 * time.Minute)
		odo, soc := 1012.4, 70.0
		id, err := f.svc.EndFromDevice(ctx, f.device, telemetry.TripEndEvent{TS: endAt, OdometerKm: &odo, SOC: &soc})
		if err != nil {
			t.Fatalf("end: %v", err)
		}
		if id != tripID {
			t.Errorf("ended trip %s, want %s", id, tripID)
		}
		var tr row
		var vStatus, aStatus string
		var signOn bool
		f.scan(t, `SELECT status FROM trips WHERE id = $1`, &tr.Status, tripID)
		f.scan(t, `SELECT distance_km FROM trips WHERE id = $1`, &tr.DistanceKm, tripID)
		f.scan(t, `SELECT energy_kwh FROM trips WHERE id = $1`, &tr.EnergyKwh, tripID)
		f.scan(t, `SELECT max_speed FROM trips WHERE id = $1`, &tr.MaxSpeed, tripID)
		f.scan(t, `SELECT avg_speed FROM trips WHERE id = $1`, &tr.AvgSpeed, tripID)
		f.scan(t, `SELECT harsh_accel FROM trips WHERE id = $1`, &tr.HarshAccel, tripID)
		f.scan(t, `SELECT harsh_brake FROM trips WHERE id = $1`, &tr.HarshBrake, tripID)
		f.scan(t, `SELECT roof_sign_status FROM trips WHERE id = $1`, &tr.RoofSignStatus, tripID)
		f.scan(t, `SELECT end_soc FROM trips WHERE id = $1`, &tr.EndSOC, tripID)
		f.scan(t, `SELECT end_lng FROM trips WHERE id = $1`, &tr.EndLng, tripID)
		f.scan(t, `SELECT status FROM vehicles WHERE id = $1`, &vStatus, f.vehicle)
		f.scan(t, `SELECT status FROM approvals WHERE id = $1`, &aStatus, f.approval)
		f.scan(t, `SELECT sign_on FROM vehicle_status WHERE vehicle_id = $1`, &signOn, f.vehicle)
		if tr.Status != "completed" || vStatus != "idle" || aStatus != "completed" || signOn {
			t.Errorf("state after end: trip=%s vehicle=%s approval=%s sign_on=%v", tr.Status, vStatus, aStatus, signOn)
		}
		if tr.DistanceKm == nil || *tr.DistanceKm != 12.4 {
			t.Errorf("distance = %v, want 12.4 (odometer)", tr.DistanceKm)
		}
		if tr.EnergyKwh == nil || *tr.EnergyKwh != 6 {
			t.Errorf("energy = %v, want 6 kWh", tr.EnergyKwh)
		}
		if tr.MaxSpeed == nil || *tr.MaxSpeed != 99 {
			t.Errorf("max speed = %v, want 99", tr.MaxSpeed)
		}
		if tr.AvgSpeed == nil || *tr.AvgSpeed != 24.8 {
			t.Errorf("avg speed = %v, want 24.8", tr.AvgSpeed)
		}
		// 0→40 in 10 s = 1.1 m/s² (no), 40→95 = 1.5 (no), 98→60 = -1.06 (no): no harsh events in the seeded track
		if tr.HarshAccel != 0 || tr.HarshBrake != 0 {
			t.Errorf("harsh accel/brake = %d/%d, want 0/0", tr.HarshAccel, tr.HarshBrake)
		}
		if tr.RoofSignStatus != "on" {
			t.Errorf("roof sign = %s, want on (sign_on event seen)", tr.RoofSignStatus)
		}
		if tr.EndSOC == nil || *tr.EndSOC != 70 || tr.EndLng == nil {
			t.Errorf("end soc/lng = %v/%v", tr.EndSOC, tr.EndLng)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = 'end'`, tripID); n != 1 {
			t.Errorf("end event = %d", n)
		}
		if n := f.count(t, `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'trip.ended'`, f.applicant); n != 1 {
			t.Errorf("trip.ended notification = %d", n)
		}
		// ending again is refused
		if _, err := f.svc.EndFromDevice(ctx, f.device, telemetry.TripEndEvent{TS: endAt}); status(err) != 409 {
			t.Errorf("second end: got %v", err)
		}
	})

	t.Run("start by user id with explicit apply_no then cancel", func(t *testing.T) {
		apID, no := f.newApproval(t, f.other, "daily", time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)) // window passed → only explicit no works
		res, err := f.svc.StartFromDevice(ctx, f.device, telemetry.TripStartEvent{TS: now, UserID: &f.other, ApprovalNo: no})
		if err != nil {
			t.Fatalf("start: %v", err)
		}
		if res.SignOn || res.TripType != "daily" {
			t.Errorf("daily trip must not light the roof sign: %+v", res)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = 'sign_on'`, res.TripID); n != 0 {
			t.Errorf("daily trip must not record sign_on, got %d", n)
		}
		super := &auth.Principal{UserID: f.approver, TenantID: f.tenant, IsSuper: true}
		tr, err := f.svc.Cancel(ctx, f.tenant, super, res.TripID, nil)
		if err != nil {
			t.Fatalf("cancel: %v", err)
		}
		var vStatus, aStatus string
		f.scan(t, `SELECT status FROM vehicles WHERE id = $1`, &vStatus, f.vehicle)
		f.scan(t, `SELECT status FROM approvals WHERE id = $1`, &aStatus, apID)
		if tr.Status != "cancelled" || vStatus != "idle" || aStatus != "approved" {
			t.Errorf("after cancel: trip=%s vehicle=%s approval=%s", tr.Status, vStatus, aStatus)
		}
		if n := f.count(t, `SELECT count(*) FROM trip_events WHERE trip_id = $1 AND type = 'cancel'`, res.TripID); n != 1 {
			t.Errorf("cancel event = %d", n)
		}
		// approval is approved again → can be started once more via web
		trip, err := f.svc.Start(ctx, f.tenant, super, StartRequest{ApprovalID: apID})
		if err != nil {
			t.Fatalf("web start: %v", err)
		}
		if trip.Source != "web" || trip.Driver == nil || trip.Driver.ID != f.other || trip.StartOdometer == nil {
			t.Errorf("web trip %+v", trip)
		}
		ended, err := f.svc.EndWeb(ctx, f.tenant, super, trip.ID, EndRequest{})
		if err != nil {
			t.Fatalf("web end: %v", err)
		}
		if ended.Status != "completed" || ended.RoofSignStatus != "unknown" || len(ended.Events) < 2 {
			t.Errorf("web end: %+v", ended)
		}
		// daily summary sees three trips today (one cancelled excluded)
		sm, err := f.svc.Summary(ctx, f.tenant, "")
		if err != nil {
			t.Fatal(err)
		}
		if sm.Trips != 2 || sm.Ongoing != 0 || sm.OfficialTrips != 1 || sm.DeviationTrips != 1 || sm.DistanceKm < 12.4 {
			t.Errorf("summary %+v", sm)
		}
		ov, err := f.svc.Overview(ctx, f.tenant, super)
		if err != nil {
			t.Fatal(err)
		}
		if ov.Vehicles.Total != 1 || ov.Vehicles.Idle != 1 || ov.Devices.Total != 1 || len(ov.RecentEvents) != 4 {
			t.Errorf("overview %+v", ov)
		}
	})
}

func fp(v float64) *float64 { return &v }
