package card

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "asset.card"

// Register mounts /assets/nfc-cards under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/assets/nfc-cards")
	r.GET("", auth.Require("asset:card:view"), h.list)
	r.GET("/:id", auth.Require("asset:card:view"), h.get)
	r.POST("", auth.Require("asset:card:create"), h.create)
	r.PUT("/:id", auth.Require("asset:card:update"), h.update)
	r.DELETE("/:id", auth.Require("asset:card:delete"), h.delete)
	r.POST("/:id/bind", auth.Require("asset:card:update"), h.bind)
	r.POST("/:id/unbind", auth.Require("asset:card:update"), h.unbind)
	r.POST("/:id/report-loss", auth.Require("asset:card:update"), h.reportLoss)
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
	card, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, card)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	card, err := h.svc.Create(c.Request.Context(), auth.TenantID(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "nfc_card", TargetID: card.ID.String(), Summary: "发卡 " + card.CardUID, After: card})
	httpx.Created(c, card)
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
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "nfc_card", TargetID: id.String(), Summary: "编辑卡 " + after.CardUID, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) delete(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	card, err := h.svc.Delete(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "nfc_card", TargetID: id.String(), Summary: "删除卡 " + card.CardUID, Before: card})
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
	before, after, err := h.svc.Bind(c.Request.Context(), auth.TenantID(c), id, req.UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "bind", TargetType: "nfc_card", TargetID: id.String(), Summary: "绑定持卡人 " + after.CardUID, Before: before, After: after})
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
	audit.Record(c, audit.Entry{Module: module, Action: "unbind", TargetType: "nfc_card", TargetID: id.String(), Summary: "解绑持卡人 " + after.CardUID, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) reportLoss(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.ReportLoss(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "report_loss", TargetType: "nfc_card", TargetID: id.String(), Summary: "挂失 " + after.CardUID, Before: before, After: after})
	httpx.OK(c, after)
}
