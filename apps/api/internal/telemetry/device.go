// Package telemetry is the ingestion edge for vehicle gateways: device
// authentication, telemetry points and trip events. In phase 2 the simulator
// (and any HTTP-capable gateway) posts here; in phase 4 the MQTT consumer
// (cmd/iot) reuses the same Service so both paths behave identically.
package telemetry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Device is the authenticated gateway attached to an ingest request.
type Device struct {
	ID        uuid.UUID  `db:"id"`
	TenantID  uuid.UUID  `db:"tenant_id"`
	SerialNo  string     `db:"serial_no"`
	VehicleID *uuid.UUID `db:"vehicle_id"`
	Status    string     `db:"status"`
	KeyHash   string     `db:"api_key_hash"`
	// LastOnlineAt is the value before this request's presence update, so the
	// ingest handler can detect an offline→online transition.
	LastOnlineAt *time.Time `db:"last_online_at"`
}

const (
	HeaderSerial = "X-Device-Serial"
	HeaderKey    = "X-Device-Key"
	ctxDevice    = "telemetry.device"
)

// NewAPIKey generates a device api key and its stored hash.
func NewAPIKey() (key, hash string, err error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	key = "vig_" + base64.RawURLEncoding.EncodeToString(b)
	return key, HashAPIKey(key), nil
}

// HashAPIKey is sha256 hex; keys are high-entropy so no salt/stretching needed.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// LookupDevice loads an active device by serial (nil if unknown).
func LookupDevice(ctx context.Context, db *pgxpool.Pool, serial string) (*Device, error) {
	var d Device
	err := pgxscan.Get(ctx, db, &d, `
		SELECT id, tenant_id, serial_no, vehicle_id, status, api_key_hash, last_online_at
		FROM devices WHERE serial_no = $1 AND deleted_at IS NULL`, serial)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &d, err
}

// RequireDevice authenticates gateway requests with X-Device-Serial / X-Device-Key.
func RequireDevice(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		serial, key := c.GetHeader(HeaderSerial), c.GetHeader(HeaderKey)
		if serial == "" || key == "" {
			httpx.Fail(c, httpx.Unauthorized("missing device credentials"))
			return
		}
		d, err := LookupDevice(c.Request.Context(), db, serial)
		if err != nil {
			httpx.Fail(c, httpx.Internal(err))
			return
		}
		if d == nil || subtle.ConstantTimeCompare([]byte(d.KeyHash), []byte(HashAPIKey(key))) != 1 {
			httpx.Fail(c, httpx.Unauthorized("invalid device credentials"))
			return
		}
		if d.Status != "active" {
			httpx.Fail(c, httpx.Forbidden("device disabled"))
			return
		}
		c.Set(ctxDevice, d)
		// best-effort presence update
		_, _ = db.Exec(c.Request.Context(), `UPDATE devices SET last_online_at = $2, last_ip = $3 WHERE id = $1`, d.ID, time.Now(), c.ClientIP())
		c.Next()
	}
}

// CurrentDevice returns the authenticated device (nil outside ingest routes).
func CurrentDevice(c *gin.Context) *Device {
	if v, ok := c.Get(ctxDevice); ok {
		if d, ok := v.(*Device); ok {
			return d
		}
	}
	return nil
}
