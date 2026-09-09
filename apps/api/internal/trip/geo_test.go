package trip

import (
	"math"
	"testing"
	"time"
)

func f(v float64) *float64 { return &v }

func TestHaversine(t *testing.T) {
	// 济南泉城广场 → 西侧约 11.4 km 处
	d := Haversine(117.0245, 36.6647, 116.8985, 36.6817)
	if d < 11000 || d > 12000 {
		t.Errorf("haversine = %.0f m, want ≈ 11.4 km", d)
	}
	if Haversine(117, 36, 117, 36) != 0 {
		t.Error("same point should be 0")
	}
	// one degree of latitude ≈ 111.2 km
	if d := Haversine(0, 0, 0, 1); math.Abs(d-111195) > 200 {
		t.Errorf("1° lat = %.0f m", d)
	}
}

func TestDistanceToPolyline(t *testing.T) {
	route := [][]float64{{117.00, 36.60}, {117.10, 36.60}, {117.10, 36.70}}
	// on the first segment
	if d := DistanceToPolyline(117.05, 36.60, route); d > 1 {
		t.Errorf("on-route point distance = %.1f m, want 0", d)
	}
	// 0.01° north of the first segment ≈ 1105 m
	if d := DistanceToPolyline(117.05, 36.61, route); math.Abs(d-1105) > 30 {
		t.Errorf("1105 m expected, got %.1f", d)
	}
	// beyond the end vertex: distance to the vertex itself
	if d := DistanceToPolyline(117.10, 36.71, route); math.Abs(d-1105) > 30 {
		t.Errorf("distance to end vertex expected ≈ 1105, got %.1f", d)
	}
	// closest to the second segment
	if d := DistanceToPolyline(117.11, 36.65, route); d > 950 || d < 850 {
		t.Errorf("0.01° east of the vertical segment ≈ 893 m, got %.1f", d)
	}
	if !math.IsInf(DistanceToPolyline(117, 36, [][]float64{{117, 36}}), 1) {
		t.Error("single-vertex route should be +Inf")
	}
	if !math.IsInf(DistanceToPolyline(117, 36, [][]float64{{117}, {118}}), 1) {
		t.Error("malformed vertices should be ignored")
	}
}

func TestMaxDeviation(t *testing.T) {
	route := [][]float64{{117.00, 36.60}, {117.10, 36.60}}
	pts := []TrackPoint{{Lng: 117.02, Lat: 36.60}, {Lng: 117.05, Lat: 36.605}, {Lng: 117.08, Lat: 36.62}}
	m := MaxDeviation(pts, route)
	if m < 2150 || m > 2300 {
		t.Errorf("max deviation ≈ 2211 m, got %.1f", m)
	}
	if MaxDeviation(pts, nil) != 0 || MaxDeviation(nil, route) != 0 {
		t.Error("no route / no points → 0")
	}
}

func TestHarshCounts(t *testing.T) {
	t0 := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	p := func(sec int, kmh float64) TrackPoint {
		return TrackPoint{TS: t0.Add(time.Duration(sec) * time.Second), Speed: f(kmh)}
	}
	pts := []TrackPoint{
		p(0, 0),
		p(2, 20),                       // +2.78 m/s² > 0.25g (2.45) → harsh accel
		p(4, 25),                       // +0.69 → normal
		p(6, 0),                        // -3.47 m/s² < -0.35g (-3.43) → harsh brake
		p(8, 10),                       // +1.39 → normal
		p(10, 4),                       // -0.83 → normal
		p(10, 40),                      // dt = 0 → ignored
		{TS: t0.Add(12 * time.Second)}, // no speed → skipped, does not break the chain
		p(14, 60),                      // vs 40 @10s: +1.39 → normal
	}
	a, b := HarshCounts(pts)
	if a != 1 || b != 1 {
		t.Errorf("harsh accel/brake = %d/%d, want 1/1", a, b)
	}
	if a, b := HarshCounts(nil); a != 0 || b != 0 {
		t.Error("empty → 0/0")
	}
}

