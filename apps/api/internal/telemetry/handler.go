package telemetry

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Register mounts the gateway ingest routes under /ingest (device auth, no JWT).
// Implemented in phase 2 by the asset/telemetry work package:
//
//	POST /ingest/telemetry  body: {"points":[Point,...]}  → stores vehicle_telemetry, updates vstatus, calls hooks.OnTelemetry
//	POST /ingest/events     body: {"type":"trip_start", "trip_start":{...}} | {"type":"trip_end", "trip_end":{...}}
func Register(g *gin.RouterGroup, a *app.App, hooks TripHooks) {
	r := g.Group("/ingest", RequireDevice(a.DB))
	r.POST("/telemetry", notImplemented)
	r.POST("/events", notImplemented)
}

func notImplemented(c *gin.Context) {
	httpx.Fail(c, &httpx.AppError{Code: httpx.CodeInternal, Status: 501, Message: "not implemented"})
}
