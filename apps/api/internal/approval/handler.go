package approval

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "approval"

// Register mounts /approvals under an authenticated group.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/approvals")
	r.GET("", auth.Require(PermView), h.list)
	r.POST("", auth.Require(PermCreate), h.create)
	r.POST("/precheck", auth.Require(PermCreate), h.precheck)
	r.GET("/todo-count", auth.Require(PermView), h.todoCount)
	r.GET("/available-vehicles", auth.Require(PermCreate), h.availableVehicles)
	r.GET("/rules", auth.Require(PermRule), h.getRules)
	r.PUT("/rules", auth.Require(PermRule), h.setRules)
	r.GET("/:id", auth.Require(PermView), h.get)
	r.POST("/:id/approve", auth.Require(PermApprove), h.approve)
	r.POST("/:id/reject", auth.Require(PermApprove), h.reject)
	r.POST("/:id/cancel", auth.Require(PermView), h.cancel)
}

type handler struct{ svc *Service }

func (h *handler) list(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.List(c.Request.Context(), auth.TenantID(c), auth.Current(c), q, pagination.Parse(c))
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
	a, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), auth.Current(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, a)
}

func (h *handler) precheck(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Skip(c)
	res, err := h.svc.Precheck(c.Request.Context(), auth.TenantID(c), auth.Current(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *handler) create(c *gin.Context) {
	var req CreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	a, err := h.svc.Create(c.Request.Context(), auth.TenantID(c), auth.Current(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "approval", TargetID: a.ID.String(), Summary: "发起用车申请 " + a.ApplyNo, After: a})
	httpx.Created(c, a)
}

func (h *handler) approve(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req ApproveRequest
	if c.Request.ContentLength != 0 {
		if err := httpx.BindJSON(c, &req); err != nil {
			httpx.Fail(c, err)
			return
		}
	}
	a, err := h.svc.Approve(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, req, c.ClientIP())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "approve", TargetType: "approval", TargetID: id.String(), Summary: "审批通过 " + a.ApplyNo + " → " + a.Status, After: a})
	httpx.OK(c, a)
}

func (h *handler) reject(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req RejectRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	a, err := h.svc.Reject(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, req.Reason, c.ClientIP())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "reject", TargetType: "approval", TargetID: id.String(), Summary: "驳回 " + a.ApplyNo + "：" + req.Reason, After: a})
	httpx.OK(c, a)
}

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
	a, err := h.svc.Cancel(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, req.Reason)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "cancel", TargetType: "approval", TargetID: id.String(), Summary: "撤销 " + a.ApplyNo, After: a})
	httpx.OK(c, a)
}

func (h *handler) todoCount(c *gin.Context) {
	n, err := h.svc.TodoCount(c.Request.Context(), auth.TenantID(c), auth.Current(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"todo": n})
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

func (h *handler) getRules(c *gin.Context) {
	r, err := h.svc.GetRules(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, r)
}

func (h *handler) setRules(c *gin.Context) {
	var req RulesUpdate
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, _ := h.svc.GetRules(c.Request.Context(), auth.TenantID(c))
	r, err := h.svc.SetRules(c.Request.Context(), auth.TenantID(c), auth.Current(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "set_rules", TargetType: "approval_rules", TargetID: auth.TenantID(c).String(), Summary: "保存审批规则", Before: before, After: r})
	httpx.OK(c, r)
}
