package dept

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

const module = "system.dept"

// Register mounts /system/depts under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/system/depts")
	r.GET("/tree", auth.Require("system:dept:view"), h.tree)
	r.GET("", auth.Require("system:dept:view"), h.list)
	r.GET("/:id", auth.Require("system:dept:view"), h.get)
	r.POST("", auth.Require("system:dept:create"), h.create)
	r.PUT("/:id", auth.Require("system:dept:update"), h.update)
	r.DELETE("/:id", auth.Require("system:dept:delete"), h.delete)
}

type handler struct{ svc *Service }

func (h *handler) tree(c *gin.Context) {
	tree, err := h.svc.Tree(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, tree)
}

func (h *handler) list(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	rows, err := h.svc.List(c.Request.Context(), auth.TenantID(c), q)
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
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "dept", TargetID: d.ID.String(), Summary: "新建部门 " + d.Name, After: d})
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
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "dept", TargetID: id.String(), Summary: "编辑部门 " + after.Name, Before: before, After: after})
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
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "dept", TargetID: id.String(), Summary: "删除部门 " + d.Name, Before: d})
	httpx.OK(c, gin.H{"deleted": true})
}
