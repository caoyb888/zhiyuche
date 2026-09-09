// Package vehicle manages the vehicle archive (/assets/vehicles) and exposes
// the live status and history telemetry queries. It follows the
// store / service / handler layout of internal/system/user.
package vehicle

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
)

// Date is a calendar day (contract `format: date`) backed by a PostgreSQL date column.
type Date struct{ time.Time }

const dateLayout = "2006-01-02"

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.Format(dateLayout)) }

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("date must be a string YYYY-MM-DD")
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, s); err != nil {
			return fmt.Errorf("invalid date %q, want YYYY-MM-DD", s)
		}
	}
	d.Time = t
	return nil
}

// Scan implements sql.Scanner; pgx hands date columns over as time.Time.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		d.Time = v
	case string:
		t, err := time.Parse(dateLayout, v)
		if err != nil {
			return err
		}
		d.Time = t
	case nil:
		d.Time = time.Time{}
	default:
		return fmt.Errorf("cannot scan %T into Date", src)
	}
	return nil
}

// Value implements driver.Valuer.
func (d Date) Value() (driver.Value, error) { return d.Time, nil }

// dateArg converts an optional Date into a plain *time.Time query argument.
func dateArg(d *Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

// Vehicle is the API representation (contract schema Vehicle).
type Vehicle struct {
	ID               uuid.UUID            `json:"id" db:"id"`
	TenantID         uuid.UUID            `json:"tenant_id" db:"tenant_id"`
	PlateNo          string               `json:"plate_no" db:"plate_no"`
	VIN              *string              `json:"vin" db:"vin"`
	Brand            *string              `json:"brand" db:"brand"`
	Model            *string              `json:"model" db:"model"`
	Color            *string              `json:"color" db:"color"`
	SeatCount        int                  `json:"seat_count" db:"seat_count"`
	BatteryKwh       float64              `json:"battery_kwh" db:"battery_kwh"`
	RangeKmFull      int                  `json:"range_km_full" db:"range_km_full"`
	Status           string               `json:"status" db:"status"`
	OdometerKm       float64              `json:"odometer_km" db:"odometer_km"`
	SOC              *float64             `json:"soc" db:"soc"`
	SOH              *float64             `json:"soh" db:"soh"`
	PurchaseDate     *Date                `json:"purchase_date" db:"purchase_date"`
	InsuranceExpire  *Date                `json:"insurance_expire" db:"insurance_expire"`
	InspectionExpire *Date                `json:"inspection_expire" db:"inspection_expire"`
	HomeDeptID       *uuid.UUID           `json:"home_dept_id" db:"home_dept_id"`
	HomeDeptName     *string              `json:"home_dept_name" db:"home_dept_name"`
	DeviceID         *uuid.UUID           `json:"device_id" db:"device_id"`
	DeviceSerial     *string              `json:"device_serial" db:"device_serial"`
	Remark           *string              `json:"remark" db:"remark"`
	Live             *vstatus.VehicleLive `json:"live" db:"-"`
	TripCount        *int64               `json:"trip_count,omitempty" db:"-"` // detail only
	CreatedAt        time.Time            `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at" db:"updated_at"`
}

// Brief is the dropdown row (contract schema VehicleBrief).
type Brief struct {
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

// TelemetryPoint is one archived sample (contract schema TelemetryPoint).
type TelemetryPoint struct {
	TS         time.Time `json:"ts" db:"ts"`
	Lng        *float64  `json:"lng" db:"lng"`
	Lat        *float64  `json:"lat" db:"lat"`
	Speed      *float64  `json:"speed" db:"speed"`
	Heading    *float64  `json:"heading" db:"heading"`
	SOC        *float64  `json:"soc" db:"soc"`
	SOH        *float64  `json:"soh" db:"soh"`
	CellTemp   *float64  `json:"cell_temp" db:"cell_temp"`
	MotorTemp  *float64  `json:"motor_temp" db:"motor_temp"`
	OdometerKm *float64  `json:"odometer_km" db:"odometer_km"`
	AccOn      *bool     `json:"acc_on" db:"acc_on"`
	Locked     *bool     `json:"locked" db:"locked"`
	SignOn     *bool     `json:"sign_on" db:"sign_on"`
	Charging   *bool     `json:"charging" db:"charging"`
}

// ListQuery mirrors the query string of GET /assets/vehicles.
type ListQuery struct {
	Keyword string `form:"keyword"`
	Status  string `form:"status" binding:"omitempty,oneof=idle in_use charging maintenance disabled"`
	DeptID  string `form:"dept_id"`
	Online  *bool  `form:"online"`
}

type CreateRequest struct {
	PlateNo          string     `json:"plate_no" binding:"required,min=2,max=16"`
	VIN              *string    `json:"vin" binding:"omitempty,max=32"`
	Brand            *string    `json:"brand" binding:"omitempty,max=64"`
	Model            *string    `json:"model" binding:"omitempty,max=64"`
	Color            *string    `json:"color" binding:"omitempty,max=32"`
	SeatCount        *int       `json:"seat_count" binding:"omitempty,min=1,max=60"`
	BatteryKwh       *float64   `json:"battery_kwh" binding:"omitempty,gte=1"`
	RangeKmFull      *int       `json:"range_km_full" binding:"omitempty,min=1"`
	OdometerKm       *float64   `json:"odometer_km" binding:"omitempty,gte=0"`
	PurchaseDate     *Date      `json:"purchase_date"`
	InsuranceExpire  *Date      `json:"insurance_expire"`
	InspectionExpire *Date      `json:"inspection_expire"`
	HomeDeptID       *uuid.UUID `json:"home_dept_id"`
	Remark           *string    `json:"remark" binding:"omitempty,max=500"`
}

// UpdateRequest only touches fields that are present; clear_home_dept empties home_dept_id.
type UpdateRequest struct {
	PlateNo          *string    `json:"plate_no" binding:"omitempty,min=2,max=16"`
	VIN              *string    `json:"vin" binding:"omitempty,max=32"`
	Brand            *string    `json:"brand" binding:"omitempty,max=64"`
	Model            *string    `json:"model" binding:"omitempty,max=64"`
	Color            *string    `json:"color" binding:"omitempty,max=32"`
	SeatCount        *int       `json:"seat_count" binding:"omitempty,min=1,max=60"`
	BatteryKwh       *float64   `json:"battery_kwh" binding:"omitempty,gte=1"`
	RangeKmFull      *int       `json:"range_km_full" binding:"omitempty,min=1"`
	OdometerKm       *float64   `json:"odometer_km" binding:"omitempty,gte=0"`
	PurchaseDate     *Date      `json:"purchase_date"`
	InsuranceExpire  *Date      `json:"insurance_expire"`
	InspectionExpire *Date      `json:"inspection_expire"`
	HomeDeptID       *uuid.UUID `json:"home_dept_id"`
	ClearHomeDept    bool       `json:"clear_home_dept"`
	Status           *string    `json:"status" binding:"omitempty,oneof=idle maintenance disabled"`
	Remark           *string    `json:"remark" binding:"omitempty,max=500"`
}
