package vehicle

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "asset.vehicle"

// Register mounts /assets/vehicles under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/assets/vehicles")
	r.GET("", auth.Require("asset:vehicle:view"), h.list)
	r.GET("/options", auth.Require("asset:vehicle:view"), h.options)
	r.GET("/status", auth.Require("asset:vehicle:view"), h.liveAll)
	r.GET("/:id", auth.Require("asset:vehicle:view"), h.get)
	r.GET("/:id/status", auth.Require("asset:vehicle:view"), h.live)
	r.GET("/:id/telemetry", auth.Require("asset:vehicle:view"), h.telemetry)
	r.POST("", auth.Require("asset:vehicle:create"), h.create)
	r.PUT("/:id", auth.Require("asset:vehicle:update"), h.update)
	r.DELETE("/:id", auth.Require("asset:vehicle:delete"), h.delete)
}

type handler struct{ svc *Service }

func (h *handler) list(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.List(c.Request.Context(), auth.TenantID(c), q, pagination.Parse(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, page)
}

func (h *handler) options(c *gin.Context) {
	var q struct {
		Status string `form:"status" binding:"omitempty,oneof=idle in_use charging maintenance disabled"`
	}
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	rows, err := h.svc.Options(c.Request.Context(), auth.TenantID(c), q.Status)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}

func (h *handler) liveAll(c *gin.Context) {
	rows, err := h.svc.LiveAll(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}

func (h *handler) get(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	v, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, v)
}

func (h *handler) live(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	live, err := h.svc.Live(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, live)
}

func (h *handler) telemetry(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	from, err1 := time.Parse(time.RFC3339, c.Query("from"))
	to, err2 := time.Parse(time.RFC3339, c.Query("to"))
	if err1 != nil || err2 != nil {
		httpx.Fail(c, httpx.BadRequest("from and to are required (RFC3339)"))
		return
	}
	limit, err := ParseTelemetryLimit(c.Query("limit"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	pts, err := h.svc.Telemetry(c.Request.Context(), auth.TenantID(c), id, from, to, limit)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, pts)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	v, err := h.svc.Create(c.Request.Context(), auth.TenantID(c), req, auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "vehicle", TargetID: v.ID.String(), Summary: "新建车辆 " + v.PlateNo, After: v})
	httpx.Created(c, v)
}

func (h *handler) update(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req UpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.Update(c.Request.Context(), auth.TenantID(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "vehicle", TargetID: id.String(), Summary: "编辑车辆 " + after.PlateNo, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) delete(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	v, err := h.svc.Delete(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "vehicle", TargetID: id.String(), Summary: "删除车辆 " + v.PlateNo, Before: v})
	httpx.OK(c, gin.H{"deleted": true})
}
