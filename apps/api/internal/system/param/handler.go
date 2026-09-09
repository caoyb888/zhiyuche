package param

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/common"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

const module = "system.param"

// Register mounts /system/params.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/system/params")
	r.GET("", auth.Require("system:param:view"), h.list)
	r.PUT("/:key", auth.Require("system:param:update"), h.set)
	r.DELETE("/:key", auth.Require("system:param:update"), h.reset)
}

type handler struct{ svc *Service }

func (h *handler) list(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	items, err := h.svc.List(c.Request.Context(), common.FromContext(c), q)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}

func (h *handler) set(c *gin.Context) {
	var req UpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	key := c.Param("key")
	sc := common.FromContext(c)
	before, after, err := h.svc.Set(c.Request.Context(), sc, key, req, auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	summary := "设置参数 " + key
	if sc.Global {
		summary = "设置全局参数 " + key
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "param", TargetID: key, Summary: summary, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) reset(c *gin.Context) {
	key := c.Param("key")
	before, err := h.svc.Reset(c.Request.Context(), common.FromContext(c), key)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "param", TargetID: key, Summary: "恢复参数缺省值 " + key, Before: before})
	httpx.OK(c, gin.H{"deleted": true})
}
