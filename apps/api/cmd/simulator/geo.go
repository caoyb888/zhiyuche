package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
)

// LngLat is a WGS-84 position.
type LngLat struct {
	Lng float64 `json:"lng"`
	Lat float64 `json:"lat"`
}

const earthRadiusKm = 6371.0

// consumptionKwhPer100km is the assumed energy use of the simulated e6.
const consumptionKwhPer100km = 16.0

func toRad(d float64) float64 { return d * math.Pi / 180 }
func toDeg(r float64) float64 { return r * 180 / math.Pi }

// parseLngLat parses "lng,lat".
func parseLngLat(s string) (LngLat, error) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 2 {
		return LngLat{}, fmt.Errorf("want lng,lat got %q", s)
	}
	lng, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || lng < -180 || lng > 180 || lat < -90 || lat > 90 {
		return LngLat{}, fmt.Errorf("invalid lng,lat %q", s)
	}
	return LngLat{lng, lat}, nil
}

// haversineKm is the great-circle distance.
func haversineKm(a, b LngLat) float64 {
	dLat := toRad(b.Lat - a.Lat)
	dLng := toRad(b.Lng - a.Lng)
	la1, la2 := toRad(a.Lat), toRad(b.Lat)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(h)))
}

// bearingDeg is the initial heading from a to b, 0–360 clockwise from north.
func bearingDeg(a, b LngLat) float64 {
	la1, la2 := toRad(a.Lat), toRad(b.Lat)
	dLng := toRad(b.Lng - a.Lng)
	y := math.Sin(dLng) * math.Cos(la2)
	x := math.Cos(la1)*math.Sin(la2) - math.Sin(la1)*math.Cos(la2)*math.Cos(dLng)
	return math.Mod(toDeg(math.Atan2(y, x))+360, 360)
}

// destination moves distKm from p along bearing.
func destination(p LngLat, bearing, distKm float64) LngLat {
	br := toRad(bearing)
	la1, lo1 := toRad(p.Lat), toRad(p.Lng)
	ad := distKm / earthRadiusKm
	la2 := math.Asin(math.Sin(la1)*math.Cos(ad) + math.Cos(la1)*math.Sin(ad)*math.Cos(br))
	lo2 := lo1 + math.Atan2(math.Sin(br)*math.Sin(ad)*math.Cos(la1), math.Cos(ad)-math.Sin(la1)*math.Sin(la2))
	return LngLat{toDeg(lo2), toDeg(la2)}
}

// randomPointWithin is uniform over the disk of radiusKm around center.
func randomPointWithin(rng *rand.Rand, center LngLat, radiusKm float64) LngLat {
	return destination(center, rng.Float64()*360, radiusKm*math.Sqrt(rng.Float64()))
}

// lerp interpolates between a and b (fine at city scale).
func lerp(a, b LngLat, f float64) LngLat {
	return LngLat{a.Lng + (b.Lng-a.Lng)*f, a.Lat + (b.Lat-a.Lat)*f}
}

// Route is a polyline with cumulative distances, sampled by distance travelled.
type Route struct {
	pts []LngLat
	cum []float64
}

func newRoute(pts []LngLat) *Route {
	r := &Route{pts: pts, cum: make([]float64, len(pts))}
	for i := 1; i < len(pts); i++ {
		r.cum[i] = r.cum[i-1] + haversineKm(pts[i-1], pts[i])
	}
	return r
}

func (r *Route) TotalKm() float64 {
	if len(r.cum) == 0 {
		return 0
	}
	return r.cum[len(r.cum)-1]
}

func (r *Route) End() LngLat { return r.pts[len(r.pts)-1] }

// At returns the position and heading after distKm along the route, and
// whether the end has been reached.
func (r *Route) At(distKm float64) (LngLat, float64, bool) {
	n := len(r.pts)
	if n == 0 {
		return LngLat{}, 0, true
	}
	if n == 1 || distKm >= r.TotalKm() {
		h := 0.0
		if n >= 2 {
			h = bearingDeg(r.pts[n-2], r.pts[n-1])
		}
		return r.pts[n-1], h, true
	}
	if distKm <= 0 {
		return r.pts[0], bearingDeg(r.pts[0], r.pts[1]), false
	}
	i := 1
	for i < n-1 && r.cum[i] < distKm {
		i++
	}
	a, b := r.pts[i-1], r.pts[i]
	seg := r.cum[i] - r.cum[i-1]
	f := 0.0
	if seg > 0 {
		f = (distKm - r.cum[i-1]) / seg
	}
	return lerp(a, b, f), bearingDeg(a, b), false
}

// buildRoute makes a wiggly polyline from start to dest with k intermediate
// waypoints spread along the straight line and jittered sideways.
func buildRoute(rng *rand.Rand, start, dest LngLat, k int, jitterKm float64) *Route {
	pts := []LngLat{start}
	for i := 1; i <= k; i++ {
		p := lerp(start, dest, float64(i)/float64(k+1))
		pts = append(pts, destination(p, rng.Float64()*360, jitterKm*rng.Float64()))
	}
	pts = append(pts, dest)
	return newRoute(pts)
}

// socDropPct is the battery percentage consumed over distKm.
func socDropPct(distKm, batteryKwh float64) float64 {
	if batteryKwh <= 0 {
		return 0
	}
	return distKm / 100 * consumptionKwhPer100km / batteryKwh * 100
}

// chargeGainPct is the battery percentage gained charging at powerKw for hours.
func chargeGainPct(powerKw, hours, batteryKwh float64) float64 {
	if batteryKwh <= 0 {
		return 0
	}
	return powerKw * hours / batteryKwh * 100
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
