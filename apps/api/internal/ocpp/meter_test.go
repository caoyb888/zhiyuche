package ocpp

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseMeterValuesNormalisesUnits(t *testing.T) {
	raw := `[
	  {"timestamp":"2026-09-09T10:00:00Z","sampledValue":[
	    {"value":"12.345","measurand":"Energy.Active.Import.Register","unit":"kWh"},
	    {"value":"230.5","measurand":"Voltage","unit":"V","phase":"L1"},
	    {"value":"228.0","measurand":"Voltage","unit":"V"},
	    {"value":"32.1","measurand":"Current.Import","unit":"A"},
	    {"value":"7360","measurand":"Power.Active.Import","unit":"W"},
	    {"value":"55","measurand":"SoC","unit":"Percent"},
	    {"value":"deadbeef","measurand":"Energy.Active.Import.Register","format":"SignedData"}
	  ]},
	  {"timestamp":"2026-09-09T10:00:30Z","sampledValue":[
	    {"value":"12400"},
	    {"value":"7.5","measurand":"Power.Active.Import","unit":"kW"},
	    {"value":"not-a-number","measurand":"Voltage"}
	  ]},
	  {"timestamp":"garbage","sampledValue":[{"value":"12500","unit":"Wh"}]}
	]`
	var entries []MeterValueEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 9, 10, 1, 0, 0, time.UTC)
	got := ParseMeterValues(entries, now)
	if len(got) != 3 {
		t.Fatalf("samples = %d, want 3", len(got))
	}
	s := got[0]
	if s.TS != time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC) {
		t.Errorf("ts %v", s.TS)
	}
	if s.Wh == nil || *s.Wh != 12345 {
		t.Errorf("kWh → Wh: %v", s.Wh)
	}
	if s.Voltage == nil || *s.Voltage != 228.0 {
		t.Errorf("aggregate voltage must win over phase L1: %v", s.Voltage)
	}
	if s.Current == nil || *s.Current != 32.1 {
		t.Errorf("current %v", s.Current)
	}
	if s.PowerKw == nil || *s.PowerKw != 7.36 {
		t.Errorf("W → kW: %v", s.PowerKw)
	}
	if s.SOC == nil || *s.SOC != 55 {
		t.Errorf("soc %v", s.SOC)
	}
	if len(s.Raw) == 0 {
		t.Errorf("raw must be kept")
	}

	s = got[1]
	if s.Wh == nil || *s.Wh != 12400 {
		t.Errorf("measurand default = energy, unit default = Wh: %v", s.Wh)
	}
	if s.PowerKw == nil || *s.PowerKw != 7.5 {
		t.Errorf("kW stays kW: %v", s.PowerKw)
	}
	if s.Voltage != nil {
		t.Errorf("unparsable value must be dropped: %v", *s.Voltage)
	}

	s = got[2]
	if s.TS != now {
		t.Errorf("bad timestamp must fall back to now, got %v", s.TS)
	}
	if s.Wh == nil || *s.Wh != 12500 {
		t.Errorf("Wh %v", s.Wh)
	}
}

func TestParseMeterValuesPhaseOnly(t *testing.T) {
	entries := []MeterValueEntry{{Timestamp: "2026-09-09T10:00:00Z", SampledValue: []SampledValue{
		{Value: "220", Measurand: "Voltage", Phase: "L1"},
		{Value: "221", Measurand: "Voltage", Phase: "L2"},
	}}}
	got := ParseMeterValues(entries, time.Now())
	if got[0].Voltage == nil || *got[0].Voltage != 220 {
		t.Errorf("first phase value kept when no aggregate: %v", got[0].Voltage)
	}
}

func TestParseTime(t *testing.T) {
	for _, s := range []string{"2026-09-09T10:00:00Z", "2026-09-09T10:00:00.123Z", "2026-09-09T18:00:00+08:00", "2026-09-09T10:00:00", "2026-09-09T10:00:00.000"} {
		if _, ok := ParseTime(s); !ok {
			t.Errorf("%s should parse", s)
		}
	}
	if _, ok := ParseTime(""); ok {
		t.Errorf("empty must fail")
	}
	if FormatTime(time.Date(2026, 9, 9, 18, 0, 0, 0, time.FixedZone("CST", 8*3600))) != "2026-09-09T10:00:00Z" {
		t.Errorf("FormatTime must render UTC RFC3339")
	}
}
