package charging

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
)

// Permission codes used by this module (contract x-permission).
const (
	PermView   = "charging:view"
	PermManage = "charging:manage"
	PermReview = "charging:review"
	PermExport = "charging:export"
)

// Transaction statuses (charge_transactions.status).
const (
	StatusCharging  = "charging"
	StatusEnded     = "ended"
	StatusSettled   = "settled"
	StatusCancelled = "cancelled"
)

// Review statuses.
const (
	ReviewNone     = "none"
	ReviewPending  = "pending"
	ReviewApproved = "approved"
	ReviewRejected = "rejected"
)

// Vehicle binding methods (charge_transactions.bind_method).
const (
	BindManual     = "manual"      // vehicle preset by remote start / fixed at review
	BindLocation   = "location"    // vehicle parked within BindRadiusM of the pile
	BindRecentTrip = "recent_trip" // vehicle of the card holder's trip that ended recently
	BindCard       = "card"        // card holder known, no vehicle
	BindNone       = "none"        // neither
)

// OCPP ChargePointStatus values (pile_connectors.status).
const (
	ConnAvailable     = "Available"
	ConnPreparing     = "Preparing"
	ConnCharging      = "Charging"
	ConnSuspendedEV   = "SuspendedEV"
	ConnSuspendedEVSE = "SuspendedEVSE"
	ConnFinishing     = "Finishing"
	ConnReserved      = "Reserved"
	ConnUnavailable   = "Unavailable"
	ConnFaulted       = "Faulted"
)

// Pile statuses (charge_piles.status).
const (
	PileAvailable = "available"
	PileCharging  = "charging"
	PileOffline   = "offline"
	PileFaulted   = "faulted"
	PileDisabled  = "disabled"
)

// Notification types emitted by this module.
const (
	NotifyStarted = "charging.started"
	NotifyEnded   = "charging.ended"
	NotifyReview  = "charging.review"
)

// Rule thresholds.
const (
	DeviationThresholdPct = 5.0             // |pile − BMS| / pile above this → review
	BindRadiusM           = 200.0           // location binding radius
	BindTelemetryWindow   = 5 * time.Minute // vehicle must have reported within this window
	BindRecentTripWindow  = 60 * time.Minute
	DetailMeterPoints     = 200 // detail returns at most this many sampled meter values
	PendingVehicleTTL     = 10 * time.Minute
	DefaultHeartbeat      = 60 * time.Second
)

