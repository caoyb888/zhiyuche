// Package trip owns trips, their track points and events, the device-side
// start/end hooks, and the dashboard overview. Implemented in phase 2.
package trip

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/telemetry"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Register mounts /trips and /dashboard/overview.
func Register(g *gin.RouterGroup, a *app.App) {}

// NewDeviceHooks returns the telemetry.TripHooks implementation.
func NewDeviceHooks(a *app.App) telemetry.TripHooks { return &deviceHooks{app: a} }

type deviceHooks struct{ app *app.App }

func (h *deviceHooks) DeviceTripStart(ctx context.Context, dev telemetry.Device, ev telemetry.TripStartEvent) (*telemetry.TripStartResult, error) {
	return nil, httpx.Internal(errNotImplemented)
}

func (h *deviceHooks) DeviceTripEnd(ctx context.Context, dev telemetry.Device, ev telemetry.TripEndEvent) (uuid.UUID, error) {
	return uuid.Nil, httpx.Internal(errNotImplemented)
}

func (h *deviceHooks) OnTelemetry(ctx context.Context, dev telemetry.Device, vehicleID uuid.UUID, pts []telemetry.Point) error {
	return nil
}

var errNotImplemented = &httpx.AppError{Code: httpx.CodeInternal, Status: 501, Message: "not implemented"}
