// Package approval implements the official-vehicle request workflow:
// pre-check, one/two-level approval steps, rules, cancellation.
//
// Layout follows internal/system/user: dto.go (API shapes), rules.go (pure,
// unit-tested decision logic), store.go (SQL), service.go (business rules),
// handler.go (HTTP).
package approval

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Approval statuses.
const (
	StatusPendingL1 = "pending_l1"
	StatusPendingL2 = "pending_l2"
	StatusApproved  = "approved"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled"
	StatusInUse     = "in_use"
	StatusCompleted = "completed"
	StatusExpired   = "expired"
)

// Step actions.
const (
	ActionPending  = "pending"
	ActionApproved = "approved"
	ActionRejected = "rejected"
	ActionSkipped  = "skipped"
)

// Trip types.
const (
	TripTypeOfficial = "official"
	TripTypeDaily    = "daily"
)

// Permission codes used by this module.
const (
	PermView    = "approval:view"
	PermCreate  = "approval:create"
	PermApprove = "approval:approve"
	PermManage  = "approval:manage"
	PermRule    = "approval:rule"
)

// UserBrief is the compact user shape embedded in approvals and trips.
type UserBrief struct {
	ID       uuid.UUID `json:"id" db:"id"`
	Name     string    `json:"name" db:"name"`
	Username string    `json:"username" db:"username"`
	DeptName *string   `json:"dept_name" db:"dept_name"`
	Phone    *string   `json:"phone" db:"phone"`
}

// VehicleBrief is the compact vehicle shape (dropdowns, approvals, trips).
type VehicleBrief struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	PlateNo      string     `json:"plate_no" db:"plate_no"`
	Brand        *string    `json:"brand" db:"brand"`
	Model        *string    `json:"model" db:"model"`
	Status       string     `json:"status" db:"status"`
	SOC          *float64   `json:"soc" db:"soc"`
	RangeKm      *float64   `json:"range_km" db:"range_km"`
	HomeDeptID   *uuid.UUID `json:"home_dept_id" db:"home_dept_id"`
	HomeDeptName *string    `json:"home_dept_name" db:"home_dept_name"`
}

// PlannedRoute is the map route the applicant planned; points are [lng, lat].
type PlannedRoute struct {
	Points      [][]float64 `json:"points,omitempty"`
	DistanceKm  *float64    `json:"distance_km,omitempty"`
	DurationMin *float64    `json:"duration_min,omitempty"`
	Summary     string      `json:"summary,omitempty"`
}

// Attachment is an uploaded file reference.
type Attachment struct {
	Name string `json:"name" binding:"required"`
	URL  string `json:"url" binding:"required"`
}

// Step is one approval level with its approver and outcome.
type Step struct {
	StepNo   int        `json:"step_no"`
	Approver UserBrief  `json:"approver"`
	Action   string     `json:"action"`
	Remark   *string    `json:"remark"`
	ActedAt  *time.Time `json:"acted_at"`
}

// TripBrief is the trip summary embedded in an approval.
type TripBrief struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	TripNo     string     `json:"trip_no" db:"trip_no"`
	Status     string     `json:"status" db:"status"`
	StartAt    time.Time  `json:"start_at" db:"start_at"`
	EndAt      *time.Time `json:"end_at" db:"end_at"`
	DistanceKm *float64   `json:"distance_km" db:"distance_km"`
}

