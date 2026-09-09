package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// fakeHooks records what the ingest layer hands over and returns canned results.
type fakeHooks struct {
	startRes  *TripStartResult
	startErr  error
	endID     uuid.UUID
	endErr    error
	gotStart  *TripStartEvent
	gotEnd    *TripEndEvent
	gotDevice Device
}

func (f *fakeHooks) DeviceTripStart(_ context.Context, dev Device, ev TripStartEvent) (*TripStartResult, error) {
	f.gotDevice, f.gotStart = dev, &ev
	return f.startRes, f.startErr
}

func (f *fakeHooks) DeviceTripEnd(_ context.Context, dev Device, ev TripEndEvent) (uuid.UUID, error) {
	f.gotDevice, f.gotEnd = dev, &ev
	return f.endID, f.endErr
}

func (f *fakeHooks) OnTelemetry(context.Context, Device, uuid.UUID, []Point) error { return nil }

func boundDevice() Device {
	vid := uuid.New()
	return Device{ID: uuid.New(), TenantID: uuid.New(), SerialNo: "SIM-0001", VehicleID: &vid, Status: "active"}
}

func statusOf(err error) int {
	var ae *httpx.AppError
	if errors.As(err, &ae) {
		return ae.Status
	}
	if err == nil {
		return 200
	}
	return 500
}

func TestHandleEventTripStartMapsResult(t *testing.T) {
	dev := boundDevice()
	want := &TripStartResult{TripID: uuid.New(), TripNo: "T-20260909-001", TripType: "official", SignOn: true, DriverID: uuid.New()}
	h := &fakeHooks{startRes: want}
	ts := time.Now()
	res, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_start", TripStart: &TripStartEvent{TS: ts, CardUID: " a1b2c3d0 "}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Type != "trip_start" || res.TripID == nil || *res.TripID != want.TripID || *res.TripNo != want.TripNo ||
		*res.TripType != want.TripType || !res.SignOn || *res.DriverID != want.DriverID {
		t.Errorf("mapping mismatch: %+v", res)
	}
	if h.gotStart == nil || h.gotStart.CardUID != "A1B2C3D0" {
		t.Errorf("card uid should be normalized upper-case, got %+v", h.gotStart)
	}
	if h.gotDevice.ID != dev.ID || !h.gotStart.TS.Equal(ts) {
		t.Error("device/event not passed through")
	}
}

func TestHandleEventTripStartPassesAppErrorThrough(t *testing.T) {
	dev := boundDevice()
	h := &fakeHooks{startErr: httpx.Conflict("车辆非空闲")}
	_, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_start", TripStart: &TripStartEvent{TS: time.Now(), CardUID: "ABCDEF12"}})
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Status != 409 || ae.Message != "车辆非空闲" {
		t.Errorf("want the hook's 409 as-is, got %v", err)
	}
	// 501 placeholder from the not-yet-implemented trip module also passes through
	h = &fakeHooks{startErr: &httpx.AppError{Code: httpx.CodeInternal, Status: 501, Message: "not implemented"}}
	_, err = HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_start", TripStart: &TripStartEvent{TS: time.Now(), CardUID: "ABCDEF12"}})
	if statusOf(err) != 501 {
		t.Errorf("want 501, got %v", err)
	}
	// plain errors are not wrapped here (Fail turns them into 500)
	h = &fakeHooks{startErr: errors.New("db down")}
	_, err = HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_start", TripStart: &TripStartEvent{TS: time.Now(), CardUID: "ABCDEF12"}})
	if err == nil || statusOf(err) != 500 {
		t.Errorf("want plain error, got %v", err)
	}
}

func TestHandleEventValidation(t *testing.T) {
	dev := boundDevice()
	h := &fakeHooks{}
	if _, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_start"}); statusOf(err) != 400 {
		t.Errorf("missing trip_start body: %v", err)
	}
	if _, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_start", TripStart: &TripStartEvent{TS: time.Now()}}); statusOf(err) != 400 {
		t.Errorf("neither card_uid nor user_id: %v", err)
	}
	if _, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_end"}); statusOf(err) != 400 {
		t.Errorf("missing trip_end body: %v", err)
	}
	if _, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "bogus", TripEnd: &TripEndEvent{TS: time.Now()}}); statusOf(err) != 400 {
		t.Errorf("unknown type: %v", err)
	}
	unbound := dev
	unbound.VehicleID = nil
	_, err := HandleEvent(context.Background(), h, unbound, EventRequest{Type: "trip_end", TripEnd: &TripEndEvent{TS: time.Now()}})
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Status != 409 || ae.Message != "device not bound to a vehicle" {
		t.Errorf("unbound device: %v", err)
	}
	if h.gotStart != nil || h.gotEnd != nil {
		t.Error("hooks must not be called on validation failures")
	}
}

func TestHandleEventTripEnd(t *testing.T) {
	dev := boundDevice()
	id := uuid.New()
	h := &fakeHooks{endID: id}
	odo := 1234.5
	res, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_end", TripEnd: &TripEndEvent{TS: time.Now(), OdometerKm: &odo}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Type != "trip_end" || res.TripID == nil || *res.TripID != id || res.SignOn || res.TripNo != nil {
		t.Errorf("mapping: %+v", res)
	}
	if h.gotEnd == nil || h.gotEnd.OdometerKm == nil || *h.gotEnd.OdometerKm != odo {
		t.Error("event not passed through")
	}
	h = &fakeHooks{endErr: httpx.NotFound("无进行中的行程")}
	if _, err := HandleEvent(context.Background(), h, dev, EventRequest{Type: "trip_end", TripEnd: &TripEndEvent{TS: time.Now()}}); statusOf(err) != 404 {
		t.Errorf("want 404 passthrough, got %v", err)
	}
}

func TestSortPoints(t *testing.T) {
	t0 := time.Now()
	in := []Point{{TS: t0.Add(2 * time.Second)}, {TS: t0}, {TS: t0.Add(time.Second)}}
	out := SortPoints(in)
	if !out[0].TS.Equal(t0) || !out[1].TS.Equal(t0.Add(time.Second)) || !out[2].TS.Equal(t0.Add(2*time.Second)) {
		t.Errorf("not sorted: %v", out)
	}
	if !in[0].TS.Equal(t0.Add(2 * time.Second)) {
		t.Error("input must not be mutated")
	}
}

func TestWasOnline(t *testing.T) {
	now := time.Now()
	if WasOnline(nil, now) {
		t.Error("never seen → offline")
	}
	recent := now.Add(-4 * time.Minute)
	if !WasOnline(&recent, now) {
		t.Error("4m ago → still online")
	}
	old := now.Add(-5 * time.Minute)
	if WasOnline(&old, now) {
		t.Error("5m ago → offline (transition)")
	}
}
