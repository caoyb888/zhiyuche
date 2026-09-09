package device

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "asset.device"

// Register mounts /assets/devices under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/assets/devices")
	r.GET("", auth.Require("asset:device:view"), h.list)
	r.GET("/:id", auth.Require("asset:device:view"), h.get)
	r.POST("", auth.Require("asset:device:create"), h.create)
	r.PUT("/:id", auth.Require("asset:device:update"), h.update)
	r.DELETE("/:id", auth.Require("asset:device:delete"), h.delete)
	r.POST("/:id/bind", auth.Require("asset:device:update"), h.bind)
	r.POST("/:id/unbind", auth.Require("asset:device:update"), h.unbind)
	r.POST("/:id/rotate-key", auth.Require("asset:device:update"), h.rotateKey)
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
	d, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, d)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.Create(c.Request.Context(), auth.TenantID(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	// 审计只记档案，不记明文 key
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "device", TargetID: d.ID.String(), Summary: "新建设备 " + d.SerialNo, After: d.Device})
	httpx.Created(c, d)
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
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "device", TargetID: id.String(), Summary: "编辑设备 " + after.SerialNo, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) delete(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.Delete(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "device", TargetID: id.String(), Summary: "删除设备 " + d.SerialNo, Before: d})
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *handler) bind(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req BindRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.Bind(c.Request.Context(), auth.TenantID(c), id, req.VehicleID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "bind", TargetType: "device", TargetID: id.String(), Summary: "绑定车辆 " + after.SerialNo, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) unbind(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.Unbind(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "unbind", TargetType: "device", TargetID: id.String(), Summary: "解绑车辆 " + after.SerialNo, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) rotateKey(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.RotateKey(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "rotate_key", TargetType: "device", TargetID: id.String(), Summary: "重置设备密钥 " + d.SerialNo})
	httpx.OK(c, d)
}
