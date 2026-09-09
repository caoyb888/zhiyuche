package trip

import (
	"math"
	"time"
)

// Pure geometry / statistics used by the trip summary and the live rule
// evaluation. No database, unit-tested in geo_test.go.

const (
	earthRadiusM = 6371000.0
	gravity      = 9.80665 // m/s²
)

// Haversine returns the great-circle distance in metres between two lng/lat points.
func Haversine(lng1, lat1, lng2, lat2 float64) float64 {
	toRad := math.Pi / 180
	dLat := (lat2 - lat1) * toRad
	dLng := (lng2 - lng1) * toRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*toRad)*math.Cos(lat2*toRad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(a)))
}

// PathDistanceKm sums the haversine distance between consecutive points.
func PathDistanceKm(pts []TrackPoint) float64 {
	var m float64
	for i := 1; i < len(pts); i++ {
		m += Haversine(pts[i-1].Lng, pts[i-1].Lat, pts[i].Lng, pts[i].Lat)
	}
	return m / 1000
}

// DistanceToPolyline is the shortest distance in metres from (lng, lat) to a
// polyline of [lng, lat] vertices, using a local equirectangular projection
// around the point (accurate to well under 1 % at city scale). A polyline with
// fewer than two valid vertices yields +Inf.
func DistanceToPolyline(lng, lat float64, line [][]float64) float64 {
	kx := 111320 * math.Cos(lat*math.Pi/180) // metres per degree of longitude at this latitude
	ky := 110540.0                           // metres per degree of latitude
	best := math.Inf(1)
	var prev []float64
	for _, v := range line {
		if len(v) < 2 {
			continue
		}
		if prev != nil {
			d := pointSegment((lng-prev[0])*kx, (lat-prev[1])*ky, 0, 0, (v[0]-prev[0])*kx, (v[1]-prev[1])*ky)
			if d < best {
				best = d
			}
		}
		prev = v
	}
	return best
}

// pointSegment: distance from P=(px,py) to segment A=(ax,ay)–B=(bx,by), planar.
func pointSegment(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = ((px-ax)*dx + (py-ay)*dy) / l2
		t = math.Max(0, math.Min(1, t))
	}
	cx, cy := ax+t*dx, ay+t*dy
	return math.Hypot(px-cx, py-cy)
}

// MaxDeviation returns the largest off-route distance of the points (0 when
// the route has fewer than two vertices or there are no points).
func MaxDeviation(pts []TrackPoint, route [][]float64) float64 {
	if countVertices(route) < 2 {
		return 0
	}
	var m float64
	for _, p := range pts {
		if d := DistanceToPolyline(p.Lng, p.Lat, route); d > m {
			m = d
		}
	}
	return m
}

func countVertices(route [][]float64) int {
	n := 0
	for _, v := range route {
		if len(v) >= 2 {
			n++
		}
	}
	return n
}

// HarshCounts counts accelerations above HarshAccelG and decelerations below
// -HarshBrakeG between consecutive points that both carry a speed (km/h);
// the acceleration is Δv / Δt using the point timestamps.
func HarshCounts(pts []TrackPoint) (accel, brake int) {
	var prev *TrackPoint
	for i := range pts {
		p := &pts[i]
		if p.Speed == nil {
			continue
		}
		if prev != nil {
			dt := p.TS.Sub(prev.TS).Seconds()
			if dt > 0 {
				a := (*p.Speed - *prev.Speed) / 3.6 / dt
				switch {
				case a > HarshAccelG*gravity:
					accel++
				case a < -HarshBrakeG*gravity:
					brake++
				}
			}
		}
		prev = p
	}
	return accel, brake
}

// MaxSpeed returns the highest speed among the points (nil when none has one).
func MaxSpeed(pts []TrackPoint) *float64 {
	var m *float64
	for i := range pts {
		if s := pts[i].Speed; s != nil && (m == nil || *s > *m) {
			v := *s
			m = &v
		}
	}
	return m
}

// Thin keeps every step-th point plus the last one (step ≤ 1 keeps all).
func Thin(pts []TrackPoint, step int) []TrackPoint {
	if step <= 1 || len(pts) <= 2 {
		return pts
	}
	out := make([]TrackPoint, 0, len(pts)/step+2)
	for i := 0; i < len(pts); i += step {
		out = append(out, pts[i])
	}
	if last := pts[len(pts)-1]; out[len(out)-1].TS != last.TS {
		out = append(out, last)
	}
	return out
}

// Stats is the summary derived from a finished trip.
type Stats struct {
	DistanceKm     *float64
	EnergyKwh      *float64
	AvgSpeed       *float64
	MaxSpeed       *float64
	HarshAccel     int
	HarshBrake     int
	DeviationMaxM  *float64
	DeviationFlag  bool
	DurationMin    float64
	EnergyPer100Km *float64
}

// Summarize computes the trip statistics. Odometer difference wins over the
// track length when both readings exist and the difference is positive;
// energy = ΔSOC% × battery capacity.
func Summarize(startAt, endAt time.Time, startOdo, endOdo, startSOC, endSOC *float64, batteryKwh float64, pts []TrackPoint, route [][]float64) Stats {
	st := Stats{DurationMin: endAt.Sub(startAt).Minutes()}
	if startOdo != nil && endOdo != nil && *endOdo-*startOdo > 0 {
		d := round1(*endOdo - *startOdo)
		st.DistanceKm = &d
	} else if len(pts) >= 2 {
		d := round1(PathDistanceKm(pts))
		st.DistanceKm = &d
	}
	if startSOC != nil && endSOC != nil && batteryKwh > 0 {
		e := (*startSOC - *endSOC) / 100 * batteryKwh
		if e < 0 {
			e = 0
		}
		e = round2(e)
		st.EnergyKwh = &e
	}
	if st.DistanceKm != nil && st.DurationMin > 0 {
		v := round1(*st.DistanceKm / (st.DurationMin / 60))
		st.AvgSpeed = &v
	}
	st.MaxSpeed = MaxSpeed(pts)
	st.HarshAccel, st.HarshBrake = HarshCounts(pts)
	if countVertices(route) >= 2 && len(pts) > 0 {
		m := round1(MaxDeviation(pts, route))
		st.DeviationMaxM = &m
		st.DeviationFlag = m > DeviationMeters
	}
	if st.DistanceKm != nil && st.EnergyKwh != nil && *st.DistanceKm > 0 {
		v := round2(*st.EnergyKwh / *st.DistanceKm * 100)
		st.EnergyPer100Km = &v
	}
	return st
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }
