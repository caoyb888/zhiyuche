package booking

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "booking"

// Register mounts /bookings under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/bookings")
	r.GET("", auth.Require(PermView), h.list)
	r.POST("", auth.Require(PermCreate), h.create)
	r.GET("/available-vehicles", auth.Require(PermCreate), h.availableVehicles)
	r.GET("/:id", auth.Require(PermView), h.get)
	r.PUT("/:id", auth.Require(PermUpdate), h.update)
	r.POST("/:id/depart", auth.Require(PermUpdate), h.depart)
	r.POST("/:id/complete", auth.Require(PermUpdate), h.complete)
	r.POST("/:id/cancel", auth.Require(PermCancel), h.cancel)
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

func (h *handler) get(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	b, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, b)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	b, err := h.svc.Create(c.Request.Context(), auth.TenantID(c), auth.Current(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "booking", TargetID: b.ID.String(),
		Summary: "新建" + SourceLabel(b.Source) + " " + b.BookingNo, After: b})
	httpx.Created(c, b)
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
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "booking", TargetID: id.String(),
		Summary: "编辑预约 " + after.BookingNo, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) depart(c *gin.Context)   { h.transition(c, EvDepart, "确认出车") }
func (h *handler) complete(c *gin.Context) { h.transition(c, EvComplete, "完成预约") }

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
	b, err := h.svc.Transition(c.Request.Context(), auth.TenantID(c), id, EvCancel, req.Reason)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	summary := "取消预约 " + b.BookingNo
	if b.CancelReason != nil {
		summary += "：" + *b.CancelReason
	}
	audit.Record(c, audit.Entry{Module: module, Action: "cancel", TargetType: "booking", TargetID: id.String(), Summary: summary, After: b})
	httpx.OK(c, b)
}

func (h *handler) transition(c *gin.Context, event, label string) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	b, err := h.svc.Transition(c.Request.Context(), auth.TenantID(c), id, event, nil)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: event, TargetType: "booking", TargetID: id.String(),
		Summary: label + " " + b.BookingNo, After: b})
	httpx.OK(c, b)
}

func (h *handler) availableVehicles(c *gin.Context) {
	var q AvailableQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	rows, err := h.svc.AvailableVehicles(c.Request.Context(), auth.TenantID(c), q)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}
