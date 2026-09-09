// Package trip owns trips, their track points and events, the device-side
// start/end hooks (telemetry.TripHooks), and the dashboard overview.
package trip

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
)

// Trip statuses / sources / event types.
const (
	StatusOngoing   = "ongoing"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"

	SourceDevice = "device"
	SourceWeb    = "web"

	EvStart     = "start"
	EvEnd       = "end"
	EvDeviation = "deviation"
	EvOverspeed = "overspeed"
	EvLowSOC    = "low_soc"
	EvSignOn    = "sign_on"
	EvSignOff   = "sign_off"
	EvCancel    = "cancel"
)

// Permission codes used by this module.
const (
	PermView      = "trip:view"
	PermManage    = "trip:manage"
	PermExport    = "trip:export"
	PermDashboard = "dashboard:view"
)

// Rule thresholds.
const (
	DeviationMeters   = 1000.0          // off-route distance that raises a deviation
	EventCooldown     = 2 * time.Minute // min gap between two events of the same type
	DefaultOverspeed  = 80              // km/h, param trip.overspeed_kmh
	DefaultLowSOCLock = 10              // %, param vehicle.low_soc_lock_percent (+10 = alert)
	HarshAccelG       = 0.25
	HarshBrakeG       = 0.35
	ParamOverspeed    = "trip.overspeed_kmh"
)

// ApprovalRef is the approval summary embedded in a trip.
type ApprovalRef struct {
	ID            uuid.UUID `json:"id"`
	ApplyNo       string    `json:"apply_no"`
	PurposeDetail string    `json:"purpose_detail"`
	PlannedKm     *float64  `json:"planned_km"`
	Destination   string    `json:"destination"`
}

// Event is a trip_events row.
type Event struct {
	ID        int64           `json:"id" db:"id"`
	TenantID  uuid.UUID       `json:"-" db:"tenant_id"`
	TripID    uuid.UUID       `json:"trip_id" db:"trip_id"`
	VehicleID uuid.UUID       `json:"vehicle_id" db:"vehicle_id"`
	Type      string          `json:"type" db:"type"`
	TS        time.Time       `json:"ts" db:"ts"`
	Payload   json.RawMessage `json:"payload" db:"payload"`
}

// EventWithVehicle is an event enriched for the dashboard / WebSocket.
type EventWithVehicle struct {
	Event
	PlateNo    string  `json:"plate_no" db:"plate_no"`
	DriverName *string `json:"driver_name" db:"driver_name"`
	TripNo     string  `json:"trip_no" db:"trip_no"`
}

