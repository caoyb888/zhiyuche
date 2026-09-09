package telemetry

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Point is one telemetry sample from a gateway (all optional fields are pointers).
type Point struct {
	TS         time.Time `json:"ts" binding:"required"`
	Lng        *float64  `json:"lng"`
	Lat        *float64  `json:"lat"`
	Speed      *float64  `json:"speed"`       // km/h
	Heading    *float64  `json:"heading"`     // degrees
	SOC        *float64  `json:"soc"`         // %
	SOH        *float64  `json:"soh"`         // %
	CellTemp   *float64  `json:"cell_temp"`   // °C
	MotorTemp  *float64  `json:"motor_temp"`  // °C
	OdometerKm *float64  `json:"odometer_km"` // km
	AccOn      *bool     `json:"acc_on"`
	Locked     *bool     `json:"locked"`
	SignOn     *bool     `json:"sign_on"`
	Charging   *bool     `json:"charging"`
}

// TripStartEvent is sent when a driver authenticates at the vehicle.
type TripStartEvent struct {
	TS         time.Time `json:"ts" binding:"required"`
	CardUID    string    `json:"card_uid"`    // NFC card; or
	UserID     *uuid.UUID `json:"user_id"`    // app/bluetooth key
	ApprovalNo string    `json:"approval_no"` // optional explicit apply_no
	Lng        *float64  `json:"lng"`
	Lat        *float64  `json:"lat"`
	OdometerKm *float64  `json:"odometer_km"`
	SOC        *float64  `json:"soc"`
}

// TripEndEvent is sent when the driver returns the vehicle.
type TripEndEvent struct {
	TS         time.Time `json:"ts" binding:"required"`
	Lng        *float64  `json:"lng"`
	Lat        *float64  `json:"lat"`
	OdometerKm *float64  `json:"odometer_km"`
	SOC        *float64  `json:"soc"`
}

// TripStartResult tells the gateway what to do.
type TripStartResult struct {
	TripID   uuid.UUID `json:"trip_id"`
	TripNo   string    `json:"trip_no"`
	TripType string    `json:"trip_type"` // official → gateway lights the roof sign
	SignOn   bool      `json:"sign_on"`
	DriverID uuid.UUID `json:"driver_id"`
}

// TripHooks is implemented by the trip module and injected at wiring time, so
// the ingest path does not import trip and vice versa.
type TripHooks interface {
	// DeviceTripStart validates the driver/approval and opens a trip. Errors are
	// returned to the gateway as a rejection (httpx.AppError → its message).
	DeviceTripStart(ctx context.Context, dev Device, ev TripStartEvent) (*TripStartResult, error)
	// DeviceTripEnd closes the ongoing trip of the device's vehicle.
	DeviceTripEnd(ctx context.Context, dev Device, ev TripEndEvent) (tripID uuid.UUID, err error)
	// OnTelemetry appends points to the ongoing trip (if any) and evaluates trip rules.
	OnTelemetry(ctx context.Context, dev Device, vehicleID uuid.UUID, pts []Point) error
}
