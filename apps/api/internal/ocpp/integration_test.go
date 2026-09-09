package ocpp

// Integration test against a real PostgreSQL (migrations applied). Runs only
// when ZY_TEST_DATABASE_URL is set, e.g.
//
//	ZY_TEST_DATABASE_URL=postgres://zhiyuche:zhiyuche@localhost:20432/zhiyuche_be2?sslmode=disable go test ./internal/ocpp/
//
// It starts the OCPP server on an ephemeral port, connects a gorilla client
// as the pile and walks Boot → Status → Authorize → Start → Meter → Stop,
// checking the rows the charging module writes and the verdicts it reaches.
// The billing hook is a fake. The test seeds its own tenant and removes it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

// ---- fake billing

type fakeBilling struct {
	mu    sync.Mutex
	fail  error
	calls []charging.BillingInput
}

func (f *fakeBilling) PriceAndCharge(_ context.Context, in charging.BillingInput) (*charging.BillingResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	if f.fail != nil {
		return nil, f.fail
	}
	return &charging.BillingResult{UnitPrice: 1.2, Cost: math.Round(in.Kwh*1.2*100) / 100, Attribution: "employee", AccountID: uuid.New(), AccountTxnID: 42}, nil
}

func (f *fakeBilling) setFail(err error) {
	f.mu.Lock()
	f.fail = err
	f.mu.Unlock()
}

// ---- pile client

type pileClient struct {
	t       *testing.T
	ws      *websocket.Conn
	mu      sync.Mutex
	wmu     sync.Mutex // gorilla conns allow one concurrent writer
	pending map[string]chan *Message
	calls   chan *Message // CALLs the server sends us
	closed  chan error
}

func (c *pileClient) write(frame []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.ws.WriteMessage(websocket.TextMessage, frame)
}

func dialPile(t *testing.T, addr, code string, subprotocol bool) (*pileClient, *http.Response, error) {
	t.Helper()
	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	if subprotocol {
		d.Subprotocols = []string{Subprotocol}
	}
	conn, resp, err := d.Dial("ws://"+addr+"/ocpp/"+code, nil)
	if err != nil {
		return nil, resp, err
	}
	c := &pileClient{t: t, ws: conn, pending: map[string]chan *Message{}, calls: make(chan *Message, 8), closed: make(chan error, 1)}
	go c.readLoop()
	return c, resp, nil
}

func (c *pileClient) readLoop() {
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			c.closed <- err
			return
		}
		m, err := Decode(data)
		if err != nil {
			c.t.Logf("client: bad frame %s: %v", data, err)
			continue
		}
		switch m.Type {
		case Call:
			c.calls <- m
		default:
			c.mu.Lock()
			ch := c.pending[m.UniqueID]
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		}
	}
}

// call sends a CALL and returns the CALLRESULT payload, or the CALLERROR message.
func (c *pileClient) call(action string, payload any) (json.RawMessage, *Message) {
	c.t.Helper()
	uid := NewUniqueID()
	frame, err := EncodeCall(uid, action, payload)
	if err != nil {
		c.t.Fatal(err)
	}
	ch := make(chan *Message, 1)
	c.mu.Lock()
	c.pending[uid] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, uid)
		c.mu.Unlock()
	}()
	if err := c.write(frame); err != nil {
		c.t.Fatalf("write %s: %v", action, err)
	}
	select {
	case m := <-ch:
		if m.Type == CallError {
			return nil, m
		}
		return m.Payload, nil
	case <-time.After(10 * time.Second):
		c.t.Fatalf("%s: no answer within 10s", action)
	}
	return nil, nil
}

// must performs a call that has to succeed and decodes the answer into dst.
func (c *pileClient) must(action string, payload, dst any) {
	c.t.Helper()
	raw, cerr := c.call(action, payload)
	if cerr != nil {
		c.t.Fatalf("%s → CALLERROR %s: %s", action, cerr.ErrorCode, cerr.ErrorDescription)
	}
	if dst != nil {
		if err := json.Unmarshal(raw, dst); err != nil {
			c.t.Fatalf("%s: decode %s: %v", action, raw, err)
		}
	}
}

func (c *pileClient) reply(uid string, payload any) {
	frame, _ := EncodeResult(uid, payload)
	_ = c.write(frame)
}

func (c *pileClient) close() { _ = c.ws.Close() }

// ---- fixture

