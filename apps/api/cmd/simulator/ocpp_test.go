package main

import (
	"log/slog"
	"testing"
)

func TestDeriveOCPP(t *testing.T) {
	cases := map[string]string{
		"http://localhost:20080/api/v1":   "ws://localhost:20081",
		"http://100.95.76.81:20091/api/v1": "ws://100.95.76.81:20081",
		"https://zy.example.com/api/v1":   "ws://zy.example.com:20081",
		"not a url":                        "ws://localhost:20081",
	}
	for in, want := range cases {
		if got := deriveOCPP(in); got != want {
			t.Errorf("deriveOCPP(%q) = %q, want %q", in, got, want)
		}
	}
	cfg, err := parseFlags([]string{"--api", "http://10.0.0.5:20091/api/v1"})
	if err != nil || cfg.OCPP != "ws://10.0.0.5:20081" {
		t.Errorf("default --ocpp from --api: %q %v", cfg.OCPP, err)
	}
	cfg, err = parseFlags([]string{"--ocpp", "ws://localhost:20181/", "--low-soc", "2"})
	if err != nil || cfg.OCPP != "ws://localhost:20181" || cfg.LowSOC != 2 {
		t.Errorf("explicit --ocpp: %+v %v", cfg, err)
	}
	if _, err := parseFlags([]string{"--ocpp", "http://x"}); err == nil {
		t.Errorf("--ocpp must be ws(s)://")
	}
	if _, err := parseFlags([]string{"--low-soc", "99"}); err == nil {
		t.Errorf("--low-soc above --vehicles must fail")
	}
}

func TestSimPileConnectors(t *testing.T) {
	p := newSimPile(&PileAsset{Code: "P", PowerKw: 30, ConnectorCount: 2, MeterWh: []float64{100, 200}}, slog.Default())
	if p.powerKw != 30 || len(p.connectors) != 2 || p.connectors[1].meterWh != 200 {
		t.Fatalf("%+v", p)
	}
	if p.online() {
		t.Errorf("no client → offline")
	}
	c := p.freeConnector()
	if c == nil || c.id != 1 {
		t.Fatalf("free connector %+v", c)
	}
	c.vehicle, c.txID = &simVehicle{}, 1234
	if p.freeConnector() == nil || p.freeConnector().id != 2 {
		t.Errorf("second connector must be free")
	}
	if p.connectorByTx(1234) != c || p.connectorByTx(1) != nil {
		t.Errorf("connectorByTx")
	}
	p.persist()
	if len(p.asset.MeterWh) != 2 || p.asset.MeterWh[0] != 100 {
		t.Errorf("persist %v", p.asset.MeterWh)
	}
	d := newSimPile(&PileAsset{Code: "D"}, slog.Default())
	if d.powerKw != defaultPileKw || len(d.connectors) != defaultConnCount {
		t.Errorf("defaults %+v", d)
	}
}
