package telemetry

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// TelemetryRequest is the body of POST /ingest/telemetry.
type TelemetryRequest struct {
	Points []Point `json:"points" binding:"required,min=1,max=500,dive"`
}

// TelemetryResult is the data of POST /ingest/telemetry.
type TelemetryResult struct {
	Accepted int        `json:"accepted"`
	TripID   *uuid.UUID `json:"trip_id"`
}

// EventRequest is the body of POST /ingest/events (contract IngestEvent).
type EventRequest struct {
	Type      string          `json:"type" binding:"required,oneof=trip_start trip_end"`
	TripStart *TripStartEvent `json:"trip_start"`
	TripEnd   *TripEndEvent   `json:"trip_end"`
}

// EventResult is the data of POST /ingest/events (contract IngestEventResult).
type EventResult struct {
	Type     string     `json:"type"`
	TripID   *uuid.UUID `json:"trip_id"`
	TripNo   *string    `json:"trip_no"`
	TripType *string    `json:"trip_type"`
	SignOn   bool       `json:"sign_on"`
	DriverID *uuid.UUID `json:"driver_id"`
}

const (
	EventTripStart = "trip_start"
	EventTripEnd   = "trip_end"
)

// ErrNotBound is returned when the gateway has no vehicle to report for.
var ErrNotBound = httpx.Conflict("device not bound to a vehicle")

// SortPoints returns a copy ordered by ts ascending (stable for equal stamps).
func SortPoints(pts []Point) []Point {
	out := make([]Point, len(pts))
	copy(out, pts)
	sort.SliceStable(out, func(i, j int) bool { return out[i].TS.Before(out[j].TS) })
	return out
}

// WasOnline reports whether a device that last reported at last counts as
// online at now, i.e. whether this report is NOT an offline→online transition.
func WasOnline(last *time.Time, now time.Time) bool {
	return last != nil && now.Sub(*last) < vstatus.OfflineAfter
}

// HandleEvent validates an event request, dispatches it to the trip hooks and
// maps the hook result onto the wire shape. Hook errors are returned as-is so
// an *httpx.AppError keeps its status and message.
func HandleEvent(ctx context.Context, hooks TripHooks, dev Device, req EventRequest) (*EventResult, error) {
	if dev.VehicleID == nil {
		return nil, ErrNotBound
	}
	switch req.Type {
	case EventTripStart:
		if req.TripStart == nil {
			return nil, httpx.BadRequest("trip_start is required")
		}
		ev := *req.TripStart
		ev.CardUID = strings.ToUpper(strings.TrimSpace(ev.CardUID))
		if ev.CardUID == "" && ev.UserID == nil {
			return nil, httpx.BadRequest("card_uid or user_id is required")
		}
		r, err := hooks.DeviceTripStart(ctx, dev, ev)
		if err != nil {
			return nil, err
		}
		if r == nil {
			return nil, httpx.Internal(errors.New("trip hook returned no result"))
		}
		tripID, driverID, tripNo, tripType := r.TripID, r.DriverID, r.TripNo, r.TripType
		return &EventResult{Type: req.Type, TripID: &tripID, TripNo: &tripNo, TripType: &tripType, SignOn: r.SignOn, DriverID: &driverID}, nil
	case EventTripEnd:
		if req.TripEnd == nil {
			return nil, httpx.BadRequest("trip_end is required")
		}
		id, err := hooks.DeviceTripEnd(ctx, dev, *req.TripEnd)
		if err != nil {
			return nil, err
		}
		return &EventResult{Type: req.Type, TripID: &id}, nil
	}
	return nil, httpx.BadRequest("unknown event type " + req.Type)
}

// toSnapshot picks the fields of a point that update vehicle_status.
func toSnapshot(p Point) vstatus.Telemetry {
	return vstatus.Telemetry{
		TS: p.TS, Lng: p.Lng, Lat: p.Lat, Speed: p.Speed, Heading: p.Heading,
		SOC: p.SOC, SOH: p.SOH, OdometerKm: p.OdometerKm,
		SignOn: p.SignOn, Locked: p.Locked, Charging: p.Charging,
	}
}
