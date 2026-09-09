package main

import (
	"math"
	"math/rand"
	"testing"
)

var jinan = LngLat{117.12, 36.65}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestParseLngLat(t *testing.T) {
	p, err := parseLngLat("117.12,36.65")
	if err != nil || p != jinan {
		t.Fatalf("got %v %v", p, err)
	}
	if _, err := parseLngLat(" 117.12 , 36.65 "); err != nil {
		t.Error("spaces should be tolerated")
	}
	for _, bad := range []string{"", "117.12", "abc,36", "200,36", "117,95"} {
		if _, err := parseLngLat(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestHaversineAndBearing(t *testing.T) {
	north := LngLat{jinan.Lng, jinan.Lat + 1}
	if d := haversineKm(jinan, north); !near(d, 111.19, 0.3) {
		t.Errorf("1° of latitude: got %.2f km", d)
	}
	if b := bearingDeg(jinan, north); !near(b, 0, 0.01) {
		t.Errorf("north bearing: %.2f", b)
	}
	east := LngLat{jinan.Lng + 0.1, jinan.Lat}
	if b := bearingDeg(jinan, east); !near(b, 90, 0.1) {
		t.Errorf("east bearing: %.2f", b)
	}
	if d := haversineKm(jinan, jinan); d != 0 {
		t.Errorf("zero distance: %v", d)
	}
}

func TestDestinationRoundTrip(t *testing.T) {
	for _, br := range []float64{0, 45, 90, 180, 270, 333} {
		p := destination(jinan, br, 5)
		if d := haversineKm(jinan, p); !near(d, 5, 0.001) {
			t.Errorf("bearing %v: distance %.4f", br, d)
		}
		if b := bearingDeg(jinan, p); !near(b, br, 0.05) {
			t.Errorf("bearing %v: got %.3f", br, b)
		}
	}
}

func TestRandomPointWithin(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 1000; i++ {
		p := randomPointWithin(rng, jinan, 8)
		if d := haversineKm(jinan, p); d > 8.0001 {
			t.Fatalf("point %v is %.3f km away", p, d)
		}
	}
}

func TestRouteInterpolation(t *testing.T) {
	a := jinan
	b := LngLat{jinan.Lng, jinan.Lat + 0.1} // ≈ 11.12 km due north
	c := LngLat{jinan.Lng + 0.1, jinan.Lat + 0.1}
	r := newRoute([]LngLat{a, b, c})
	total := r.TotalKm()
	if !near(total, haversineKm(a, b)+haversineKm(b, c), 1e-9) {
		t.Errorf("total %.3f", total)
	}
	p, h, done := r.At(0)
	if p != a || done || !near(h, 0, 0.01) {
		t.Errorf("start: %v %v %v", p, h, done)
	}
	half := haversineKm(a, b) / 2
	p, h, done = r.At(half)
	if done || !near(p.Lat, a.Lat+0.05, 1e-6) || !near(p.Lng, a.Lng, 1e-9) || !near(h, 0, 0.01) {
		t.Errorf("midpoint of first leg: %v heading %.2f done %v", p, h, done)
	}
	p, h, done = r.At(haversineKm(a, b) + 0.001)
	if done || !near(h, 90, 0.2) || !near(p.Lat, b.Lat, 1e-6) {
		t.Errorf("just into second leg: %v heading %.2f done %v", p, h, done)
	}
	p, _, done = r.At(total)
	if !done || p != c {
		t.Errorf("end: %v done %v", p, done)
	}
	p, _, done = r.At(total + 5)
	if !done || p != c {
		t.Errorf("past end: %v done %v", p, done)
	}
	if _, _, done := newRoute(nil).At(1); !done {
		t.Error("empty route is always done")
	}
}

func TestBuildRoute(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	dest := destination(jinan, 60, 6)
	r := buildRoute(rng, jinan, dest, 4, 0.8)
	if len(r.pts) != 6 || r.pts[0] != jinan || r.End() != dest {
		t.Errorf("shape: %d points, start %v end %v", len(r.pts), r.pts[0], r.End())
	}
	straight := haversineKm(jinan, dest)
	if r.TotalKm() < straight || r.TotalKm() > straight*1.6 {
		t.Errorf("route %.2f km vs straight %.2f km", r.TotalKm(), straight)
	}
}

func TestEnergy(t *testing.T) {
	if d := socDropPct(100, 60); !near(d, 26.6667, 0.001) {
		t.Errorf("100 km on 60 kWh: %.4f%%", d)
	}
	if d := socDropPct(0, 60); d != 0 {
		t.Errorf("zero distance: %v", d)
	}
	if g := chargeGainPct(60, 0.5, 60); !near(g, 50, 1e-9) {
		t.Errorf("30 min at 60 kW: %.2f%%", g)
	}
	if socDropPct(10, 0) != 0 || chargeGainPct(7, 1, 0) != 0 {
		t.Error("zero battery must not divide by zero")
	}
}