// Connector is one pile_connectors row (contract PileLive.connectors[]).
type Connector struct {
	ConnectorID int          `json:"connector_id" db:"connector_id"`
	Status      string       `json:"status" db:"status"`
	ErrorCode   *string      `json:"error_code" db:"error_code"`
	Info        *string      `json:"info,omitempty" db:"info"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
	Transaction *Transaction `json:"transaction" db:"-"`
}

// PileLive is the contract PileLive shape.
type PileLive struct {
	ID              uuid.UUID   `json:"id" db:"id"`
	TenantID        uuid.UUID   `json:"tenant_id" db:"tenant_id"`
	PileCode        string      `json:"pile_code" db:"pile_code"`
	Name            string      `json:"name" db:"name"`
	Type            string      `json:"type" db:"type"`
	PowerKw         float64     `json:"power_kw" db:"power_kw"`
	ConnectorCount  int         `json:"connector_count" db:"connector_count"`
	Vendor          *string     `json:"vendor" db:"vendor"`
	Location        *string     `json:"location" db:"location"`
	Lng             *float64    `json:"lng" db:"lng"`
	Lat             *float64    `json:"lat" db:"lat"`
	Status          string      `json:"status" db:"status"`
	Online          bool        `json:"online" db:"-"`
	LastHeartbeatAt *time.Time  `json:"last_heartbeat_at" db:"last_heartbeat_at"`
	Connectors      []Connector `json:"connectors" db:"-"`
}

// Transaction is the contract ChargeTransaction shape.
type Transaction struct {
	ID             uuid.UUID              `json:"id"`
	TenantID       uuid.UUID              `json:"tenant_id"`
	TxNo           string                 `json:"tx_no"`
	PileID         uuid.UUID              `json:"pile_id"`
	PileCode       string                 `json:"pile_code"`
	PileName       string                 `json:"pile_name"`
	ConnectorID    int                    `json:"connector_id"`
	OcppTxID       int                    `json:"ocpp_tx_id"`
	IDTag          string                 `json:"id_tag"`
	User           *approval.UserBrief    `json:"user"`
	DeptID         *uuid.UUID             `json:"dept_id"`
	DeptName       *string                `json:"dept_name"`
	Vehicle        *approval.VehicleBrief `json:"vehicle"`
	BindMethod     *string                `json:"bind_method"`
	Status         string                 `json:"status"`
	StartAt        time.Time              `json:"start_at"`
	EndAt          *time.Time             `json:"end_at"`
	DurationMin    *float64               `json:"duration_min"`
	MeterStart     float64                `json:"meter_start"`
	MeterStop      *float64               `json:"meter_stop"`
	Kwh            *float64               `json:"kwh"`
	PowerKw        *float64               `json:"power_kw"`
	UnitPrice      *float64               `json:"unit_price"`
	Cost           *float64               `json:"cost"`
	StopReason     *string                `json:"stop_reason"`
	BmsSocStart    *float64               `json:"bms_soc_start"`
	BmsSocEnd      *float64               `json:"bms_soc_end"`
	BmsKwhEst      *float64               `json:"bms_kwh_est"`
	DeviationPct   *float64               `json:"deviation_pct"`
	ReviewStatus   string                 `json:"review_status"`
	ReviewNote     *string                `json:"review_note"`
	ReviewedBy     *uuid.UUID             `json:"reviewed_by"`
	ReviewedByName *string                `json:"reviewed_by_name"`
	ReviewedAt     *time.Time             `json:"reviewed_at"`
	Attribution    *string                `json:"attribution"`
	AccountID      *uuid.UUID             `json:"account_id"`
	AccountTxnID   *int64                 `json:"account_txn_id"`
	MeterValues    []MeterValue           `json:"meter_values,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// MeterValue is one charge_meter_values row (contract MeterValue).
type MeterValue struct {
	TS      time.Time `json:"ts" db:"ts"`
	Wh      *float64  `json:"wh" db:"wh"`
	Kwh     *float64  `json:"kwh" db:"-"`
	Voltage *float64  `json:"voltage" db:"voltage"`
	Current *float64  `json:"current" db:"current"`
	PowerKw *float64  `json:"power_kw" db:"power_kw"`
	SOC     *float64  `json:"soc" db:"soc"`
}

// MeterSample is a parsed OCPP MeterValues entry handed over by the OCPP server.
type MeterSample struct {
	TS      time.Time
	Wh      *float64 // Energy.Active.Import.Register, normalised to Wh
	Voltage *float64 // V
	Current *float64 // A (Current.Import)
	PowerKw *float64 // Power.Active.Import, normalised to kW
	SOC     *float64 // %
	Raw     json.RawMessage
}

// SummaryBucket is {sessions, kwh, cost}.
type SummaryBucket struct {
	Sessions int     `json:"sessions"`
	Kwh      float64 `json:"kwh"`
	Cost     float64 `json:"cost"`
}

// PileCounts is ChargingSummary.piles.
type PileCounts struct {
	Total    int `json:"total"`
	Online   int `json:"online"`
	Charging int `json:"charging"`
	Faulted  int `json:"faulted"`
}

// PileSummary is one ChargingSummary.by_pile entry.
type PileSummary struct {
	PileID   uuid.UUID `json:"pile_id" db:"pile_id"`
	PileName string    `json:"pile_name" db:"pile_name"`
	Sessions int       `json:"sessions" db:"sessions"`
	Kwh      float64   `json:"kwh" db:"kwh"`
	Cost     float64   `json:"cost" db:"cost"`
}

// Summary is the contract ChargingSummary.
type Summary struct {
	Date          string        `json:"date"`
	Today         SummaryBucket `json:"today"`
	Month         SummaryBucket `json:"month"`
	Ongoing       int           `json:"ongoing"`
	PendingReview int           `json:"pending_review"`
	Piles         PileCounts    `json:"piles"`
	ByPile        []PileSummary `json:"by_pile"`
}

// ListQuery mirrors the query string of GET /charging/transactions (and /export).
type ListQuery struct {
	Status       string `form:"status" binding:"omitempty,oneof=charging ended settled cancelled"`
	ReviewStatus string `form:"review_status" binding:"omitempty,oneof=none pending approved rejected"`
	PileID       string `form:"pile_id" binding:"omitempty,uuid"`
	VehicleID    string `form:"vehicle_id" binding:"omitempty,uuid"`
	UserID       string `form:"user_id" binding:"omitempty,uuid"`
	DeptID       string `form:"dept_id" binding:"omitempty,uuid"`
	From         string `form:"from"`
	To           string `form:"to"`
	Keyword      string `form:"keyword"`
}

// RemoteStartRequest is the body of POST /charging/piles/{id}/remote-start.
type RemoteStartRequest struct {
	ConnectorID *int       `json:"connector_id" binding:"omitempty,min=1,max=32"`
	IDTag       *string    `json:"id_tag" binding:"omitempty,min=1,max=20"`
	VehicleID   *uuid.UUID `json:"vehicle_id"`
}

// RemoteStopRequest is the body of POST /charging/piles/{id}/remote-stop.
type RemoteStopRequest struct {
	TransactionID uuid.UUID `json:"transaction_id" binding:"required"`
}

// RemoteResult is the pile's answer ({status: Accepted|Rejected}).
type RemoteResult struct {
	Status string `json:"status"`
}

// ReviewRequest is the body of POST /charging/transactions/{id}/review.
type ReviewRequest struct {
	Action    string     `json:"action" binding:"required,oneof=approve reject"`
	Note      *string    `json:"note" binding:"omitempty,max=500"`
	VehicleID *uuid.UUID `json:"vehicle_id"`
	UserID    *uuid.UUID `json:"user_id"`
}

// ChargingUpdate is the payload of ws "charging.updated".
type ChargingUpdate struct {
	PileID      uuid.UUID  `json:"pile_id"`
	PileCode    string     `json:"pile_code"`
	PileStatus  string     `json:"pile_status"`
	Online      bool       `json:"online"`
	ConnectorID *int       `json:"connector_id,omitempty"`
	Connector   *string    `json:"connector_status,omitempty"`
	TxID        *uuid.UUID `json:"tx_id,omitempty"`
	TxNo        *string    `json:"tx_no,omitempty"`
	TxStatus    *string    `json:"tx_status,omitempty"`
	Kwh         *float64   `json:"kwh,omitempty"`
	PowerKw     *float64   `json:"power_kw,omitempty"`
	SOC         *float64   `json:"soc,omitempty"`
	Event       string     `json:"event"` // boot | heartbeat | status | started | meter | ended | offline
}
