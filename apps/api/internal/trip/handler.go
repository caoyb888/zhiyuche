package trip

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/telemetry"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "trip"

// Register mounts /trips and /dashboard/overview.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/trips")
	r.GET("", auth.Require(PermView), h.list)
	r.POST("/start", auth.Require(PermManage), h.start)
	r.GET("/summary", auth.Require(PermView), h.summary)
	r.GET("/export", auth.Require(PermExport), h.export)
	r.GET("/:id", auth.Require(PermView), h.get)
	r.GET("/:id/track", auth.Require(PermView), h.track)
	r.GET("/:id/events", auth.Require(PermView), h.events)
	r.POST("/:id/end", auth.Require(PermManage), h.end)
	r.POST("/:id/cancel", auth.Require(PermManage), h.cancel)
	g.GET("/dashboard/overview", auth.Require(PermDashboard), h.overview)
}

// NewDeviceHooks returns the telemetry.TripHooks implementation.
func NewDeviceHooks(a *app.App) telemetry.TripHooks { return &deviceHooks{svc: NewService(a)} }

type deviceHooks struct{ svc *Service }

func (h *deviceHooks) DeviceTripStart(ctx context.Context, dev telemetry.Device, ev telemetry.TripStartEvent) (*telemetry.TripStartResult, error) {
	return h.svc.StartFromDevice(ctx, dev, ev)
}

func (h *deviceHooks) DeviceTripEnd(ctx context.Context, dev telemetry.Device, ev telemetry.TripEndEvent) (uuid.UUID, error) {
	return h.svc.EndFromDevice(ctx, dev, ev)
}

func (h *deviceHooks) OnTelemetry(ctx context.Context, dev telemetry.Device, vehicleID uuid.UUID, pts []telemetry.Point) error {
	return h.svc.OnTelemetry(ctx, dev.TenantID, vehicleID, pts)
}

type handler struct{ svc *Service }

func (h *handler) list(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.List(c.Request.Context(), auth.TenantID(c), auth.Current(c), q, pagination.Parse(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, page)
}

func (h *handler) get(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), auth.Current(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *handler) track(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	step := 1
	if v := c.Query("step"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			httpx.Fail(c, httpx.BadRequest("step 须为 ≥1 的整数"))
			return
		}
		step = n
	}
	tr, err := h.svc.Track(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, step)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, tr)
}

func (h *handler) events(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	evs, err := h.svc.Events(c.Request.Context(), auth.TenantID(c), auth.Current(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, evs)
}

func (h *handler) start(c *gin.Context) {
	var req StartRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Start(c.Request.Context(), auth.TenantID(c), auth.Current(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "start", TargetType: "trip", TargetID: t.ID.String(), Summary: "手动开始行程 " + t.TripNo + " 车辆 " + t.Vehicle.PlateNo, After: t})
	httpx.Created(c, t)
}

func (h *handler) end(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req EndRequest
	if c.Request.ContentLength != 0 {
		if err := httpx.BindJSON(c, &req); err != nil {
			httpx.Fail(c, err)
			return
		}
	}
	t, err := h.svc.EndWeb(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "end", TargetType: "trip", TargetID: id.String(), Summary: "手动结束行程 " + t.TripNo, After: t})
	httpx.OK(c, t)
}

func (h *handler) cancel(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req CancelRequest
	if c.Request.ContentLength != 0 {
		if err := httpx.BindJSON(c, &req); err != nil {
			httpx.Fail(c, err)
			return
		}
	}
	t, err := h.svc.Cancel(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, req.Reason)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "cancel", TargetType: "trip", TargetID: id.String(), Summary: "作废行程 " + t.TripNo, After: t})
	httpx.OK(c, t)
}

func (h *handler) summary(c *gin.Context) {
	sm, err := h.svc.Summary(c.Request.Context(), auth.TenantID(c), c.Query("date"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sm)
}

func (h *handler) overview(c *gin.Context) {
	ov, err := h.svc.Overview(c.Request.Context(), auth.TenantID(c), auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, ov)
}

func (h *handler) export(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	if q.Scope == "" {
		q.Scope = "all"
	}
	trips, err := h.svc.Export(c.Request.Context(), auth.TenantID(c), auth.Current(c), q)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	f, err := buildExport(trips)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	buf, err := f.WriteToBuffer()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="trips-`+time.Now().Format("20060102-150405")+`.xlsx"`)
	c.Data(200, xlsxMIME, buf.Bytes())
}
