package tenant

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "system.tenant"

// Register mounts /system/tenants under an authenticated group. The
// system:tenant:* codes are platform-only, so only the platform tenant's roles
// (in practice the super admin) can pass these guards.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/system/tenants")
	r.GET("", auth.Require("system:tenant:view"), h.list)
	r.GET("/:id", auth.Require("system:tenant:view"), h.get)
	r.POST("", auth.Require("system:tenant:create"), h.create)
	r.PUT("/:id", auth.Require("system:tenant:update"), h.update)
	r.DELETE("/:id", auth.Require("system:tenant:delete"), h.disable)
}

type handler struct{ svc *Service }

func (h *handler) list(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.List(c.Request.Context(), q, pagination.Parse(c))
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
	t, err := h.svc.Get(c.Request.Context(), id)
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
	t, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	// After 只记租户本身，不落管理员密码
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "tenant", TargetID: t.ID.String(), Summary: "新建租户 " + t.Code + "（管理员 " + req.AdminUsername + "）", After: t})
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
	before, after, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "tenant", TargetID: id.String(), Summary: "编辑租户 " + after.Code, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) disable(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.Disable(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "disable", TargetType: "tenant", TargetID: id.String(), Summary: "停用租户 " + after.Code, Before: before, After: after})
	httpx.OK(c, gin.H{"disabled": true})
}