// Approval is the API representation.
type Approval struct {
	ID            uuid.UUID     `json:"id"`
	TenantID      uuid.UUID     `json:"tenant_id"`
	ApplyNo       string        `json:"apply_no"`
	Applicant     UserBrief     `json:"applicant"`
	DeptID        *uuid.UUID    `json:"dept_id"`
	DeptName      *string       `json:"dept_name"`
	TripType      string        `json:"trip_type"`
	PurposeCode   string        `json:"purpose_code"`
	PurposeLabel  *string       `json:"purpose_label"`
	PurposeDetail string        `json:"purpose_detail"`
	PlannedStart  time.Time     `json:"planned_start"`
	PlannedEnd    time.Time     `json:"planned_end"`
	Destination   string        `json:"destination"`
	DestLng       *float64      `json:"dest_lng"`
	DestLat       *float64      `json:"dest_lat"`
	PlannedRoute  *PlannedRoute `json:"planned_route"`
	PlannedKm     *float64      `json:"planned_km"`
	Passengers    []UserBrief   `json:"passengers"`
	Attachments   []Attachment  `json:"attachments"`
	Vehicle       *VehicleBrief `json:"vehicle"`
	Urgency       string        `json:"urgency"`
	Status        string        `json:"status"`
	LevelRequired int           `json:"level_required"`
	CurrentStep   int           `json:"current_step"`
	Steps         []Step        `json:"steps"`
	RejectReason  *string       `json:"reject_reason"`
	CancelReason  *string       `json:"cancel_reason"`
	ApprovedAt    *time.Time    `json:"approved_at"`
	Trip          *TripBrief    `json:"trip"`
	CanApprove    bool          `json:"can_approve"`
	CanCancel     bool          `json:"can_cancel"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// ListQuery mirrors the query string of GET /approvals.
type ListQuery struct {
	Scope       string `form:"scope" binding:"omitempty,oneof=mine todo all"`
	Status      string `form:"status" binding:"omitempty,oneof=pending_l1 pending_l2 approved rejected cancelled in_use completed expired"`
	TripType    string `form:"trip_type" binding:"omitempty,oneof=official daily"`
	DeptID      string `form:"dept_id" binding:"omitempty,uuid"`
	VehicleID   string `form:"vehicle_id" binding:"omitempty,uuid"`
	ApplicantID string `form:"applicant_id" binding:"omitempty,uuid"`
	From        string `form:"from"`
	To          string `form:"to"`
	Keyword     string `form:"keyword"`
}

// CreateRequest is the body of POST /approvals and /approvals/precheck.
type CreateRequest struct {
	ApplicantID   *uuid.UUID    `json:"applicant_id"`
	TripType      string        `json:"trip_type" binding:"required,oneof=official daily"`
	PurposeCode   string        `json:"purpose_code" binding:"required,max=64"`
	PurposeDetail string        `json:"purpose_detail" binding:"required,min=2,max=500"`
	PlannedStart  time.Time     `json:"planned_start" binding:"required"`
	PlannedEnd    time.Time     `json:"planned_end" binding:"required"`
	Destination   string        `json:"destination" binding:"required,min=1,max=200"`
	DestLng       *float64      `json:"dest_lng"`
	DestLat       *float64      `json:"dest_lat"`
	PlannedRoute  *PlannedRoute `json:"planned_route"`
	PlannedKm     *float64      `json:"planned_km" binding:"omitempty,gte=0"`
	PassengerIDs  []uuid.UUID   `json:"passenger_ids"`
	Attachments   []Attachment  `json:"attachments" binding:"omitempty,dive"`
	VehicleID     *uuid.UUID    `json:"vehicle_id"`
	Urgency       string        `json:"urgency" binding:"omitempty,oneof=normal urgent"`
}

// Conflict is another request that overlaps the requested vehicle window.
type Conflict struct {
	ApplyNo       string    `json:"apply_no" db:"apply_no"`
	ApplicantName string    `json:"applicant_name" db:"applicant_name"`
	PlannedStart  time.Time `json:"planned_start" db:"planned_start"`
	PlannedEnd    time.Time `json:"planned_end" db:"planned_end"`
	Status        string    `json:"status" db:"status"`
}

// PrecheckResult is the answer of POST /approvals/precheck.
type PrecheckResult struct {
	OK            bool        `json:"ok"`
	LevelRequired int         `json:"level_required"`
	Level2Reasons []string    `json:"level2_reasons"`
	Approvers     []UserBrief `json:"approvers"`
	Conflicts     []Conflict  `json:"conflicts"`
	Problems      []string    `json:"problems"`
}

// ApproveRequest is the body of POST /approvals/{id}/approve.
type ApproveRequest struct {
	Remark    *string    `json:"remark" binding:"omitempty,max=500"`
	VehicleID *uuid.UUID `json:"vehicle_id"`
}

// RejectRequest is the body of POST /approvals/{id}/reject.
type RejectRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

// CancelRequest is the body of POST /approvals/{id}/cancel.
type CancelRequest struct {
	Reason *string `json:"reason" binding:"omitempty,max=500"`
}

// AvailableQuery is the query string of GET /approvals/available-vehicles.
type AvailableQuery struct {
	Start             time.Time `form:"start" binding:"required" time_format:"2006-01-02T15:04:05Z07:00"`
	End               time.Time `form:"end" binding:"required" time_format:"2006-01-02T15:04:05Z07:00"`
	ExcludeApprovalID string    `form:"exclude_approval_id" binding:"omitempty,uuid"`
}

// Rules is the tenant approval rule set (GET/PUT /approvals/rules).
type Rules struct {
	Enabled              bool       `json:"enabled"`
	Level2Km             *float64   `json:"level2_km"`
	Level2Night          bool       `json:"level2_night"`
	NightStart           string     `json:"night_start"`
	NightEnd             string     `json:"night_end"`
	Level2CrossDept      bool       `json:"level2_cross_dept"`
	Level2TripTypes      []string   `json:"level2_trip_types"`
	Level2ApproverID     *uuid.UUID `json:"level2_approver_id"`
	Level2Approver       *UserBrief `json:"level2_approver"`
	FallbackApproverRole string     `json:"fallback_approver_role"`
	OverdueAlertMinutes  int        `json:"overdue_alert_minutes"`
	UpdatedAt            *time.Time `json:"updated_at"`
}

// DefaultRules mirrors the column defaults of approval_rules.
func DefaultRules() Rules {
	return Rules{
		Enabled:              true,
		Level2Night:          true,
		NightStart:           "22:00",
		NightEnd:             "06:00",
		Level2TripTypes:      []string{},
		FallbackApproverRole: "approver",
		OverdueAlertMinutes:  30,
	}
}

// RulesUpdate is the body of PUT /approvals/rules. Only present keys are
// applied; level2_km / level2_approver_id accept an explicit null to clear.
type RulesUpdate struct {
	Enabled              *bool      `json:"enabled"`
	Level2Km             *float64   `json:"level2_km" binding:"omitempty,gte=0"`
	Level2Night          *bool      `json:"level2_night"`
	NightStart           *string    `json:"night_start"`
	NightEnd             *string    `json:"night_end"`
	Level2CrossDept      *bool      `json:"level2_cross_dept"`
	Level2TripTypes      []string   `json:"level2_trip_types" binding:"omitempty,dive,oneof=official daily"`
	Level2ApproverID     *uuid.UUID `json:"level2_approver_id"`
	FallbackApproverRole *string    `json:"fallback_approver_role" binding:"omitempty,min=1,max=64"`
	OverdueAlertMinutes  *int       `json:"overdue_alert_minutes" binding:"omitempty,gte=5"`

	present map[string]bool
}

// UnmarshalJSON remembers which keys were present so an explicit null can be
// told apart from an absent key.
func (r *RulesUpdate) UnmarshalJSON(b []byte) error {
	type alias RulesUpdate
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		return err
	}
	*r = RulesUpdate(a)
	r.present = make(map[string]bool, len(keys))
	for k := range keys {
		r.present[k] = true
	}
	return nil
}

// Has reports whether the key appeared in the request body.
func (r *RulesUpdate) Has(key string) bool { return r.present[key] }
