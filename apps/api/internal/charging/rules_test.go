package charging

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func conns(statuses ...string) []Connector {
	out := make([]Connector, len(statuses))
	for i, s := range statuses {
		out[i] = Connector{ConnectorID: i + 1, Status: s}
	}
	return out
}

func TestAggregatePileStatus(t *testing.T) {
	cases := []struct {
		name string
		in   []Connector
		want string
	}{
		{"no connectors", nil, PileOffline},
		{"all unavailable", conns(ConnUnavailable, ConnUnavailable), PileOffline},
		{"one available", conns(ConnUnavailable, ConnAvailable), PileAvailable},
		{"preparing counts as available", conns(ConnPreparing), PileAvailable},
		{"reserved counts as available", conns(ConnReserved, ConnUnavailable), PileAvailable},
		{"charging beats available", conns(ConnAvailable, ConnCharging), PileCharging},
		{"suspended EV is charging", conns(ConnSuspendedEV, ConnAvailable), PileCharging},
		{"finishing is charging", conns(ConnFinishing), PileCharging},
		{"faulted beats everything", conns(ConnCharging, ConnAvailable, ConnFaulted), PileFaulted},
		{"connector 0 faulted", []Connector{{ConnectorID: 0, Status: ConnFaulted}, {ConnectorID: 1, Status: ConnAvailable}}, PileFaulted},
	}
	for _, c := range cases {
		if got := AggregatePileStatus(c.in); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	if !ValidConnectorStatus("SuspendedEVSE") || ValidConnectorStatus("Busy") {
		t.Errorf("ValidConnectorStatus")
	}
	if !CanRemoteStart(ConnAvailable) || !CanRemoteStart(ConnPreparing) || CanRemoteStart(ConnCharging) || CanRemoteStart(ConnUnavailable) {
		t.Errorf("CanRemoteStart")
	}
}

func fp(v float64) *float64 { return &v }

func TestCompareAndVerdict(t *testing.T) {
	// 20 → 90 % of a 60 kWh pack = 42 kWh; pile says 43.5 kWh → 3.45 % deviation → no review
	cc := Compare(43.5, fp(20), fp(90), 60)
	if cc.BmsKwhEst == nil || *cc.BmsKwhEst != 42 {
		t.Errorf("est = %v", cc.BmsKwhEst)
	}
	if cc.DeviationPct == nil || *cc.DeviationPct != 3.45 {
		t.Errorf("deviation = %v", cc.DeviationPct)
	}
	if needs, _ := NeedsReview(true, cc.DeviationPct); needs {
		t.Errorf("3.45 %% must not need review")
	}
	// pile says 48 kWh → 12.5 % → review
	cc = Compare(48, fp(20), fp(90), 60)
	if cc.DeviationPct == nil || *cc.DeviationPct != 12.5 {
		t.Errorf("deviation = %v", cc.DeviationPct)
	}
	if needs, why := NeedsReview(true, cc.DeviationPct); !needs || why == "" {
		t.Errorf("12.5 %% must need review: %v %q", needs, why)
	}
	// exactly 5 % is fine, 5.01 is not
	if needs, _ := NeedsReview(true, fp(5)); needs {
		t.Errorf("5 %% is the inclusive limit")
	}
	if needs, _ := NeedsReview(true, fp(5.01)); !needs {
		t.Errorf("5.01 %% must need review")
	}
	// SOC went down (BMS glitch) → est clamped to 0
	cc = Compare(10, fp(50), fp(40), 60)
	if cc.BmsKwhEst == nil || *cc.BmsKwhEst != 0 || cc.DeviationPct == nil || *cc.DeviationPct != 100 {
		t.Errorf("clamped est: %+v", cc)
	}
	// missing SOC → no estimate, no deviation → no review on that account
	cc = Compare(10, nil, fp(40), 60)
	if cc.BmsKwhEst != nil || cc.DeviationPct != nil {
		t.Errorf("missing soc: %+v", cc)
	}
	if needs, _ := NeedsReview(true, nil); needs {
		t.Errorf("no deviation known → no review")
	}
	// zero energy → estimate but no deviation
	cc = Compare(0, fp(20), fp(30), 60)
	if cc.BmsKwhEst == nil || *cc.BmsKwhEst != 6 || cc.DeviationPct != nil {
		t.Errorf("zero kwh: %+v", cc)
	}
	// no vehicle → always review
	if needs, why := NeedsReview(false, fp(0)); !needs || why != "未能绑定车辆" {
		t.Errorf("unbound: %v %q", needs, why)
	}
	if SessionKwh(1000, 43500) != 42.5 || SessionKwh(5000, 4000) != 0 {
		t.Errorf("SessionKwh")
	}
}

func TestPickVehiclePriority(t *testing.T) {
	now := time.Now()
	preset, near, far, stale, trip := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	nearby := []NearbyVehicle{
		{VehicleID: far, DistanceM: 150, LastSeen: now.Add(-time.Minute)},
		{VehicleID: near, DistanceM: 12, LastSeen: now.Add(-2 * time.Minute)},
		{VehicleID: stale, DistanceM: 1, LastSeen: now.Add(-10 * time.Minute)}, // too old
		{VehicleID: uuid.New(), DistanceM: 350, LastSeen: now},                 // too far
	}
	// ① preset wins over everything
	v, m := PickVehicle(BindCandidates{Preset: &preset, Nearby: nearby, RecentTrip: &trip, UserKnown: true}, now)
	if v == nil || *v != preset || m != BindManual {
		t.Errorf("preset: %v %s", v, m)
	}
	// ② nearest recent vehicle within 200 m
	v, m = PickVehicle(BindCandidates{Nearby: nearby, RecentTrip: &trip, UserKnown: true}, now)
	if v == nil || *v != near || m != BindLocation {
		t.Errorf("location: %v %s", v, m)
	}
	// ③ recent trip when nothing is parked nearby
	v, m = PickVehicle(BindCandidates{Nearby: nearby[2:], RecentTrip: &trip, UserKnown: true}, now)
	if v == nil || *v != trip || m != BindRecentTrip {
		t.Errorf("recent trip: %v %s", v, m)
	}
	// ④ nothing: card holder known → card, unknown → none
	v, m = PickVehicle(BindCandidates{UserKnown: true}, now)
	if v != nil || m != BindCard {
		t.Errorf("card only: %v %s", v, m)
	}
	v, m = PickVehicle(BindCandidates{}, now)
	if v != nil || m != BindNone {
		t.Errorf("none: %v %s", v, m)
	}
	// nil-uuid preset is ignored
	nilID := uuid.Nil
	v, m = PickVehicle(BindCandidates{Preset: &nilID, RecentTrip: &trip, UserKnown: true}, now)
	if v == nil || *v != trip || m != BindRecentTrip {
		t.Errorf("nil preset: %v %s", v, m)
	}
	// equal distance: the more recently seen one wins
	a, b := uuid.New(), uuid.New()
	v, _ = PickVehicle(BindCandidates{Nearby: []NearbyVehicle{{VehicleID: a, DistanceM: 5, LastSeen: now.Add(-time.Minute)}, {VehicleID: b, DistanceM: 5, LastSeen: now}}}, now)
	if v == nil || *v != b {
		t.Errorf("tie-break by last seen: %v", v)
	}
}

func TestDistanceM(t *testing.T) {
	// ~111 m per 0.001° latitude
	if d := DistanceM(117.0, 36.6, 117.0, 36.601); d < 110 || d > 112 {
		t.Errorf("distance = %.1f", d)
	}
	if DistanceM(117, 36.6, 117, 36.6) != 0 {
		t.Errorf("zero distance")
	}
}

func TestThinAndWithKwh(t *testing.T) {
	pts := make([]MeterValue, 1000)
	base := time.Now()
	for i := range pts {
		wh := 1000 + float64(i)*10
		pts[i] = MeterValue{TS: base.Add(time.Duration(i) * time.Second), Wh: &wh}
	}
	out := Thin(pts, 200)
	if len(out) != 200 {
		t.Fatalf("thinned to %d, want 200", len(out))
	}
	if out[0].TS != pts[0].TS || out[199].TS != pts[999].TS {
		t.Errorf("first/last must be kept")
	}
	for i := 1; i < len(out); i++ {
		if !out[i].TS.After(out[i-1].TS) {
			t.Errorf("order broken at %d", i)
		}
	}
	if got := Thin(pts[:50], 200); len(got) != 50 {
		t.Errorf("short series untouched: %d", len(got))
	}
	if got := Thin(pts, 1); len(got) != 1000 {
		t.Errorf("max<2 returns input: %d", len(got))
	}
	w := WithKwh(pts[:3], 1000)
	if w[0].Kwh == nil || *w[0].Kwh != 0 || w[2].Kwh == nil || *w[2].Kwh != 0.02 {
		t.Errorf("kwh relative to meter_start: %v %v", w[0].Kwh, w[2].Kwh)
	}
	lower := 500.0
	w = WithKwh([]MeterValue{{Wh: &lower}}, 1000)
	if w[0].Kwh == nil || *w[0].Kwh != 0 {
		t.Errorf("negative clamped to 0: %v", w[0].Kwh)
	}
}

func TestPendingVehicle(t *testing.T) {
	pile, veh, user := uuid.New(), uuid.New(), uuid.New()
	if v, _ := TakePendingVehicle(pile, 1); v != nil {
		t.Errorf("nothing pending yet")
	}
	SetPendingVehicle(pile, 1, veh, &user)
	v, u := TakePendingVehicle(pile, 2)
	if v != nil || u != nil {
		t.Errorf("other connector must not consume it")
	}
	v, u = TakePendingVehicle(pile, 1)
	if v == nil || *v != veh || u == nil || *u != user {
		t.Errorf("take: %v %v", v, u)
	}
	if v, _ := TakePendingVehicle(pile, 1); v != nil {
		t.Errorf("consumed once only")
	}
	SetPendingVehicle(pile, 1, veh, nil)
	ClearPendingVehicle(pile, 1)
	if v, _ := TakePendingVehicle(pile, 1); v != nil {
		t.Errorf("cleared")
	}
}

func TestFillConnectors(t *testing.T) {
	tx := &Transaction{ConnectorID: 2}
	out := fillConnectors([]Connector{{ConnectorID: 0, Status: ConnAvailable}, {ConnectorID: 2, Status: ConnCharging}}, 2, map[int]*Transaction{2: tx})
	if len(out) != 3 || out[0].ConnectorID != 0 || out[1].ConnectorID != 1 || out[1].Status != ConnUnavailable || out[2].Transaction != tx {
		t.Errorf("%+v", out)
	}
	out = fillConnectors([]Connector{{ConnectorID: 3, Status: ConnAvailable}}, 1, nil)
	if len(out) != 3 || out[2].Status != ConnAvailable {
		t.Errorf("reported connector beyond archive count is kept: %+v", out)
	}
}

func TestExportRow(t *testing.T) {
	kwh, cost := 12.5, 8.75
	bind := BindLocation
	row := ExportRow(Transaction{TxNo: "C-20260909-001", PileName: "A", PileCode: "P1", ConnectorID: 1, IDTag: "ABCD", BindMethod: &bind,
		StartAt: time.Now(), Kwh: &kwh, Cost: &cost, Status: StatusSettled, ReviewStatus: ReviewNone})
	if len(row) != len(ExportHeaders) {
		t.Fatalf("row has %d cells, header %d", len(row), len(ExportHeaders))
	}
	if row[0] != "C-20260909-001" || row[8] != "位置匹配" || row[12] != "12.500" || row[14] != "8.75" || row[20] != "已结算" || row[21] != "无需复核" {
		t.Errorf("%v", row)
	}
}
