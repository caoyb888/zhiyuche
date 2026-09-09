// Package pile manages the charge pile archive (/assets/charge-piles). Only
// the archive is edited here; the live status comes from OCPP in phase 3, so
// an operator may merely disable a pile or restore it to offline.
package pile

import (
	"time"

	"github.com/google/uuid"
)

// Pile is the API representation (contract schema Pile).
type Pile struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	TenantID        uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	PileCode        string     `json:"pile_code" db:"pile_code"`
	Name            string     `json:"name" db:"name"`
	Type            string     `json:"type" db:"type"`
	PowerKw         float64    `json:"power_kw" db:"power_kw"`
	ConnectorCount  int        `json:"connector_count" db:"connector_count"`
	Vendor          *string    `json:"vendor" db:"vendor"`
	Location        *string    `json:"location" db:"location"`
	Lng             *float64   `json:"lng" db:"lng"`
	Lat             *float64   `json:"lat" db:"lat"`
	Status          string     `json:"status" db:"status"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at" db:"last_heartbeat_at"`
	Remark          *string    `json:"remark" db:"remark"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// ListQuery mirrors the query string of GET /assets/charge-piles.
type ListQuery struct {
	Keyword string `form:"keyword"`
	Type    string `form:"type" binding:"omitempty,oneof=fast slow"`
	Status  string `form:"status" binding:"omitempty,oneof=available charging offline faulted disabled"`
}

type CreateRequest struct {
	PileCode       string   `json:"pile_code" binding:"required"`
	Name           string   `json:"name" binding:"required,min=1,max=64"`
	Type           *string  `json:"type" binding:"omitempty,oneof=fast slow"`
	PowerKw        *float64 `json:"power_kw" binding:"omitempty,gt=0"`
	ConnectorCount *int     `json:"connector_count" binding:"omitempty,min=1,max=32"`
	Vendor         *string  `json:"vendor" binding:"omitempty,max=64"`
	Location       *string  `json:"location" binding:"omitempty,max=200"`
	Lng            *float64 `json:"lng" binding:"omitempty,gte=-180,lte=180"`
	Lat            *float64 `json:"lat" binding:"omitempty,gte=-90,lte=90"`
	Remark         *string  `json:"remark" binding:"omitempty,max=500"`
}

// UpdateRequest: status may only be set to disabled or offline by hand.
type UpdateRequest struct {
	Name           *string  `json:"name" binding:"omitempty,min=1,max=64"`
	Type           *string  `json:"type" binding:"omitempty,oneof=fast slow"`
	PowerKw        *float64 `json:"power_kw" binding:"omitempty,gt=0"`
	ConnectorCount *int     `json:"connector_count" binding:"omitempty,min=1,max=32"`
	Vendor         *string  `json:"vendor" binding:"omitempty,max=64"`
	Location       *string  `json:"location" binding:"omitempty,max=200"`
	Lng            *float64 `json:"lng" binding:"omitempty,gte=-180,lte=180"`
	Lat            *float64 `json:"lat" binding:"omitempty,gte=-90,lte=90"`
	Status         *string  `json:"status"`
	Remark         *string  `json:"remark" binding:"omitempty,max=500"`
}
