package charging

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/pagination"
)

const module = "charging"

// Register mounts /charging/* (live piles, transactions, review, remote start/stop, summary, export).
func Register(g *gin.RouterGroup, a *app.App, billing BillingHook) {
	h := &handler{svc: NewService(a, billing)}
	r := g.Group("/charging")
	r.GET("/piles/live", auth.Require(PermView), h.live)
	r.POST("/piles/:id/remote-start", auth.Require(PermManage), h.remoteStart)
	r.POST("/piles/:id/remote-stop", auth.Require(PermManage), h.remoteStop)
	r.GET("/transactions", auth.Require(PermView), h.list)
	r.GET("/transactions/export", auth.Require(PermExport), h.export)
	r.GET("/transactions/:id", auth.Require(PermView), h.get)
	r.GET("/transactions/:id/meter-values", auth.Require(PermView), h.meterValues)
	r.POST("/transactions/:id/review", auth.Require(PermReview), h.review)
	r.GET("/summary", auth.Require(PermView), h.summary)
}

type handler struct{ svc *Service }

func (h *handler) live(c *gin.Context) {
	piles, err := h.svc.ListLive(c.Request.Context(), auth.TenantID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, piles)
}

func (h *handler) remoteStart(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req RemoteStartRequest
	if c.Request.ContentLength != 0 {
		if err := httpx.BindJSON(c, &req); err != nil {
			httpx.Fail(c, err)
			return
		}
	}
	res, err := h.svc.RemoteStart(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	conn := 1
	if req.ConnectorID != nil {
		conn = *req.ConnectorID
	}
	audit.Record(c, audit.Entry{Module: module, Action: "remote_start", TargetType: "charge_pile", TargetID: id.String(),
		Summary: fmt.Sprintf("远程启动充电 %d 号枪 → %s", conn, res.Status), After: res})
	httpx.OK(c, res)
}

func (h *handler) remoteStop(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req RemoteStopRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := h.svc.RemoteStop(c.Request.Context(), auth.TenantID(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	audit.Record(c, audit.Entry{Module: module, Action: "remote_stop", TargetType: "charge_transaction", TargetID: req.TransactionID.String(),
		Summary: "远程停止充电 → " + res.Status, After: res})
	httpx.OK(c, res)
}

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
	t, err := h.svc.Get(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *handler) meterValues(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	pts, err := h.svc.MeterValuesOf(c.Request.Context(), auth.TenantID(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, pts)
}

func (h *handler) review(c *gin.Context) {
	id, err := httpx.ParamUUID(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req ReviewRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Review(c.Request.Context(), auth.TenantID(c), auth.Current(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	verb := "复核通过"
	if req.Action == "reject" {
		verb = "复核拒绝"
	}
	audit.Record(c, audit.Entry{Module: module, Action: "review_" + req.Action, TargetType: "charge_transaction", TargetID: id.String(),
		Summary: verb + " 充电事务 " + t.TxNo, Before: req, After: t})
	httpx.OK(c, t)
}

func (h *handler) summary(c *gin.Context) {
	sm, err := h.svc.Summary(c.Request.Context(), auth.TenantID(c), c.Query("date"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sm)
}

func (h *handler) export(c *gin.Context) {
	var q ListQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		httpx.Fail(c, err)
		return
	}
	txs, err := h.svc.Export(c.Request.Context(), auth.TenantID(c), q)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	f, err := buildExport(txs)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer f.Close()
	buf, err := f.WriteToBuffer()
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="charging-`+time.Now().Format("20060102-150405")+`.xlsx"`)
	c.Data(200, xlsxMIME, buf.Bytes())
}
