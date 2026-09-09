// Package device manages VIG-100E gateway devices (/assets/devices): the
// archive, vehicle binding (one gateway per vehicle) and the api key the
// gateway uses to authenticate on /ingest.
package device

import (
	"time"

	"github.com/google/uuid"
)

// Device is the API representation (contract schema Device).
type Device struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	TenantID     uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	SerialNo     string     `json:"serial_no" db:"serial_no"`
	VehicleID    *uuid.UUID `json:"vehicle_id" db:"vehicle_id"`
	VehiclePlate *string    `json:"vehicle_plate" db:"vehicle_plate"`
	Model        string     `json:"model" db:"model"`
	Firmware     *string    `json:"firmware" db:"firmware"`
	ICCID        *string    `json:"iccid" db:"iccid"`
	Status       string     `json:"status" db:"status"`
	Online       bool       `json:"online" db:"-"`
	LastOnlineAt *time.Time `json:"last_online_at" db:"last_online_at"`
	LastIP       *string    `json:"last_ip" db:"last_ip"`
	Remark       *string    `json:"remark" db:"remark"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// WithKey is returned by create and rotate-key: the plaintext key appears exactly once.
type WithKey struct {
	Device
	APIKey string `json:"api_key"`
}

// ListQuery mirrors the query string of GET /assets/devices.
type ListQuery struct {
	Keyword string `form:"keyword"`
	Status  string `form:"status" binding:"omitempty,oneof=active disabled"`
	Bound   *bool  `form:"bound"`
	Online  *bool  `form:"online"`
}

type CreateRequest struct {
	SerialNo  string     `json:"serial_no" binding:"required"`
	VehicleID *uuid.UUID `json:"vehicle_id"`
	Model     *string    `json:"model" binding:"omitempty,max=64"`
	Firmware  *string    `json:"firmware" binding:"omitempty,max=64"`
	ICCID     *string    `json:"iccid" binding:"omitempty,max=32"`
	Remark    *string    `json:"remark" binding:"omitempty,max=500"`
}

type UpdateRequest struct {
	Model    *string `json:"model" binding:"omitempty,min=1,max=64"`
	Firmware *string `json:"firmware" binding:"omitempty,max=64"`
	ICCID    *string `json:"iccid" binding:"omitempty,max=32"`
	Status   *string `json:"status" binding:"omitempty,oneof=active disabled"`
	Remark   *string `json:"remark" binding:"omitempty,max=500"`
}

type BindRequest struct {
	VehicleID uuid.UUID `json:"vehicle_id" binding:"required"`
}
