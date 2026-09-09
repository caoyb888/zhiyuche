// Package server wires the HTTP router, middleware and module routes.
package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/approval"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/card"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/device"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/pile"
	"github.com/caoyb888/zhiyuche/apps/api/internal/asset/vehicle"
	"github.com/caoyb888/zhiyuche/apps/api/internal/audit"
	"github.com/caoyb888/zhiyuche/apps/api/internal/auth"
	"github.com/caoyb888/zhiyuche/apps/api/internal/billing"
	"github.com/caoyb888/zhiyuche/apps/api/internal/charging"
	"github.com/caoyb888/zhiyuche/apps/api/internal/notify"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/auditlog"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/dept"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/dict"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/param"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/role"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/template"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/tenant"
	"github.com/caoyb888/zhiyuche/apps/api/internal/system/user"
	"github.com/caoyb888/zhiyuche/apps/api/internal/telemetry"
	"github.com/caoyb888/zhiyuche/apps/api/internal/trip"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Version is injected at build time via -ldflags "-X .../server.Version=x.y.z".
var Version = "dev"

// Server owns the gin engine and the http.Server.
type Server struct {
	app     *app.App
	engine  *gin.Engine
	http    *http.Server
	started time.Time
}

// New builds the router with global middleware and registers all routes.
func New(a *app.App) *Server {
	if a.Cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	e := gin.New()
	e.Use(requestID(), recovery(a.Log), accessLog(a.Log), cors(a.Cfg.HTTP.CORSOrigins))
	if a.DB != nil {
		e.Use(audit.NewWriter(a.DB, a.Log, resolveActor).Middleware())
	}
	e.NoRoute(func(c *gin.Context) { httpx.Fail(c, httpx.NotFound("route not found")) })

	s := &Server{
		app:     a,
		engine:  e,
		started: time.Now(),
		http: &http.Server{
			Addr:         a.Cfg.HTTP.Addr,
			Handler:      e,
			ReadTimeout:  a.Cfg.HTTP.ReadTimeout,
			WriteTimeout: a.Cfg.HTTP.WriteTimeout,
		},
	}
	s.registerRoutes()
	return s
}

// resolveActor adapts the auth principal for the audit middleware.
func resolveActor(c *gin.Context) (audit.Actor, bool) {
	p := auth.Current(c)
	if p == nil {
		return audit.Actor{}, false
	}
	return audit.Actor{UserID: p.UserID, TenantID: auth.TenantID(c), Username: p.Username}, true
}

// Handler exposes the router (used by tests).
func (s *Server) Handler() http.Handler { return s.engine }

// Run serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.app.Log.Info().Str("addr", s.http.Addr).Str("version", Version).Msg("http server listening")
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.app.Cfg.HTTP.ShutdownTimeout)
		defer cancel()
		s.app.Log.Info().Msg("http server shutting down")
		return s.http.Shutdown(shutdownCtx)
	}
}

func (s *Server) registerRoutes() {
	v1 := s.engine.Group("/api/v1")
	v1.GET("/health", s.health)
	v1.GET("/ready", s.ready)

	// Business routes need DB/Redis; tests may construct the server without them.
	if s.app.DB == nil || s.app.Redis == nil {
		return
	}
	authSvc := auth.NewService(s.app)
	authSvc.RegisterPublic(v1)

	protected := v1.Group("", authSvc.RequireAuth())
	authSvc.RegisterProtected(protected)

	// 系统管理模块
	user.Register(protected, s.app)
	dept.Register(protected, s.app)
	role.Register(protected, s.app)
	tenant.Register(protected, s.app)
	dict.Register(protected, s.app)
	param.Register(protected, s.app)
	auditlog.Register(protected, s.app)
	template.Register(protected, s.app)

	// 资产、审批、行程、通知（阶段 2）
	vehicle.Register(protected, s.app)
	device.Register(protected, s.app)
	card.Register(protected, s.app)
	pile.Register(protected, s.app)
	approval.Register(protected, s.app)
	trip.Register(protected, s.app)
	notify.Register(protected, s.app)

	// 计费与充电（阶段 3）：行程结束 → 计费扣费；充电结束 → 计价扣费
	trip.CompletedHook = billing.NewTripHook(s.app)
	billing.Register(protected, s.app)
	charging.Register(protected, s.app, billing.NewChargingHook(s.app))

	// WebSocket 推送：token 经 ?access_token= 传入（RequireAuth 已支持）；
	// 浏览器无法在 WS 握手上设请求头，超级管理员用 ?tenant_id= 指定订阅的租户
	if s.app.Hub != nil {
		protected.GET("/ws", s.app.Hub.Handler(s.app.Cfg.HTTP.CORSOrigins, func(c *gin.Context) (ws.Principal, bool) {
			p := auth.Current(c)
			if p == nil {
				return ws.Principal{}, false
			}
			tenantID := auth.TenantID(c)
			if p.IsSuper {
				if q := c.Query("tenant_id"); q != "" {
					if id, err := uuid.Parse(q); err == nil {
						tenantID = id
					}
				}
			}
			return ws.Principal{UserID: p.UserID, TenantID: tenantID}, true
		}))
	}

	// 车载网关上报（设备鉴权，不走 JWT）
	telemetry.Register(v1, s.app, trip.NewDeviceHooks(s.app))
}

type healthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Env           string `json:"env"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// health is a liveness probe: the process is up. It never touches dependencies.
func (s *Server) health(c *gin.Context) {
	httpx.OK(c, healthResponse{
		Status:        "ok",
		Version:       Version,
		Env:           s.app.Cfg.Env,
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
	})
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// ready is a readiness probe: dependencies must answer.
func (s *Server) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	resp := readyResponse{Status: "ok", Checks: map[string]string{}}
	check := func(name string, err error) {
		if err != nil {
			resp.Status = "degraded"
			resp.Checks[name] = err.Error()
			return
		}
		resp.Checks[name] = "ok"
	}
	if s.app.DB != nil {
		check("postgres", s.app.DB.Ping(ctx))
	} else {
		check("postgres", errors.New("not configured"))
	}
	if s.app.Redis != nil {
		check("redis", s.app.Redis.Ping(ctx).Err())
	} else {
		check("redis", errors.New("not configured"))
	}

	if resp.Status != "ok" {
		c.JSON(http.StatusServiceUnavailable, httpx.Envelope{Code: httpx.CodeInternal, Message: "not ready", Data: resp, RequestID: httpx.RequestID(c)})
		return
	}
	httpx.OK(c, resp)
}
