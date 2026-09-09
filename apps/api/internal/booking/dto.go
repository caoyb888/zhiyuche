// Package booking implements 预约派车: bookings taken by phone (source=phone)
// or entered directly (source=direct). A booking is not an approval request —
// it takes effect the moment it is recorded, so the module only guards the
// vehicle-window conflicts it shares with the approval module.
//
// Layout follows internal/approval: dto.go (API shapes), rules.go (pure,
// unit-tested decision logic), store.go (SQL), service.go (business rules),
// handler.go (HTTP).
package booking

import (
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
)

// Sources: how the booking reached the dispatcher.
const (
	SourcePhone  = "phone"
	SourceDirect = "direct"
)

// Statuses.
const (
	StatusReserved  = "reserved"
	StatusDeparted  = "departed"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// Events driving the status machine.
const (
	EvDepart   = "depart"
	EvComplete = "complete"
	EvCancel   = "cancel"
)

// Permission codes used by this module.
const (
	PermView   = "booking:view"
	PermCreate = "booking:create"
	PermUpdate = "booking:update"
	PermCancel = "booking:cancel"
)

// Booking is the API representation.
type Booking struct {
	ID           uuid.UUID              `json:"id"`
	TenantID     uuid.UUID              `json:"tenant_id"`
	BookingNo    string                 `json:"booking_no"`
	Source       string                 `json:"source"`
	ContactName  string                 `json:"contact_name"`
	ContactPhone *string                `json:"contact_phone"`
	Passenger    *approval.UserBrief    `json:"passenger"`
	DeptID       *uuid.UUID             `json:"dept_id"`
	DeptName     *string                `json:"dept_name"`
	ReserveStart time.Time              `json:"reserve_start"`
	ReserveEnd   time.Time              `json:"reserve_end"`
	Vehicle      *approval.VehicleBrief `json:"vehicle"`
	Origin       string                 `json:"origin"`
	OriginLng    *float64               `json:"origin_lng"`
	OriginLat    *float64               `json:"origin_lat"`
	Destination  string                 `json:"destination"`
	DestLng      *float64               `json:"dest_lng"`
	DestLat      *float64               `json:"dest_lat"`
	Purpose      *string                `json:"purpose"`
	Remark       *string                `json:"remark"`
	Status       string                 `json:"status"`
	CancelReason *string                `json:"cancel_reason"`
	DepartedAt   *time.Time             `json:"departed_at"`
	CompletedAt  *time.Time             `json:"completed_at"`
	CreatedBy    *approval.UserBrief    `json:"created_by"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// ListQuery mirrors the query string of GET /bookings.
type ListQuery struct {
	Source    string `form:"source" binding:"omitempty,oneof=phone direct"`
	Status    string `form:"status" binding:"omitempty,oneof=reserved departed completed cancelled"`
	VehicleID string `form:"vehicle_id" binding:"omitempty,uuid"`
	DeptID    string `form:"dept_id" binding:"omitempty,uuid"`
	From      string `form:"from"`
	To        string `form:"to"`
	Keyword   string `form:"keyword"`
}

// CreateRequest is the body of POST /bookings.
type CreateRequest struct {
	Source       string     `json:"source" binding:"required,oneof=phone direct"`
	ContactName  string     `json:"contact_name" binding:"required,min=1,max=64"`
	ContactPhone *string    `json:"contact_phone" binding:"omitempty,max=32"`
	PassengerID  *uuid.UUID `json:"passenger_id"`
	ReserveStart time.Time  `json:"reserve_start" binding:"required"`
	ReserveEnd   time.Time  `json:"reserve_end" binding:"required"`
	VehicleID    *uuid.UUID `json:"vehicle_id"`
	Origin       string     `json:"origin" binding:"required,min=1,max=200"`
	OriginLng    *float64   `json:"origin_lng" binding:"omitempty,gte=-180,lte=180"`
	OriginLat    *float64   `json:"origin_lat" binding:"omitempty,gte=-90,lte=90"`
	Destination  string     `json:"destination" binding:"required,min=1,max=200"`
	DestLng      *float64   `json:"dest_lng" binding:"omitempty,gte=-180,lte=180"`
	DestLat      *float64   `json:"dest_lat" binding:"omitempty,gte=-90,lte=90"`
	Purpose      *string    `json:"purpose" binding:"omitempty,max=200"`
	Remark       *string    `json:"remark" binding:"omitempty,max=500"`
}

// UpdateRequest is the body of PUT /bookings/{id}. Absent keys are left alone;
// clearing the vehicle (back to 待派车) is done with clear_vehicle.
type UpdateRequest struct {
	Source         *string    `json:"source" binding:"omitempty,oneof=phone direct"`
	ContactName    *string    `json:"contact_name" binding:"omitempty,min=1,max=64"`
	ContactPhone   *string    `json:"contact_phone" binding:"omitempty,max=32"`
	PassengerID    *uuid.UUID `json:"passenger_id"`
	ClearPassenger bool       `json:"clear_passenger"`
	ReserveStart   *time.Time `json:"reserve_start"`
	ReserveEnd     *time.Time `json:"reserve_end"`
	VehicleID      *uuid.UUID `json:"vehicle_id"`
	ClearVehicle   bool       `json:"clear_vehicle"`
	Origin         *string    `json:"origin" binding:"omitempty,min=1,max=200"`
	OriginLng      *float64   `json:"origin_lng" binding:"omitempty,gte=-180,lte=180"`
	OriginLat      *float64   `json:"origin_lat" binding:"omitempty,gte=-90,lte=90"`
	Destination    *string    `json:"destination" binding:"omitempty,min=1,max=200"`
	DestLng        *float64   `json:"dest_lng" binding:"omitempty,gte=-180,lte=180"`
	DestLat        *float64   `json:"dest_lat" binding:"omitempty,gte=-90,lte=90"`
	Purpose        *string    `json:"purpose" binding:"omitempty,max=200"`
	Remark         *string    `json:"remark" binding:"omitempty,max=500"`
}

// CancelRequest is the body of POST /bookings/{id}/cancel.
type CancelRequest struct {
	Reason *string `json:"reason" binding:"omitempty,max=500"`
}

// Conflict is another booking or approval holding the same vehicle in an
// overlapping window.
type Conflict struct {
	Kind   string    `json:"kind" db:"kind"` // booking | approval
	No     string    `json:"no" db:"no"`
	Who    string    `json:"who" db:"who"`
	Start  time.Time `json:"start" db:"start"`
	End    time.Time `json:"end" db:"end"`
	Status string    `json:"status" db:"status"`
}

// AvailableQuery is the query string of GET /bookings/available-vehicles.
type AvailableQuery struct {
	Start            time.Time `form:"start" binding:"required" time_format:"2006-01-02T15:04:05Z07:00"`
	End              time.Time `form:"end" binding:"required" time_format:"2006-01-02T15:04:05Z07:00"`
	ExcludeBookingID string    `form:"exclude_booking_id" binding:"omitempty,uuid"`
}
