package user

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "system.user"

// Register mounts /system/users under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/system/users")
	r.GET("", auth.Require("system:user:view"), h.list)
	r.GET("/:id", auth.Require("system:user:view"), h.get)
	r.POST("", auth.Require("system:user:create"), h.create)
	r.PUT("/:id", auth.Require("system:user:update"), h.update)
	r.DELETE("/:id", auth.Require("system:user:delete"), h.delete)
	r.POST("/:id/reset-password", auth.Require("system:user:reset-password"), h.resetPassword)
	r.PUT("/:id/roles", auth.Require("system:user:assign-roles"), h.assignRoles)
	// /system/users/import 与 /export 由 importexport.go 提供（阶段 1 后半）
	registerImportExport(r, h)
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
	u, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, u)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	u, err := h.svc.Create(c.Request.Context(), auth.TenantID(c), req, auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "user", TargetID: u.ID.String(), Summary: "新建用户 " + u.Username, After: u})
	httpx.Created(c, u)
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
	before, after, err := h.svc.Update(c.Request.Context(), auth.TenantID(c), id, req, auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "user", TargetID: id.String(), Summary: "编辑用户 " + after.Username, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) delete(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	u, err := h.svc.Delete(c.Request.Context(), auth.TenantID(c), id, auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "user", TargetID: id.String(), Summary: "删除用户 " + u.Username, Before: u})
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *handler) resetPassword(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req ResetPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	pw, err := h.svc.ResetPassword(c.Request.Context(), auth.TenantID(c), id, req, auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "reset_password", TargetType: "user", TargetID: id.String(), Summary: "重置密码"})
	httpx.OK(c, ResetPasswordResponse{Password: pw})
}

func (h *handler) assignRoles(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req AssignRolesRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.AssignRoles(c.Request.Context(), auth.TenantID(c), id, req.RoleIDs, auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "assign_roles", TargetType: "user", TargetID: id.String(), Summary: "分配角色", Before: before.Roles, After: after.Roles})
	httpx.OK(c, after)
}