type fixture struct {
	ctx      context.Context
	db       *pgxpool.Pool
	app      *app.App
	billing  *fakeBilling
	srv      *Server
	addr     string
	svc      *charging.Service
	tenant   uuid.UUID
	dept     uuid.UUID
	driver   uuid.UUID
	admin    uuid.UUID
	vehicle  uuid.UUID // parked at the pile
	vehicle2 uuid.UUID // 1 km away
	vehicle3 uuid.UUID // 50 m away but stale telemetry
	pile     uuid.UUID
	pileCode string
	card     string
	lostCard string
	freeCard string
}

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
	f := &fixture{ctx: ctx, db: db, app: a, billing: &fakeBilling{}}
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
	f.tenant = one(`INSERT INTO tenants (code, name) VALUES ($1, $2) RETURNING id`, "ocpp_"+sfx, "OCPP 集成测试租户")
	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM charge_meter_values WHERE pile_id IN (SELECT id FROM charge_piles WHERE tenant_id = $1)`,
			`DELETE FROM charge_transactions WHERE tenant_id = $1`,
			`DELETE FROM pile_connectors WHERE pile_id IN (SELECT id FROM charge_piles WHERE tenant_id = $1)`,
			`DELETE FROM charge_piles WHERE tenant_id = $1`,
			`DELETE FROM notifications WHERE tenant_id = $1`,
			`DELETE FROM trips WHERE tenant_id = $1`,
			`DELETE FROM nfc_cards WHERE tenant_id = $1`,
			`DELETE FROM vehicle_telemetry WHERE tenant_id = $1`,
			`DELETE FROM vehicle_status WHERE tenant_id = $1`,
			`DELETE FROM vehicles WHERE tenant_id = $1`,
			`DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE tenant_id = $1)`,
			`DELETE FROM roles WHERE tenant_id = $1`,
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
	f.dept = one(`INSERT INTO departments (tenant_id, name, path) VALUES ($1, '充电测试部', '/') RETURNING id`, f.tenant)
	exec(`UPDATE departments SET path = '/' || id || '/' WHERE id = $1`, f.dept)
	f.driver = one(`INSERT INTO users (tenant_id, dept_id, username, password_hash, name) VALUES ($1, $2, $3, 'x', '司机') RETURNING id`, f.tenant, f.dept, "drv_"+sfx)
	f.admin = one(`INSERT INTO users (tenant_id, dept_id, username, password_hash, name) VALUES ($1, $2, $3, 'x', '管理员') RETURNING id`, f.tenant, f.dept, "adm_"+sfx)
	role := one(`INSERT INTO roles (tenant_id, code, name, is_system) VALUES ($1, 'tenant_admin', '租户管理员', true) RETURNING id`, f.tenant)
	exec(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, f.admin, role)

	f.card, f.lostCard, f.freeCard = "aa"+sfx, "BB"+sfx, "CC"+sfx // lower-case on purpose: matching is case-insensitive
	exec(`INSERT INTO nfc_cards (tenant_id, card_uid, user_id, status) VALUES ($1, upper($2), $3, 'active'), ($1, $4, $3, 'lost'), ($1, $5, NULL, 'active')`,
		f.tenant, f.card, f.driver, f.lostCard, f.freeCard)

	const lng, lat = 117.05, 36.65
	veh := func(plate string, dlat float64, seen string, soc float64) uuid.UUID {
		id := one(`INSERT INTO vehicles (tenant_id, plate_no, battery_kwh, status, soc) VALUES ($1, $2, 60, 'idle', $3) RETURNING id`, f.tenant, plate, soc)
		exec(`INSERT INTO vehicle_status (vehicle_id, tenant_id, status, soc, lng, lat, last_telemetry_at) VALUES ($1, $2, 'idle', $3, $4, $5, now() - $6::interval)`, id, f.tenant, soc, lng, lat+dlat, seen)
		return id
	}
	f.vehicle = veh("充A"+sfx[len(sfx)-5:], 0.0001, "10 seconds", 20)  // ~11 m away
	f.vehicle2 = veh("充B"+sfx[len(sfx)-5:], 0.009, "10 seconds", 50)   // ~1 km away
	f.vehicle3 = veh("充C"+sfx[len(sfx)-5:], 0.0004, "10 minutes", 50) // ~45 m away but stale

	f.pileCode = "IT-PILE-" + sfx
	f.pile = one(`INSERT INTO charge_piles (tenant_id, pile_code, name, type, power_kw, connector_count, lng, lat, status) VALUES ($1, $2, '集成测试桩', 'fast', 60, 2, $3, $4, 'offline') RETURNING id`,
		f.tenant, f.pileCode, lng, lat)
	exec(`INSERT INTO charge_piles (tenant_id, pile_code, name, status) VALUES ($1, $2, '停用桩', 'disabled')`, f.tenant, f.pileCode+"-DISABLED")

	// server on an ephemeral port
	f.srv = New(a, f.billing)
	f.svc = charging.NewService(a, f.billing)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.addr = ln.Addr().String()
	sctx, cancel := context.WithCancel(ctx)
	go func() { _ = f.srv.Serve(sctx, ln) }()
	t.Cleanup(func() { cancel(); time.Sleep(100 * time.Millisecond) })
	return f
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

func (f *fixture) str(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var s *string
	f.scan(t, sql, &s, args...)
	if s == nil {
		return "<null>"
	}
	return *s
}

func (f *fixture) setSOC(t *testing.T, vehicle uuid.UUID, soc float64) {
	t.Helper()
	if _, err := f.db.Exec(f.ctx, `UPDATE vehicle_status SET soc = $2, last_telemetry_at = now() WHERE vehicle_id = $1`, vehicle, soc); err != nil {
		t.Fatal(err)
	}
}

// eventually polls cond for up to 5 s (side effects of a disconnect run asynchronously).
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

var txNoRe = regexp.MustCompile(`^C-\d{8}-\d{3,}$`)

func TestOCPPSessionLifecycle(t *testing.T) {
	f := newFixture(t)
	now := time.Now()

	t.Run("unknown or disabled pile is closed with 1008", func(t *testing.T) {
		for _, code := range []string{"NO-SUCH-PILE", f.pileCode + "-DISABLED"} {
			c, _, err := dialPile(t, f.addr, code, true)
			if err != nil {
				t.Fatalf("dial %s: %v", code, err)
			}
			select {
			case err := <-c.closed:
				var ce *websocket.CloseError
				if !errors.As(err, &ce) || ce.Code != websocket.ClosePolicyViolation {
					t.Errorf("%s: want close 1008, got %v", code, err)
				}
			case <-time.After(5 * time.Second):
				t.Errorf("%s: connection was not closed", code)
			}
			c.close()
		}
	})

	c, resp, err := dialPile(t, f.addr, f.pileCode, true)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.close()
	if got := resp.Header.Get("Sec-WebSocket-Protocol"); got != Subprotocol {
		t.Errorf("subprotocol = %q", got)
	}
	eventually(t, "registry to know the pile", func() bool { return f.srv.Online(f.pile) })

	t.Run("boot and heartbeat", func(t *testing.T) {
		var conf BootNotificationConf
		c.must("BootNotification", BootNotificationReq{ChargePointVendor: "TestVendor", ChargePointModel: "T1"}, &conf)
		if conf.Status != "Accepted" || conf.Interval != int(charging.DefaultHeartbeat.Seconds()) {
			t.Errorf("boot conf %+v", conf)
		}
		if _, ok := ParseTime(conf.CurrentTime); !ok {
			t.Errorf("currentTime %q", conf.CurrentTime)
		}
		if st := f.str(t, `SELECT status FROM charge_piles WHERE id = $1`, f.pile); st != "available" {
			t.Errorf("pile status after boot = %s", st)
		}
		if v := f.str(t, `SELECT vendor FROM charge_piles WHERE id = $1`, f.pile); v != "TestVendor" {
			t.Errorf("vendor = %s", v)
		}
		if n := f.count(t, `SELECT count(*) FROM charge_piles WHERE id = $1 AND last_heartbeat_at > now() - interval '10 seconds'`, f.pile); n != 1 {
			t.Errorf("last_heartbeat_at not stamped")
		}
		var hb HeartbeatConf
		c.must("Heartbeat", struct{}{}, &hb)
		if _, ok := ParseTime(hb.CurrentTime); !ok {
			t.Errorf("heartbeat currentTime %q", hb.CurrentTime)
		}
	})

	t.Run("status notifications aggregate the pile status", func(t *testing.T) {
		c.must("StatusNotification", StatusNotificationReq{ConnectorId: 1, ErrorCode: "NoError", Status: "Available"}, nil)
		c.must("StatusNotification", StatusNotificationReq{ConnectorId: 2, ErrorCode: "NoError", Status: "Unavailable"}, nil)
		if n := f.count(t, `SELECT count(*) FROM pile_connectors WHERE pile_id = $1`, f.pile); n != 2 {
			t.Errorf("connectors = %d", n)
		}
		if st := f.str(t, `SELECT status FROM charge_piles WHERE id = $1`, f.pile); st != "available" {
			t.Errorf("pile = %s, want available", st)
		}
		c.must("StatusNotification", StatusNotificationReq{ConnectorId: 2, ErrorCode: "GroundFailure", Status: "Faulted", Info: strp("relay")}, nil)
		if st := f.str(t, `SELECT status FROM charge_piles WHERE id = $1`, f.pile); st != "faulted" {
			t.Errorf("pile = %s, want faulted", st)
		}
		if ec := f.str(t, `SELECT error_code FROM pile_connectors WHERE pile_id = $1 AND connector_id = 2`, f.pile); ec != "GroundFailure" {
			t.Errorf("error_code = %s", ec)
		}
		c.must("StatusNotification", StatusNotificationReq{ConnectorId: 2, ErrorCode: "NoError", Status: "Available"}, nil)
		if st := f.str(t, `SELECT status FROM charge_piles WHERE id = $1`, f.pile); st != "available" {
			t.Errorf("pile = %s, want available", st)
		}
		if _, cerr := c.call("StatusNotification", StatusNotificationReq{ConnectorId: 1, Status: "Busy"}); cerr == nil || cerr.ErrorCode != ErrPropertyConstraintViolation {
			t.Errorf("bad status must be PropertyConstraintViolation: %+v", cerr)
		}
	})

	t.Run("authorize and error frames", func(t *testing.T) {
		var conf AuthorizeConf
		c.must("Authorize", AuthorizeReq{IdTag: strings.ToLower(f.card)}, &conf)
		if conf.IdTagInfo.Status != "Accepted" {
			t.Errorf("active card → %s", conf.IdTagInfo.Status)
		}
		c.must("Authorize", AuthorizeReq{IdTag: f.lostCard}, &conf)
		if conf.IdTagInfo.Status != "Blocked" {
			t.Errorf("lost card → %s", conf.IdTagInfo.Status)
		}
		c.must("Authorize", AuthorizeReq{IdTag: f.freeCard}, &conf)
		if conf.IdTagInfo.Status != "Invalid" {
			t.Errorf("unbound card → %s", conf.IdTagInfo.Status)
		}
		c.must("Authorize", AuthorizeReq{IdTag: "ZZZZ"}, &conf)
		if conf.IdTagInfo.Status != "Invalid" {
			t.Errorf("unknown card → %s", conf.IdTagInfo.Status)
		}
		if _, cerr := c.call("FirmwareStatusNotification", map[string]any{"status": "Idle"}); cerr == nil || cerr.ErrorCode != ErrNotImplemented {
			t.Errorf("unknown action must be NotImplemented: %+v", cerr)
		}
		if _, cerr := c.call("StatusNotification", map[string]any{"connectorId": "one", "status": "Available"}); cerr == nil || cerr.ErrorCode != ErrFormationViolation {
			t.Errorf("bad payload must be FormationViolation: %+v", cerr)
		}
		var dt DataTransferConf
		c.must("DataTransfer", DataTransferReq{VendorId: "acme"}, &dt)
		if dt.Status != "UnknownVendorId" {
			t.Errorf("DataTransfer → %s", dt.Status)
		}
		// a malformed frame with a readable UniqueId gets a CALLERROR back
		ch := make(chan *Message, 1)
		c.mu.Lock()
		c.pending["bad-1"] = ch
		c.mu.Unlock()
		_ = c.write([]byte(`[2,"bad-1","Heartbeat"]`))
		select {
		case m := <-ch:
			if m.Type != CallError || m.ErrorCode != ErrFormationViolation {
				t.Errorf("malformed frame answer %+v", m)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("no CALLERROR for a malformed frame")
		}
	})

	var tx1 uuid.UUID
	var ocpp1 int
	t.Run("start transaction binds the nearest parked vehicle", func(t *testing.T) {
		var conf StartTransactionConf
		c.must("StartTransaction", StartTransactionReq{ConnectorId: 1, IdTag: strings.ToLower(f.card), MeterStart: 1000, Timestamp: FormatTime(now)}, &conf)
		if conf.IdTagInfo.Status != "Accepted" || conf.TransactionId < 1000 {
			t.Fatalf("conf %+v", conf)
		}
		ocpp1 = conf.TransactionId
		var txNo, status, bind, review string
		var user, dept, vehicle, card *uuid.UUID
		var soc *float64
		f.scan(t, `SELECT id FROM charge_transactions WHERE pile_id = $1 AND ocpp_tx_id = $2`, &tx1, f.pile, ocpp1)
		f.scan(t, `SELECT tx_no FROM charge_transactions WHERE id = $1`, &txNo, tx1)
		f.scan(t, `SELECT status FROM charge_transactions WHERE id = $1`, &status, tx1)
		f.scan(t, `SELECT bind_method FROM charge_transactions WHERE id = $1`, &bind, tx1)
		f.scan(t, `SELECT review_status FROM charge_transactions WHERE id = $1`, &review, tx1)
		f.scan(t, `SELECT user_id FROM charge_transactions WHERE id = $1`, &user, tx1)
		f.scan(t, `SELECT dept_id FROM charge_transactions WHERE id = $1`, &dept, tx1)
		f.scan(t, `SELECT vehicle_id FROM charge_transactions WHERE id = $1`, &vehicle, tx1)
		f.scan(t, `SELECT card_id FROM charge_transactions WHERE id = $1`, &card, tx1)
		f.scan(t, `SELECT bms_soc_start FROM charge_transactions WHERE id = $1`, &soc, tx1)
		if !txNoRe.MatchString(txNo) {
			t.Errorf("tx_no = %s", txNo)
		}
		if status != "charging" || review != "none" || bind != "location" {
			t.Errorf("status=%s review=%s bind=%s", status, review, bind)
		}
		if user == nil || *user != f.driver || dept == nil || *dept != f.dept || card == nil {
			t.Errorf("attribution user=%v dept=%v card=%v", user, dept, card)
		}
		if vehicle == nil || *vehicle != f.vehicle {
			t.Errorf("vehicle = %v, want the one parked 11 m away (%s)", vehicle, f.vehicle)
		}
		if soc == nil || *soc != 20 {
			t.Errorf("bms_soc_start = %v", soc)
		}
		if st := f.str(t, `SELECT status FROM vehicle_status WHERE vehicle_id = $1`, f.vehicle); st != "charging" {
			t.Errorf("vehicle_status.status = %s", st)
		}
		if st := f.str(t, `SELECT status FROM vehicles WHERE id = $1`, f.vehicle); st != "charging" {
			t.Errorf("vehicles.status = %s", st)
		}
		if n := f.count(t, `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'charging.started' AND ref_id = $2`, f.driver, tx1.String()); n != 1 {
			t.Errorf("charging.started notification = %d", n)
		}
		c.must("StatusNotification", StatusNotificationReq{ConnectorId: 1, ErrorCode: "NoError", Status: "Charging"}, nil)
		if st := f.str(t, `SELECT status FROM charge_piles WHERE id = $1`, f.pile); st != "charging" {
			t.Errorf("pile = %s, want charging", st)
		}
	})

	t.Run("meter values are normalised and stored", func(t *testing.T) {
		mv := func(ts time.Time, kwh, w float64, soc float64) MeterValueEntry {
			return MeterValueEntry{Timestamp: FormatTime(ts), SampledValue: []SampledValue{
				{Value: fmt.Sprintf("%.3f", kwh), Measurand: MeasurandEnergy, Unit: "kWh"},
				{Value: fmt.Sprintf("%.0f", w), Measurand: MeasurandPower, Unit: "W"},
				{Value: "380", Measurand: MeasurandVoltage, Unit: "V"},
				{Value: fmt.Sprintf("%.1f", w/380), Measurand: MeasurandCurrent, Unit: "A"},
				{Value: fmt.Sprintf("%.0f", soc), Measurand: MeasurandSoC, Unit: "Percent"},
			}}
		}
		id := ocpp1
		c.must("MeterValues", MeterValuesReq{ConnectorId: 1, TransactionId: &id, MeterValue: []MeterValueEntry{
			mv(now.Add(30*time.Second), 6, 60000, 28), mv(now.Add(60*time.Second), 11, 60000, 37),
		}}, nil)
		c.must("MeterValues", MeterValuesReq{ConnectorId: 1, MeterValue: []MeterValueEntry{mv(now.Add(90*time.Second), 16, 59000, 45)}}, nil) // no transactionId → connector's session
		if n := f.count(t, `SELECT count(*) FROM charge_meter_values WHERE tx_id = $1`, tx1); n != 3 {
			t.Errorf("meter rows = %d, want 3", n)
		}
		var wh, kw float64
		f.scan(t, `SELECT wh FROM charge_meter_values WHERE tx_id = $1 ORDER BY ts DESC LIMIT 1`, &wh, tx1)
		f.scan(t, `SELECT power_kw FROM charge_meter_values WHERE tx_id = $1 ORDER BY ts DESC LIMIT 1`, &kw, tx1)
		if wh != 16000 || kw != 59 {
			t.Errorf("last sample wh=%v kw=%v", wh, kw)
		}
		// live view: kwh relative to meter_start, connector carries the session
		piles, err := f.svc.ListLive(f.ctx, f.tenant)
		if err != nil {
			t.Fatal(err)
		}
		var live *charging.PileLive
		for i := range piles {
			if piles[i].ID == f.pile {
				live = &piles[i]
			}
		}
		if live == nil || !live.Online || live.Status != "charging" || len(live.Connectors) != 2 {
			t.Fatalf("live %+v", live)
		}
		tr := live.Connectors[0].Transaction
		if tr == nil || tr.ID != tx1 || tr.Kwh == nil || *tr.Kwh != 15 || tr.PowerKw == nil || *tr.PowerKw != 59 {
			t.Errorf("connector 1 transaction %+v", tr)
		}
		if live.Connectors[1].Transaction != nil {
			t.Errorf("connector 2 must be free")
		}
		det, err := f.svc.Get(f.ctx, f.tenant, tx1)
		if err != nil || len(det.MeterValues) != 3 || det.MeterValues[2].Kwh == nil || *det.MeterValues[2].Kwh != 15 {
			t.Errorf("detail %+v %v", det, err)
		}
	})

	t.Run("stop within tolerance settles through billing", func(t *testing.T) {
		f.setSOC(t, f.vehicle, 90) // BMS: 20 → 90 % of 60 kWh = 42 kWh
		c.must("StopTransaction", StopTransactionReq{TransactionId: ocpp1, MeterStop: 44000, Timestamp: FormatTime(now.Add(40 * time.Minute)), Reason: "Local", IdTag: strp(f.card)}, nil)
		var status, review string
		var kwh, est, dev, cost *float64
		f.scan(t, `SELECT status FROM charge_transactions WHERE id = $1`, &status, tx1)
		f.scan(t, `SELECT review_status FROM charge_transactions WHERE id = $1`, &review, tx1)
		f.scan(t, `SELECT kwh FROM charge_transactions WHERE id = $1`, &kwh, tx1)
		f.scan(t, `SELECT bms_kwh_est FROM charge_transactions WHERE id = $1`, &est, tx1)
		f.scan(t, `SELECT deviation_pct FROM charge_transactions WHERE id = $1`, &dev, tx1)
		f.scan(t, `SELECT cost FROM charge_transactions WHERE id = $1`, &cost, tx1)
		if kwh == nil || *kwh != 43 || est == nil || *est != 42 || dev == nil || *dev != 2.33 {
			t.Errorf("kwh=%v est=%v dev=%v", kwh, est, dev)
		}
		if status != "settled" || review != "none" || cost == nil || *cost != 51.6 {
			t.Errorf("status=%s review=%s cost=%v", status, review, cost)
		}
		if r := f.str(t, `SELECT stop_reason FROM charge_transactions WHERE id = $1`, tx1); r != "Local" {
			t.Errorf("stop_reason = %s", r)
		}
		if st := f.str(t, `SELECT status FROM vehicle_status WHERE vehicle_id = $1`, f.vehicle); st != "idle" {
			t.Errorf("vehicle back to %s, want idle", st)
		}
		if n := f.count(t, `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'charging.ended' AND ref_id = $2 AND content LIKE '%费用 ¥51.60%'`, f.driver, tx1.String()); n != 1 {
			t.Errorf("charging.ended notification with cost = %d", n)
		}
		f.billing.mu.Lock()
		calls := len(f.billing.calls)
		f.billing.mu.Unlock()
		if calls != 1 {
			t.Errorf("billing calls = %d", calls)
		}
		// stopping again is harmless
		c.must("StopTransaction", StopTransactionReq{TransactionId: ocpp1, MeterStop: 44000, Timestamp: FormatTime(now), Reason: "Local"}, nil)
		c.must("StopTransaction", StopTransactionReq{TransactionId: 1, MeterStop: 0, Timestamp: FormatTime(now), Reason: "Local"}, nil)
	})

	t.Run("deviation above 5 percent goes to review, approve bills", func(t *testing.T) {
		f.setSOC(t, f.vehicle, 20)
		var conf StartTransactionConf
		c.must("StartTransaction", StartTransactionReq{ConnectorId: 1, IdTag: f.card, MeterStart: 44000, Timestamp: FormatTime(now)}, &conf)
		f.setSOC(t, f.vehicle, 90)
		c.must("StopTransaction", StopTransactionReq{TransactionId: conf.TransactionId, MeterStop: 92000, Timestamp: FormatTime(now.Add(time.Hour)), Reason: "EVDisconnected"}, nil) // 48 kWh vs 42 → 12.5 %
		var tx uuid.UUID
		f.scan(t, `SELECT id FROM charge_transactions WHERE pile_id = $1 AND ocpp_tx_id = $2`, &tx, f.pile, conf.TransactionId)
		var status, review, note string
		var dev *float64
		f.scan(t, `SELECT status FROM charge_transactions WHERE id = $1`, &status, tx)
		f.scan(t, `SELECT review_status FROM charge_transactions WHERE id = $1`, &review, tx)
		f.scan(t, `SELECT review_note FROM charge_transactions WHERE id = $1`, &note, tx)
		f.scan(t, `SELECT deviation_pct FROM charge_transactions WHERE id = $1`, &dev, tx)
		if status != "ended" || review != "pending" || !strings.Contains(note, "偏差") || dev == nil || *dev != 12.5 {
			t.Errorf("status=%s review=%s note=%q dev=%v", status, review, note, dev)
		}
		if n := f.count(t, `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'charging.review' AND ref_id = $2`, f.admin, tx.String()); n != 1 {
			t.Errorf("tenant admin review notification = %d", n)
		}
		actor := &auth.Principal{UserID: f.admin, TenantID: f.tenant}
		// approve moving the session to vehicle2 (BMS re-read from telemetry: none → no estimate)
		res, err := f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "approve", VehicleID: &f.vehicle2, Note: strp("确认为 B 车")})
		if err != nil {
			t.Fatalf("approve: %v", err)
		}
		if res.Status != "settled" || res.ReviewStatus != "approved" || res.Vehicle == nil || res.Vehicle.ID != f.vehicle2 || res.BindMethod == nil || *res.BindMethod != "manual" {
			t.Errorf("after approve %+v", res)
		}
		if res.Cost == nil || *res.Cost != 57.6 || res.ReviewedByName == nil || *res.ReviewedByName != "管理员" || res.ReviewNote == nil || *res.ReviewNote != "确认为 B 车" {
			t.Errorf("cost=%v reviewer=%v note=%v", res.Cost, res.ReviewedByName, res.ReviewNote)
		}
		if res.BmsKwhEst != nil {
			t.Errorf("no telemetry for vehicle2 → estimate must be nil, got %v", *res.BmsKwhEst)
		}
		if _, err := f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "approve"}); err == nil {
			t.Errorf("second review must be refused")
		}
	})

	t.Run("no vehicle → pending, reject leaves it unbilled", func(t *testing.T) {
		// move every vehicle away and use an unknown card: nothing to bind, nobody to attribute
		if _, err := f.db.Exec(f.ctx, `UPDATE vehicle_status SET lat = lat + 0.05 WHERE tenant_id = $1`, f.tenant); err != nil {
			t.Fatal(err)
		}
		var conf StartTransactionConf
		c.must("StartTransaction", StartTransactionReq{ConnectorId: 2, IdTag: "UNKNOWN1", MeterStart: 0, Timestamp: FormatTime(now)}, &conf)
		if conf.IdTagInfo.Status != "Invalid" {
			t.Errorf("unknown tag status %s", conf.IdTagInfo.Status)
		}
		var tx uuid.UUID
		f.scan(t, `SELECT id FROM charge_transactions WHERE pile_id = $1 AND ocpp_tx_id = $2`, &tx, f.pile, conf.TransactionId)
		if b := f.str(t, `SELECT bind_method FROM charge_transactions WHERE id = $1`, tx); b != "none" {
			t.Errorf("bind_method = %s", b)
		}
		if r := f.str(t, `SELECT review_status FROM charge_transactions WHERE id = $1`, tx); r != "pending" {
			t.Errorf("review at start = %s", r)
		}
		actor := &auth.Principal{UserID: f.admin, TenantID: f.tenant}
		if _, err := f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "reject"}); err == nil {
			t.Errorf("review of a charging transaction must be refused")
		}
		c.must("StopTransaction", StopTransactionReq{TransactionId: conf.TransactionId, MeterStop: 5000, Timestamp: FormatTime(now.Add(10 * time.Minute)), Reason: "DeAuthorized"}, nil)
		res, err := f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "reject", Note: strp("无法确认")})
		if err != nil {
			t.Fatalf("reject: %v", err)
		}
		if res.Status != "ended" || res.ReviewStatus != "rejected" || res.Cost != nil || res.Kwh == nil || *res.Kwh != 5 {
			t.Errorf("after reject %+v", res)
		}
		// approve without user or vehicle is refused; with a user it bills to that user
		c.must("StartTransaction", StartTransactionReq{ConnectorId: 2, IdTag: "UNKNOWN2", MeterStart: 5000, Timestamp: FormatTime(now)}, &conf)
		c.must("StopTransaction", StopTransactionReq{TransactionId: conf.TransactionId, MeterStop: 6000, Timestamp: FormatTime(now.Add(10 * time.Minute)), Reason: "Local"}, nil)
		f.scan(t, `SELECT id FROM charge_transactions WHERE pile_id = $1 AND ocpp_tx_id = $2`, &tx, f.pile, conf.TransactionId)
		if _, err := f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "approve"}); err == nil {
			t.Errorf("approve without attribution must be refused")
		}
		res, err = f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "approve", UserID: &f.driver})
		if err != nil {
			t.Fatalf("approve with user: %v", err)
		}
		if res.Status != "settled" || res.User == nil || res.User.ID != f.driver || res.DeptID == nil || *res.DeptID != f.dept || res.BindMethod == nil || *res.BindMethod != "card" {
			t.Errorf("after approve with user %+v", res)
		}
	})

	t.Run("billing failure keeps the session ended and pending", func(t *testing.T) {
		f.billing.setFail(errors.New("not implemented"))
		defer f.billing.setFail(nil)
		if _, err := f.db.Exec(f.ctx, `UPDATE vehicle_status SET lat = lat - 0.05, soc = 20, last_telemetry_at = now() WHERE tenant_id = $1`, f.tenant); err != nil {
			t.Fatal(err)
		}
		var conf StartTransactionConf
		c.must("StartTransaction", StartTransactionReq{ConnectorId: 1, IdTag: f.card, MeterStart: 92000, Timestamp: FormatTime(now)}, &conf)
		f.setSOC(t, f.vehicle, 90)
		c.must("StopTransaction", StopTransactionReq{TransactionId: conf.TransactionId, MeterStop: 134000, Timestamp: FormatTime(now.Add(time.Hour)), Reason: "Remote"}, nil)
		var tx uuid.UUID
		f.scan(t, `SELECT id FROM charge_transactions WHERE pile_id = $1 AND ocpp_tx_id = $2`, &tx, f.pile, conf.TransactionId)
		st, rv, note := f.str(t, `SELECT status FROM charge_transactions WHERE id = $1`, tx), f.str(t, `SELECT review_status FROM charge_transactions WHERE id = $1`, tx), f.str(t, `SELECT review_note FROM charge_transactions WHERE id = $1`, tx)
		if st != "ended" || rv != "pending" || !strings.HasPrefix(note, "计费失败") {
			t.Errorf("status=%s review=%s note=%q", st, rv, note)
		}
		if c := f.str(t, `SELECT cost::text FROM charge_transactions WHERE id = $1`, tx); c != "<null>" {
			t.Errorf("cost must stay empty, got %s", c)
		}
		// approve retries billing: still failing → stays pending with the error
		actor := &auth.Principal{UserID: f.admin, TenantID: f.tenant}
		if _, err := f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "approve"}); err == nil {
			t.Errorf("approve must surface the billing failure")
		}
		if rv := f.str(t, `SELECT review_status FROM charge_transactions WHERE id = $1`, tx); rv != "pending" {
			t.Errorf("review after failed approve = %s", rv)
		}
		f.billing.setFail(nil)
		res, err := f.svc.Review(f.ctx, f.tenant, actor, tx, charging.ReviewRequest{Action: "approve"})
		if err != nil {
			t.Fatalf("approve retry: %v", err)
		}
		if res.Status != "settled" || res.ReviewStatus != "approved" || res.Cost == nil || *res.Cost != 50.4 {
			t.Errorf("after retry %+v", res)
		}
	})

	t.Run("remote start with a preset vehicle, remote stop, superseded session", func(t *testing.T) {
		// the pile answers the server's CALLs
		go func() {
			for m := range c.calls {
				switch m.Action {
				case "RemoteStartTransaction", "RemoteStopTransaction":
					c.reply(m.UniqueID, StatusConf{Status: "Accepted"})
				case "Reset", "ChangeAvailability":
					c.reply(m.UniqueID, StatusConf{Status: "Rejected"})
				case "GetConfiguration":
					c.reply(m.UniqueID, GetConfigurationConf{ConfigurationKey: []KeyValue{{Key: "HeartbeatInterval", Value: strp("60")}}})
				default:
					frame, _ := EncodeError(m.UniqueID, ErrNotImplemented, "", nil)
					_ = c.write(frame)
				}
			}
		}()
		actor := &auth.Principal{UserID: f.driver, TenantID: f.tenant}
		// connector 1 still reports Charging from the earlier session → refused up front
		if _, err := f.svc.RemoteStart(f.ctx, f.tenant, actor, f.pile, charging.RemoteStartRequest{}); err == nil {
			t.Errorf("remote start on a charging connector must be 409")
		}
		c.must("StatusNotification", StatusNotificationReq{ConnectorId: 1, ErrorCode: "NoError", Status: "Available"}, nil)
		res, err := f.svc.RemoteStart(f.ctx, f.tenant, actor, f.pile, charging.RemoteStartRequest{VehicleID: &f.vehicle2})
		if err != nil {
			t.Fatalf("remote start: %v", err)
		}
		if res.Status != "Accepted" {
			t.Errorf("remote start → %s", res.Status)
		}
		// the pile now starts: the preset vehicle wins over the one parked next to the pile
		var conf StartTransactionConf
		c.must("StartTransaction", StartTransactionReq{ConnectorId: 1, IdTag: f.card, MeterStart: 134000, Timestamp: FormatTime(now)}, &conf)
		var tx uuid.UUID
		f.scan(t, `SELECT id FROM charge_transactions WHERE pile_id = $1 AND ocpp_tx_id = $2`, &tx, f.pile, conf.TransactionId)
		var vehicle *uuid.UUID
		f.scan(t, `SELECT vehicle_id FROM charge_transactions WHERE id = $1`, &vehicle, tx)
		if vehicle == nil || *vehicle != f.vehicle2 || f.str(t, `SELECT bind_method FROM charge_transactions WHERE id = $1`, tx) != "manual" {
			t.Errorf("preset vehicle not applied: %v", vehicle)
		}
		stop, err := f.svc.RemoteStop(f.ctx, f.tenant, f.pile, charging.RemoteStopRequest{TransactionID: tx})
		if err != nil || stop.Status != "Accepted" {
			t.Errorf("remote stop: %+v %v", stop, err)
		}
		if st, err := f.srv.Reset(f.ctx, f.pile, "Soft"); err != nil || st != "Rejected" {
			t.Errorf("reset: %s %v", st, err)
		}
		if cfg, err := f.srv.GetConfiguration(f.ctx, f.pile, nil); err != nil || len(cfg.ConfigurationKey) != 1 {
			t.Errorf("get configuration: %+v %v", cfg, err)
		}
		// a second StartTransaction on the busy connector supersedes the first one
		c.must("StartTransaction", StartTransactionReq{ConnectorId: 1, IdTag: f.card, MeterStart: 135000, Timestamp: FormatTime(now.Add(time.Minute))}, &conf)
		if st := f.str(t, `SELECT status FROM charge_transactions WHERE id = $1`, tx); st == "charging" {
			t.Errorf("superseded transaction must be closed, still %s", st)
		}
		if n := f.count(t, `SELECT count(*) FROM charge_transactions WHERE pile_id = $1 AND connector_id = 1 AND status = 'charging'`, f.pile); n != 1 {
			t.Errorf("active on connector 1 = %d", n)
		}
		// summary sees the sessions
		sm, err := f.svc.Summary(f.ctx, f.tenant, "")
		if err != nil {
			t.Fatal(err)
		}
		if sm.Today.Sessions < 6 || sm.Ongoing != 1 || sm.Piles.Total != 2 || sm.Piles.Online != 1 || len(sm.ByPile) != 2 {
			t.Errorf("summary %+v", sm)
		}
		if sm.Today.Kwh < 100 || sm.Today.Cost < 150 {
			t.Errorf("summary totals %+v", sm.Today)
		}
		page, err := f.svc.List(f.ctx, f.tenant, charging.ListQuery{ReviewStatus: "rejected"}, paginationAll())
		if err != nil || page.Total != 1 {
			t.Errorf("list rejected: %+v %v", page.Total, err)
		}
	})

	t.Run("disconnect marks the pile offline", func(t *testing.T) {
		c.close()
		eventually(t, "pile offline", func() bool {
			return f.str(t, `SELECT status FROM charge_piles WHERE id = $1`, f.pile) == "offline" && !f.srv.Online(f.pile)
		})
		if n := f.count(t, `SELECT count(*) FROM pile_connectors WHERE pile_id = $1 AND status <> 'Unavailable'`, f.pile); n != 0 {
			t.Errorf("connectors not marked Unavailable: %d", n)
		}
		actor := &auth.Principal{UserID: f.driver, TenantID: f.tenant}
		if _, err := f.svc.RemoteStart(f.ctx, f.tenant, actor, f.pile, charging.RemoteStartRequest{}); err == nil {
			t.Errorf("remote start on an offline pile must be refused")
		}
		// reconnect without the subprotocol still works (warned), and a fresh connection replaces the old one
		c2, resp, err := dialPile(t, f.addr, f.pileCode, false)
		if err != nil {
			t.Fatalf("redial: %v", err)
		}
		defer c2.close()
		if resp.Header.Get("Sec-WebSocket-Protocol") != "" {
			t.Errorf("no subprotocol requested → none negotiated")
		}
		var hb HeartbeatConf
		c2.must("Heartbeat", struct{}{}, &hb)
		eventually(t, "pile online again", func() bool { return f.srv.Online(f.pile) })
		c3, _, err := dialPile(t, f.addr, f.pileCode, true)
		if err != nil {
			t.Fatal(err)
		}
		defer c3.close()
		select {
		case <-c2.closed: // replaced
		case <-time.After(5 * time.Second):
			t.Errorf("old connection must be dropped when the pile reconnects")
		}
		if !f.srv.Online(f.pile) {
			t.Errorf("pile must stay online through the replacement")
		}
	})
}

func strp(s string) *string { return &s }

func paginationAll() pagination.Query { return pagination.Query{Page: 1, PageSize: 200} }
