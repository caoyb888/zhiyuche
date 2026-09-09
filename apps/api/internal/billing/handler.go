// Package billing owns billing rules, the three-level account model
// (enterprise → department → employee), account transactions, automatic
// trip/charging deductions and monthly settlements. The calculator itself is
// internal/billing/engine.
package billing

import (
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/billing/engine"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "billing"

// Register mounts /billing/rules, /billing/accounts, /billing/transactions, /billing/settlements.
func Register(g *gin.RouterGroup, a *app.App) {
	h := &handler{svc: NewService(a)}
	r := g.Group("/billing")

	rules := r.Group("/rules")
	rules.GET("", auth.Require("billing:rule:view"), h.listRules)
	rules.POST("", auth.Require("billing:rule:update"), h.createRule)
	rules.GET("/template", auth.Require("billing:rule:view"), h.template)
	rules.POST("/simulate", auth.Require("billing:rule:view"), h.simulate)
	rules.GET("/:id", auth.Require("billing:rule:view"), h.getRule)
	rules.PUT("/:id", auth.Require("billing:rule:update"), h.updateRule)
	rules.DELETE("/:id", auth.Require("billing:rule:update"), h.deleteRule)
	rules.POST("/:id/activate", auth.Require("billing:rule:update"), h.activateRule)

	acc := r.Group("/accounts")
	acc.GET("/tree", auth.Require("billing:account:view"), h.tree)
	acc.GET("", auth.Require("billing:account:view"), h.listAccounts)
	acc.GET("/me", h.me) // 任何登录用户
	acc.POST("/recharge", auth.Require("billing:account:recharge"), h.recharge)
	acc.GET("/:id", auth.Require("billing:account:view"), h.getAccount)
	acc.PUT("/:id", auth.Require("billing:account:adjust"), h.updateAccount)
	acc.POST("/:id/allocate", auth.Require("billing:account:allocate"), h.allocate)
	acc.POST("/:id/adjust", auth.Require("billing:account:adjust"), h.adjust)
	acc.GET("/:id/transactions", auth.Require("billing:account:view"), h.accountTxns)
	r.GET("/transactions", auth.Require("billing:account:view"), h.txns)

	st := r.Group("/settlements")
	st.GET("", auth.Require("billing:settlement:view"), h.listSettlements)
	st.GET("/periods", auth.Require("billing:settlement:view"), h.periods)
	st.POST("/generate", auth.Require("billing:settlement:generate"), h.generate)
	st.GET("/export", auth.Require("billing:settlement:export"), h.export)
	st.GET("/:id", auth.Require("billing:settlement:view"), h.getSettlement)
	st.POST("/:id/confirm", auth.Require("billing:settlement:confirm"), h.confirm)
}

type handler struct{ svc *Service }

func paramID(c *gin.Context) (uuid.UUID, bool) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}

// ---- rules ----

