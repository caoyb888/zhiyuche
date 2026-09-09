// Package vstatus owns the vehicle_status snapshot table: one row per vehicle
// with its live position, energy and state-machine status. Trip and telemetry
// code update it through this package so every change is also pushed over
// WebSocket as ws.EvVehicleStatus.
package vstatus

import (
	"context"
	"errors"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
)

// Vehicle status values (vehicles.status and vehicle_status.status share them).
const (
	Idle        = "idle"
	InUse       = "in_use"
	Charging    = "charging"
	Maintenance = "maintenance"
	Disabled    = "disabled"
)

// OfflineAfter: no telemetry for this long → the API reports online=false.
const OfflineAfter = 5 * time.Minute

type VehicleStatus struct {
	VehicleID       uuid.UUID  `json:"vehicle_id" db:"vehicle_id"`
	TenantID        uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	Status          string     `json:"status" db:"status"`
	CurrentTripID   *uuid.UUID `json:"current_trip_id" db:"current_trip_id"`
	DriverID        *uuid.UUID `json:"driver_id" db:"driver_id"`
	Lng             *float64   `json:"lng" db:"lng"`
	Lat             *float64   `json:"lat" db:"lat"`
	Speed           *float64   `json:"speed" db:"speed"`
	Heading         *float64   `json:"heading" db:"heading"`
	SOC             *float64   `json:"soc" db:"soc"`
	SOH             *float64   `json:"soh" db:"soh"`
	RangeKm         *float64   `json:"range_km" db:"range_km"`
	OdometerKm      *float64   `json:"odometer_km" db:"odometer_km"`
	SignOn          bool       `json:"sign_on" db:"sign_on"`
	Locked          bool       `json:"locked" db:"locked"`
	Charging        bool       `json:"charging" db:"charging"`
	LastTelemetryAt *time.Time `json:"last_telemetry_at" db:"last_telemetry_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	Online          bool       `json:"online" db:"-"`
}

func (v *VehicleStatus) computeOnline() {
	v.Online = v.LastTelemetryAt != nil && time.Since(*v.LastTelemetryAt) < OfflineAfter
}

type Store struct{ app *app.App }

func New(a *app.App) *Store { return &Store{app: a} }

// Ensure creates the snapshot row for a vehicle if missing (call on vehicle create).
func (s *Store) Ensure(ctx context.Context, tenantID, vehicleID uuid.UUID, status string) error {
	_, err := s.app.DB.Exec(ctx, `
		INSERT INTO vehicle_status (vehicle_id, tenant_id, status) VALUES ($1,$2,$3)
		ON CONFLICT (vehicle_id) DO NOTHING`, vehicleID, tenantID, status)
	return err
}

func (s *Store) Get(ctx context.Context, vehicleID uuid.UUID) (*VehicleStatus, error) {
	var v VehicleStatus
	err := pgxscan.Get(ctx, s.app.DB, &v, `SELECT * FROM vehicle_status WHERE vehicle_id = $1`, vehicleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.computeOnline()
	return &v, nil
}

func (s *Store) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]VehicleStatus, error) {
	var rows []VehicleStatus
	if err := pgxscan.Select(ctx, s.app.DB, &rows, `SELECT * FROM vehicle_status WHERE tenant_id = $1`, tenantID); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].computeOnline()
	}
	if rows == nil {
		rows = []VehicleStatus{}
	}
	return rows, nil
}

// SetStatus changes the state-machine status (idle/in_use/charging/maintenance/disabled),
// keeps vehicles.status in sync and pushes the snapshot.
func (s *Store) SetStatus(ctx context.Context, vehicleID uuid.UUID, status string, tripID, driverID *uuid.UUID) (*VehicleStatus, error) {
	tx, err := s.app.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE vehicle_status SET status = $2, current_trip_id = $3, driver_id = $4, updated_at = now()
		WHERE vehicle_id = $1`, vehicleID, status, tripID, driverID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE vehicles SET status = $2 WHERE id = $1 AND deleted_at IS NULL`, vehicleID, status); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.publish(ctx, vehicleID)
}

// Telemetry is the subset of a telemetry point that updates the snapshot.
type Telemetry struct {
	TS         time.Time
	Lng, Lat   *float64
	Speed      *float64
	Heading    *float64
	SOC, SOH   *float64
	OdometerKm *float64
	SignOn     *bool
	Locked     *bool
	Charging   *bool
}

// ApplyTelemetry updates position/energy fields from a point. Charging flag
// moves an idle vehicle to charging and back; in_use/maintenance/disabled are
// never overridden by telemetry. Returns the new snapshot (already pushed).
func (s *Store) ApplyTelemetry(ctx context.Context, vehicleID uuid.UUID, rangeKmFull float64, t Telemetry) (*VehicleStatus, error) {
	var rangeKm *float64
	if t.SOC != nil && rangeKmFull > 0 {
		r := *t.SOC / 100 * rangeKmFull
		rangeKm = &r
	}
	_, err := s.app.DB.Exec(ctx, `
		UPDATE vehicle_status SET
		  lng = COALESCE($2, lng), lat = COALESCE($3, lat), speed = COALESCE($4, speed), heading = COALESCE($5, heading),
		  soc = COALESCE($6, soc), soh = COALESCE($7, soh), range_km = COALESCE($8, range_km), odometer_km = COALESCE($9, odometer_km),
		  sign_on = COALESCE($10, sign_on), locked = COALESCE($11, locked), charging = COALESCE($12, charging),
		  status = CASE
		             WHEN status IN ('in_use','maintenance','disabled') THEN status
		             WHEN COALESCE($12, charging) THEN 'charging'
		             ELSE 'idle' END,
		  last_telemetry_at = GREATEST(COALESCE(last_telemetry_at, $13), $13), updated_at = now()
		WHERE vehicle_id = $1`,
		vehicleID, t.Lng, t.Lat, t.Speed, t.Heading, t.SOC, t.SOH, rangeKm, t.OdometerKm, t.SignOn, t.Locked, t.Charging, t.TS)
	if err != nil {
		return nil, err
	}
	// keep the archive row's headline numbers current
	_, _ = s.app.DB.Exec(ctx, `
		UPDATE vehicles v SET soc = COALESCE($2, v.soc), soh = COALESCE($3, v.soh), odometer_km = GREATEST(v.odometer_km, COALESCE($4, v.odometer_km)),
		  status = vs.status
		FROM vehicle_status vs WHERE vs.vehicle_id = v.id AND v.id = $1 AND v.deleted_at IS NULL`,
		vehicleID, t.SOC, t.SOH, t.OdometerKm)
	return s.publish(ctx, vehicleID)
}

func (s *Store) publish(ctx context.Context, vehicleID uuid.UUID) (*VehicleStatus, error) {
	v, err := s.Get(ctx, vehicleID)
	if err != nil || v == nil {
		return v, err
	}
	s.app.Hub.Publish(v.TenantID, ws.Event{Type: ws.EvVehicleStatus, Data: v})
	return v, nil
}