// Trip is the API representation.
type Trip struct {
	ID             uuid.UUID             `json:"id"`
	TenantID       uuid.UUID             `json:"tenant_id"`
	TripNo         string                `json:"trip_no"`
	Vehicle        approval.VehicleBrief `json:"vehicle"`
	Driver         *approval.UserBrief   `json:"driver"`
	CardUID        *string               `json:"card_uid"`
	Approval       *ApprovalRef          `json:"approval"`
	TripType       string                `json:"trip_type"`
	Purpose        *string               `json:"purpose"`
	Source         string                `json:"source"`
	Status         string                `json:"status"`
	StartAt        time.Time             `json:"start_at"`
	EndAt          *time.Time            `json:"end_at"`
	DurationMin    *float64              `json:"duration_min"`
	StartOdometer  *float64              `json:"start_odometer"`
	EndOdometer    *float64              `json:"end_odometer"`
	DistanceKm     *float64              `json:"distance_km"`
	EnergyKwh      *float64              `json:"energy_kwh"`
	EnergyPer100Km *float64              `json:"energy_per_100km"`
	StartSOC       *float64              `json:"start_soc"`
	EndSOC         *float64              `json:"end_soc"`
	AvgSpeed       *float64              `json:"avg_speed"`
	MaxSpeed       *float64              `json:"max_speed"`
	HarshAccel     int                   `json:"harsh_accel"`
	HarshBrake     int                   `json:"harsh_brake"`
	PointCount     int                   `json:"point_count"`
	RoofSignStatus string                `json:"roof_sign_status"`
	DeviationFlag  bool                  `json:"deviation_flag"`
	DeviationMaxM  *float64              `json:"deviation_max_m"`
	StartLng       *float64              `json:"start_lng"`
	StartLat       *float64              `json:"start_lat"`
	EndLng         *float64              `json:"end_lng"`
	EndLat         *float64              `json:"end_lat"`
	Cost           *float64              `json:"cost"`
	CostDetail     json.RawMessage       `json:"cost_detail"`
	Remark         *string               `json:"remark"`
	Events         []Event               `json:"events,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

// TrackPoint is one trip_points row.
type TrackPoint struct {
	TS      time.Time `json:"ts" db:"ts"`
	Lng     float64   `json:"lng" db:"lng"`
	Lat     float64   `json:"lat" db:"lat"`
	Speed   *float64  `json:"speed" db:"speed"`
	Heading *float64  `json:"heading" db:"heading"`
	SOC     *float64  `json:"soc" db:"soc"`
}

// Track is the answer of GET /trips/{id}/track.
type Track struct {
	TripID       uuid.UUID              `json:"trip_id"`
	PlannedRoute *approval.PlannedRoute `json:"planned_route"`
	Points       []TrackPoint           `json:"points"`
}

// Summary is the daily aggregate (GET /trips/summary, dashboard.today).
type Summary struct {
	Date           string  `json:"date"`
	Trips          int     `json:"trips"`
	DistanceKm     float64 `json:"distance_km"`
	EnergyKwh      float64 `json:"energy_kwh"`
	Ongoing        int     `json:"ongoing"`
	OfficialTrips  int     `json:"official_trips"`
	DeviationTrips int     `json:"deviation_trips"`
}

// VehicleCounts is dashboard.vehicles.
type VehicleCounts struct {
	Total       int `json:"total"`
	Idle        int `json:"idle"`
	InUse       int `json:"in_use"`
	Charging    int `json:"charging"`
	Maintenance int `json:"maintenance"`
	Disabled    int `json:"disabled"`
	Offline     int `json:"offline"`
}

// DeviceCounts is dashboard.devices.
type DeviceCounts struct {
	Total  int `json:"total"`
	Online int `json:"online"`
}

// Overview is the answer of GET /dashboard/overview.
type Overview struct {
	Vehicles         VehicleCounts      `json:"vehicles"`
	ApprovalsTodo    int64              `json:"approvals_todo"`
	ApprovalsPending int64              `json:"approvals_pending"`
	Today            Summary            `json:"today"`
	Devices          DeviceCounts       `json:"devices"`
	RecentEvents     []EventWithVehicle `json:"recent_events"`
}

// ListQuery mirrors the query string of GET /trips (and /trips/export).
type ListQuery struct {
	Scope     string `form:"scope" binding:"omitempty,oneof=mine all"`
	Status    string `form:"status" binding:"omitempty,oneof=ongoing completed cancelled"`
	TripType  string `form:"trip_type" binding:"omitempty,oneof=official daily"`
	VehicleID string `form:"vehicle_id" binding:"omitempty,uuid"`
	DriverID  string `form:"driver_id" binding:"omitempty,uuid"`
	DeptID    string `form:"dept_id" binding:"omitempty,uuid"`
	From      string `form:"from"`
	To        string `form:"to"`
	Keyword   string `form:"keyword"`
	Deviation *bool  `form:"deviation"`
}

// StartRequest is the body of POST /trips/start.
type StartRequest struct {
	ApprovalID uuid.UUID  `json:"approval_id" binding:"required"`
	DriverID   *uuid.UUID `json:"driver_id"`
	Remark     *string    `json:"remark" binding:"omitempty,max=500"`
}

// EndRequest is the body of POST /trips/{id}/end.
type EndRequest struct {
	EndOdometer *float64 `json:"end_odometer" binding:"omitempty,gte=0"`
	EndSOC      *float64 `json:"end_soc" binding:"omitempty,gte=0,lte=100"`
	Remark      *string  `json:"remark" binding:"omitempty,max=500"`
}

// CancelRequest is the body of POST /trips/{id}/cancel.
type CancelRequest struct {
	Reason *string `json:"reason" binding:"omitempty,max=500"`
}