func (h *handler) listRules(c *gin.Context) {
	rows, err := h.svc.ListRules(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}

func (h *handler) template(c *gin.Context) { httpx.OK(c, engine.Default()) }

func (h *handler) simulate(c *gin.Context) {
	var req SimulateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := h.svc.Simulate(c.Request.Context(), auth.TenantID(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

func (h *handler) getRule(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	r, err := h.svc.GetRule(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, r)
}

func (h *handler) createRule(c *gin.Context) {
	var req RuleCreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	r, err := h.svc.CreateRule(c.Request.Context(), auth.TenantID(c), req, auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "create", TargetType: "billing_rule", TargetID: r.ID.String(), Summary: "新建计费规则 " + r.Name, After: r})
	httpx.Created(c, r)
}

func (h *handler) updateRule(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req RuleUpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.UpdateRule(c.Request.Context(), auth.TenantID(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "billing_rule", TargetID: id.String(), Summary: "编辑计费规则 " + after.Name, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) deleteRule(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	r, err := h.svc.DeleteRule(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "delete", TargetType: "billing_rule", TargetID: id.String(), Summary: "删除计费规则 " + r.Name, Before: r})
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *handler) activateRule(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	r, err := h.svc.ActivateRule(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "activate", TargetType: "billing_rule", TargetID: id.String(), Summary: "设为生效规则 " + r.Name, After: r})
	httpx.OK(c, r)
}

// ---- accounts ----

func (h *handler) tree(c *gin.Context) {
	root, err := h.svc.AccountTree(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, root)
}

func (h *handler) listAccounts(c *gin.Context) {
	var q AccountListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.ListAccounts(c.Request.Context(), auth.TenantID(c), q, pagination.Parse(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, page)
}

func (h *handler) me(c *gin.Context) {
	out, err := h.svc.MyAccount(c.Request.Context(), auth.TenantID(c), auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, out)
}

func (h *handler) recharge(c *gin.Context) {
	var req RechargeRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Recharge(c.Request.Context(), auth.TenantID(c), req, auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "recharge", TargetType: "account", TargetID: t.AccountID.String(), Summary: fmt.Sprintf("企业账户充值 ¥%.2f", req.Amount), After: t})
	httpx.OK(c, t)
}

func (h *handler) getAccount(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	a, err := h.svc.GetAccount(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, a)
}

func (h *handler) updateAccount(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req AccountUpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	before, after, err := h.svc.UpdateAccount(c.Request.Context(), auth.TenantID(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "update", TargetType: "account", TargetID: id.String(), Summary: "编辑账户 " + after.OwnerName, Before: before, After: after})
	httpx.OK(c, after)
}

func (h *handler) allocate(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req AllocateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	out, err := h.svc.Allocate(c.Request.Context(), auth.TenantID(c), id, req, auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "allocate", TargetType: "account", TargetID: id.String(),
		Summary: fmt.Sprintf("划拨 ¥%.2f → %s", req.Amount, out.To.AccountName), After: out})
	httpx.OK(c, out)
}

func (h *handler) adjust(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var req AdjustRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Adjust(c.Request.Context(), auth.TenantID(c), id, req, auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "adjust", TargetType: "account", TargetID: id.String(), Summary: fmt.Sprintf("余额调整 %+.2f：%s", req.Amount, req.Remark), After: t})
	httpx.OK(c, t)
}

func (h *handler) accountTxns(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var q TxnListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.ListAccountTransactions(c.Request.Context(), auth.TenantID(c), id, q, pagination.Parse(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, page)
}

func (h *handler) txns(c *gin.Context) {
	var q TxnListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	page, err := h.svc.ListTransactions(c.Request.Context(), auth.TenantID(c), q, pagination.Parse(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, page)
}

// ---- settlements ----

func (h *handler) listSettlements(c *gin.Context) {
	var q SettlementListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	rows, err := h.svc.ListSettlements(c.Request.Context(), auth.TenantID(c), q)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}

func (h *handler) periods(c *gin.Context) {
	rows, err := h.svc.Periods(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}

func (h *handler) generate(c *gin.Context) {
	var req GenerateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	rows, err := h.svc.Generate(c.Request.Context(), auth.TenantID(c), req.Period)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "generate", TargetType: "settlement", TargetID: req.Period, Summary: "生成结算单 " + req.Period})
	httpx.OK(c, rows)
}

func (h *handler) getSettlement(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	st, err := h.svc.GetSettlement(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, st)
}

func (h *handler) confirm(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	st, err := h.svc.Confirm(c.Request.Context(), auth.TenantID(c), id, auth.Current(c).UserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "confirm", TargetType: "settlement", TargetID: id.String(), Summary: "确认结算单 " + st.Period + " " + deptLabel(*st), After: st})
	httpx.OK(c, st)
}

func (h *handler) export(c *gin.Context) {
	var q ExportQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	if _, _, err := periodRange(q.Period); err != nil {
		httpx.Fail(c, httpx.BadRequest(err.Error()))
		return
	}
	var deptID *uuid.UUID
	if q.DeptID != "" {
		id, err := uuid.Parse(q.DeptID)
		if err != nil {
			httpx.Fail(c, httpx.BadRequest("dept_id 须为 uuid"))
			return
		}
		deptID = &id
	}
	rows, lines, err := h.svc.ExportData(c.Request.Context(), auth.TenantID(c), q.Period, deptID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	f, err := buildExport(q.Period, rows, lines)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	name := exportFilename(q.Period)
	c.Header("Content-Type", xlsxMIME)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, name, url.PathEscape(name)))
	c.Status(200)
	if err := f.Write(c.Writer); err != nil {
		_ = c.Error(err)
	}
}
