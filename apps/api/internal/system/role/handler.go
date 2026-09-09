package role

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "system.role"

// Register mounts /system/roles and /system/permissions under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/system/roles")
	r.GET("", auth.Require("system:role:view"), h.list)
	r.GET("/options", auth.Require("system:user:view"), h.options)
	r.GET("/:id", auth.Require("system:role:view"), h.get)
	r.POST("", auth.Require("system:role:create"), h.create)
	r.PUT("/:id", auth.Require("system:role:update"), h.update)
	r.DELETE("/:id", auth.Require("system:role:delete"), h.delete)
	r.PUT("/:id/permissions", auth.Require("system:role:assign-perms"), h.setPermissions)

	g.GET("/system/permissions/tree", auth.Require("system:permission:view"), h.permissionTree)
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
	rows, err := h.svc.Options(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}

func (h *handler) permissionTree(c *gin.Context) {
	tree, err := h.svc.PermissionTree(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, tree)
}

func (h *handler) get(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	r, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, r)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	r, err := h.svc.Create(c.Request.Context(), auth.TenantID(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "role", TargetID: r.ID.String(), Summary: "新建角色 " + r.Code, After: r})
	httpx.Created(c, r)
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
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "role", TargetID: id.String(), Summary: "编辑角色 " + after.Code, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) delete(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	r, err := h.svc.Delete(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "role", TargetID: id.String(), Summary: "删除角色 " + r.Code, Before: r})
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *handler) setPermissions(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req SetPermissionsRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.SetPermissions(c.Request.Context(), auth.TenantID(c), id, req.Permissions)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "assign_perms", TargetType: "role", TargetID: id.String(), Summary: "设置角色权限 " + after.Code, Before: before.Permissions, After: after.Permissions})
	httpx.OK(c, after)
}
