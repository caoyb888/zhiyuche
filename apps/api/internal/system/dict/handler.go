package dict

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "system.dict"

// Register mounts /system/dict-types, /system/dict-items and /system/dicts/{code}.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	t := g.Group("/system/dict-types")
	t.GET("", auth.Require("system:dict:view"), h.list)
	t.GET("/:id", auth.Require("system:dict:view"), h.get)
	t.POST("", auth.Require("system:dict:create"), h.create)
	t.PUT("/:id", auth.Require("system:dict:update"), h.update)
	t.DELETE("/:id", auth.Require("system:dict:delete"), h.delete)
	t.POST("/:id/items", auth.Require("system:dict:update"), h.createItem)

	i := g.Group("/system/dict-items")
	i.PUT("/:id", auth.Require("system:dict:update"), h.updateItem)
	i.DELETE("/:id", auth.Require("system:dict:update"), h.deleteItem)

	// 任意登录用户可读
	g.GET("/system/dicts/:code", h.byCode)
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
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "dict_type", TargetID: t.ID.String(), Summary: "新建字典 " + t.Code, After: t})
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
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "dict_type", TargetID: id.String(), Summary: "编辑字典 " + after.Code, Before: before, After: after})
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
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "dict_type", TargetID: id.String(), Summary: "删除字典 " + t.Code, Before: t})
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *handler) createItem(c *gin.Context) {
	typeID, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req ItemCreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, it, err := h.svc.CreateItem(c.Request.Context(), common.FromContext(c), typeID, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "dict_item", TargetID: it.ID.String(), Summary: "新增字典条目 " + t.Code + "/" + it.Value, After: it})
	httpx.Created(c, it)
}

func (h *handler) updateItem(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req ItemUpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.UpdateItem(c.Request.Context(), common.FromContext(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "dict_item", TargetID: id.String(), Summary: "编辑字典条目 " + after.Value, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) deleteItem(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	it, err := h.svc.DeleteItem(c.Request.Context(), common.FromContext(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "dict_item", TargetID: id.String(), Summary: "删除字典条目 " + it.Value, Before: it})
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *handler) byCode(c *gin.Context) {
	items, err := h.svc.ByCode(c.Request.Context(), auth.TenantID(c), c.Param("code"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