func TestThin(t *testing.T) {
	t0 := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	pts := make([]TrackPoint, 11)
	for i := range pts {
		pts[i] = TrackPoint{TS: t0.Add(time.Duration(i) * time.Second)}
	}
	got := Thin(pts, 5)
	if len(got) != 3 || got[0].TS != pts[0].TS || got[1].TS != pts[5].TS || got[2].TS != pts[10].TS {
		t.Errorf("step 5 over 11 points: got %d points", len(got))
	}
	got = Thin(pts, 4)
	if len(got) != 4 || got[3].TS != pts[10].TS {
		t.Errorf("step 4 must keep the last point: got %d", len(got))
	}
	if len(Thin(pts, 1)) != 11 || len(Thin(pts, 0)) != 11 {
		t.Error("step ≤ 1 keeps everything")
	}
	if len(Thin(pts[:2], 10)) != 2 {
		t.Error("two points are never thinned")
	}
}

func TestSummarize(t *testing.T) {
	t0 := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	pts := []TrackPoint{
		{TS: t0, Lng: 117.00, Lat: 36.60, Speed: f(0)},
		{TS: t0.Add(10 * time.Minute), Lng: 117.05, Lat: 36.60, Speed: f(60)},
		{TS: t0.Add(20 * time.Minute), Lng: 117.10, Lat: 36.60, Speed: f(90)},
	}
	route := [][]float64{{117.00, 36.60}, {117.10, 36.60}}
	// odometer wins
	st := Summarize(t0, t0.Add(30*time.Minute), f(1000), f(1012.4), f(80), f(70), 60, pts, route)
	if st.DistanceKm == nil || *st.DistanceKm != 12.4 {
		t.Errorf("distance from odometer = %v", st.DistanceKm)
	}
	if st.EnergyKwh == nil || *st.EnergyKwh != 6 {
		t.Errorf("energy = %v, want 6 kWh", st.EnergyKwh)
	}
	if st.AvgSpeed == nil || *st.AvgSpeed != 24.8 {
		t.Errorf("avg speed = %v, want 24.8", st.AvgSpeed)
	}
	if st.MaxSpeed == nil || *st.MaxSpeed != 90 {
		t.Errorf("max speed = %v", st.MaxSpeed)
	}
	if st.EnergyPer100Km == nil || math.Abs(*st.EnergyPer100Km-48.39) > 0.01 {
		t.Errorf("energy/100km = %v", st.EnergyPer100Km)
	}
	if st.DeviationFlag || st.DeviationMaxM == nil || *st.DeviationMaxM != 0 {
		t.Errorf("on-route trip must not deviate: %+v", st)
	}
	if st.DurationMin != 30 {
		t.Errorf("duration = %v", st.DurationMin)
	}
	// no odometer → path length (≈ 8.9 km), energy clamps at 0 when SOC rose
	st = Summarize(t0, t0.Add(30*time.Minute), nil, nil, f(70), f(75), 60, pts, nil)
	if st.DistanceKm == nil || *st.DistanceKm < 8.8 || *st.DistanceKm > 9.0 {
		t.Errorf("path distance = %v, want ≈ 8.9", st.DistanceKm)
	}
	if st.EnergyKwh == nil || *st.EnergyKwh != 0 {
		t.Errorf("energy must clamp to 0, got %v", st.EnergyKwh)
	}
	if st.DeviationMaxM != nil {
		t.Error("no route → no deviation figure")
	}
	// odometer going backwards is ignored in favour of the track
	st = Summarize(t0, t0.Add(30*time.Minute), f(1000), f(999), nil, nil, 60, pts, nil)
	if st.DistanceKm == nil || *st.DistanceKm < 8.8 {
		t.Errorf("negative odometer delta must fall back to the track: %v", st.DistanceKm)
	}
	if st.EnergyKwh != nil || st.EnergyPer100Km != nil {
		t.Error("no SOC → no energy")
	}
	// off-route point raises the flag
	off := append(pts, TrackPoint{TS: t0.Add(25 * time.Minute), Lng: 117.10, Lat: 36.62})
	st = Summarize(t0, t0.Add(30*time.Minute), nil, nil, nil, nil, 60, off, route)
	if !st.DeviationFlag || st.DeviationMaxM == nil || *st.DeviationMaxM < 2000 {
		t.Errorf("2.2 km off route must flag: %+v", st)
	}
}
