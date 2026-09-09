package template

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "system.template"

// Register mounts /system/notification-templates.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/system/notification-templates")
	r.GET("", auth.Require("system:template:view"), h.list)
	r.GET("/:id", auth.Require("system:template:view"), h.get)
	r.POST("", auth.Require("system:template:create"), h.create)
	r.PUT("/:id", auth.Require("system:template:update"), h.update)
	r.DELETE("/:id", auth.Require("system:template:delete"), h.delete)
}

type handler struct{ svc *Service }

func (h *handler) list(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.List(c.Request.Context(), common.FromContext(c), q, pagination.Parse(c))
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
	t, err := h.svc.Get(c.Request.Context(), common.FromContext(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), common.FromContext(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "notification_template", TargetID: t.ID.String(), Summary: "新建通知模板 " + t.Code + "/" + t.Channel, After: t})
	httpx.Created(c, t)
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
	before, after, err := h.svc.Update(c.Request.Context(), common.FromContext(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "notification_template", TargetID: id.String(), Summary: "编辑通知模板 " + after.Code + "/" + after.Channel, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) delete(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Delete(c.Request.Context(), common.FromContext(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "notification_template", TargetID: id.String(), Summary: "删除通知模板 " + t.Code + "/" + t.Channel, Before: t})
	httpx.OK(c, gin.H{"deleted": true})
}
