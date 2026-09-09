package telemetry

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vstatus"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Handler serves the gateway ingest routes.
type Handler struct {
	app   *app.App
	vs    *vstatus.Store
	hooks TripHooks
}

// Register mounts the gateway ingest routes under /ingest (device auth, no JWT).
//
//	POST /ingest/telemetry  body: {"points":[Point,...]}  → stores vehicle_telemetry, updates vstatus, calls hooks.OnTelemetry
//	POST /ingest/events     body: {"type":"trip_start", "trip_start":{...}} | {"type":"trip_end", "trip_end":{...}}
func Register(g *gin.RouterGroup, a *app.App, hooks TripHooks) {
	h := &Handler{app: a, vs: vstatus.New(a), hooks: hooks}
	r := g.Group("/ingest", RequireDevice(a.DB))
	r.POST("/telemetry", h.telemetry)
	r.POST("/events", h.events)
}

var telemetryColumns = []string{
	"ts", "tenant_id", "vehicle_id", "device_id", "lng", "lat", "speed", "heading", "soc", "soh",
	"cell_temp", "motor_temp", "odometer_km", "acc_on", "locked", "sign_on", "charging",
}

func (h *Handler) telemetry(c *gin.Context) {
	audit.Skip(c) // 高频上报不进审计日志
	dev := CurrentDevice(c)
	if dev == nil {
		httpx.Fail(c, httpx.Unauthorized("unauthenticated device"))
		return
	}
	var req TelemetryRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if dev.VehicleID == nil {
		httpx.Fail(c, ErrNotBound)
		return
	}
	ctx := c.Request.Context()
	vehicleID := *dev.VehicleID
	var rangeFull int
	err := h.app.DB.QueryRow(ctx, `SELECT range_km_full FROM vehicles WHERE id = $1 AND deleted_at IS NULL`, vehicleID).Scan(&rangeFull)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Fail(c, ErrNotBound)
		return
	}
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}

	pts := SortPoints(req.Points)
	n, err := h.insert(ctx, dev, vehicleID, pts)
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	st, err := h.vs.ApplyTelemetry(ctx, vehicleID, float64(rangeFull), toSnapshot(pts[len(pts)-1]))
	if err != nil {
		httpx.Fail(c, httpx.Internal(err))
		return
	}
	if err := h.hooks.OnTelemetry(ctx, *dev, vehicleID, pts); err != nil {
		h.app.Log.Warn().Err(err).Str("device", dev.SerialNo).Msg("trip hook OnTelemetry failed")
	}
	if !WasOnline(dev.LastOnlineAt, time.Now()) {
		h.app.Hub.Publish(dev.TenantID, ws.Event{Type: ws.EvDeviceOnline, Data: gin.H{
			"device_id": dev.ID, "vehicle_id": vehicleID, "online": true,
		}})
	}
	res := TelemetryResult{Accepted: int(n)}
	if st != nil {
		res.TripID = st.CurrentTripID
	}
	httpx.OK(c, res)
}

// insert bulk-loads the points into the vehicle_telemetry hypertable.
func (h *Handler) insert(ctx context.Context, dev *Device, vehicleID uuid.UUID, pts []Point) (int64, error) {
	rows := make([][]any, len(pts))
	for i, p := range pts {
		rows[i] = []any{p.TS, dev.TenantID, vehicleID, dev.ID, p.Lng, p.Lat, p.Speed, p.Heading, p.SOC, p.SOH,
			p.CellTemp, p.MotorTemp, p.OdometerKm, p.AccOn, p.Locked, p.SignOn, p.Charging}
	}
	return h.app.DB.CopyFrom(ctx, pgx.Identifier{"vehicle_telemetry"}, telemetryColumns, pgx.CopyFromRows(rows))
}

func (h *Handler) events(c *gin.Context) {
	dev := CurrentDevice(c)
	if dev == nil {
		httpx.Fail(c, httpx.Unauthorized("unauthenticated device"))
		return
	}
	var req EventRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := HandleEvent(c.Request.Context(), h.hooks, *dev, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{
		Module: "ingest", Action: req.Type, TargetType: "device", TargetID: dev.ID.String(),
		Summary: "网关 " + dev.SerialNo + " 上报 " + req.Type, After: res,
		ActorName: "device:" + dev.SerialNo, TenantID: dev.TenantID,
	})
	httpx.OK(c, res)
}
