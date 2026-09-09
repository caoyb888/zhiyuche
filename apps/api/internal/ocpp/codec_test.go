package ocpp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeCall(t *testing.T) {
	m, err := Decode([]byte(`[2, "19223201", "BootNotification", {"chargePointVendor": "VendorX", "chargePointModel": "SingleSocketCharger"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != Call || m.UniqueID != "19223201" || m.Action != "BootNotification" {
		t.Errorf("decoded %+v", m)
	}
	var req BootNotificationReq
	if err := json.Unmarshal(m.Payload, &req); err != nil || req.ChargePointVendor != "VendorX" || req.ChargePointModel != "SingleSocketCharger" {
		t.Errorf("payload %+v %v", req, err)
	}
}

func TestDecodeResultAndError(t *testing.T) {
	m, err := Decode([]byte(`[3, "19223201", {"status": "Accepted", "currentTime": "2026-09-09T10:00:00Z", "interval": 60}]`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != CallResult || m.UniqueID != "19223201" || !strings.Contains(string(m.Payload), `"Accepted"`) {
		t.Errorf("decoded %+v", m)
	}
	m, err = Decode([]byte(`[4, "abc", "NotImplemented", "no such action", {}]`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != CallError || m.ErrorCode != ErrNotImplemented || m.ErrorDescription != "no such action" {
		t.Errorf("decoded %+v", m)
	}
}

func TestDecodeRejects(t *testing.T) {
	cases := []struct {
		name string
		in   string
		code ErrorCode
		uid  string
	}{
		{"not an array", `{"a":1}`, ErrFormationViolation, ""},
		{"too short", `[2, "x"]`, ErrFormationViolation, ""},
		{"type not int", `["2", "x", "A", {}]`, ErrFormationViolation, ""},
		{"empty uid", `[2, "", "A", {}]`, ErrFormationViolation, ""},
		{"call without payload", `[2, "u1", "Heartbeat"]`, ErrFormationViolation, "u1"},
		{"call payload not object", `[2, "u2", "Heartbeat", []]`, ErrFormationViolation, "u2"},
		{"call empty action", `[2, "u3", "", {}]`, ErrFormationViolation, "u3"},
		{"result wrong length", `[3, "u4", {}, {}]`, ErrFormationViolation, "u4"},
		{"error wrong length", `[4, "u5", "GenericError", "x"]`, ErrFormationViolation, "u5"},
		{"unknown type", `[5, "u6", {}]`, ErrProtocolError, "u6"},
		{"garbage", `not json`, ErrFormationViolation, ""},
	}
	for _, c := range cases {
		_, err := Decode([]byte(c.in))
		var de *DecodeError
		if !errors.As(err, &de) {
			t.Errorf("%s: expected DecodeError, got %v", c.name, err)
			continue
		}
		if de.Code != c.code || de.UniqueID != c.uid {
			t.Errorf("%s: got code=%s uid=%q, want %s %q", c.name, de.Code, de.UniqueID, c.code, c.uid)
		}
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	frame, err := EncodeCall("u1", "RemoteStartTransaction", RemoteStartTransactionReq{IdTag: "ABCD", ConnectorId: intp(1)})
	if err != nil {
		t.Fatal(err)
	}
	if string(frame) != `[2,"u1","RemoteStartTransaction",{"connectorId":1,"idTag":"ABCD"}]` {
		t.Errorf("call frame %s", frame)
	}
	m, err := Decode(frame)
	if err != nil || m.Type != Call || m.Action != "RemoteStartTransaction" {
		t.Errorf("round trip %+v %v", m, err)
	}

	frame, _ = EncodeResult("u2", nil)
	if string(frame) != `[3,"u2",{}]` {
		t.Errorf("empty result frame %s", frame)
	}
	frame, _ = EncodeResult("u3", HeartbeatConf{CurrentTime: "2026-09-09T10:00:00Z"})
	if string(frame) != `[3,"u3",{"currentTime":"2026-09-09T10:00:00Z"}]` {
		t.Errorf("result frame %s", frame)
	}
	frame, _ = EncodeError("u4", ErrNotImplemented, "action X is not implemented", nil)
	if string(frame) != `[4,"u4","NotImplemented","action X is not implemented",{}]` {
		t.Errorf("error frame %s", frame)
	}
	m, err = Decode(frame)
	if err != nil || m.Type != CallError || m.ErrorCode != ErrNotImplemented {
		t.Errorf("error round trip %+v %v", m, err)
	}
	if len(NewUniqueID()) != 36 {
		t.Errorf("unique id must be 36 chars")
	}
}

func TestAsCallErr(t *testing.T) {
	ce := AsCallErr(errors.New("boom"))
	if ce.Code != ErrInternalError || ce.Description != "boom" {
		t.Errorf("%+v", ce)
	}
	orig := &CallErr{Code: ErrFormationViolation, Description: "bad"}
	if AsCallErr(orig) != orig {
		t.Errorf("CallErr must pass through")
	}
}

func intp(v int) *int { return &v }
