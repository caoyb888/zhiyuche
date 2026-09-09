// Package server wires the HTTP router, middleware and module routes.
package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/internal/config"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

// Version is injected at build time via -ldflags "-X .../server.Version=x.y.z".
var Version = "dev"

// Deps are the shared infrastructure handles modules receive.
type Deps struct {
	Cfg   *config.Config
	Log   zerolog.Logger
	DB    *pgxpool.Pool
	Redis *redis.Client
}

// Server owns the gin engine and the http.Server.
type Server struct {
	deps    Deps
	engine  *gin.Engine
	http    *http.Server
	started time.Time
}

// New builds the router with global middleware and registers all routes.
func New(d Deps) *Server {
	if d.Cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	e := gin.New()
	e.Use(requestID(), recovery(d.Log), accessLog(d.Log), cors(d.Cfg.HTTP.CORSOrigins))
	e.NoRoute(func(c *gin.Context) { httpx.Fail(c, httpx.NotFound("route not found")) })

	s := &Server{
		deps:    d,
		engine:  e,
		started: time.Now(),
		http: &http.Server{
			Addr:         d.Cfg.HTTP.Addr,
			Handler:      e,
			ReadTimeout:  d.Cfg.HTTP.ReadTimeout,
			WriteTimeout: d.Cfg.HTTP.WriteTimeout,
		},
	}
	s.registerRoutes()
	return s
}

// Handler exposes the router (used by tests).
func (s *Server) Handler() http.Handler { return s.engine }

// Run serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.deps.Log.Info().Str("addr", s.http.Addr).Str("version", Version).Msg("http server listening")
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.deps.Cfg.HTTP.ShutdownTimeout)
		defer cancel()
		s.deps.Log.Info().Msg("http server shutting down")
		return s.http.Shutdown(shutdownCtx)
	}
}

func (s *Server) registerRoutes() {
	v1 := s.engine.Group("/api/v1")
	v1.GET("/health", s.health)
	v1.GET("/ready", s.ready)

	// 各业务模块从阶段 1 起在此注册：
	// auth.Register(v1, s.deps) ; system.Register(v1, s.deps) ; ...
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
		Env:           s.deps.Cfg.Env,
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
	if s.deps.DB != nil {
		check("postgres", s.deps.DB.Ping(ctx))
	} else {
		check("postgres", errors.New("not configured"))
	}
	if s.deps.Redis != nil {
		check("redis", s.deps.Redis.Ping(ctx).Err())
	} else {
		check("redis", errors.New("not configured"))
	}

	if resp.Status != "ok" {
		c.JSON(http.StatusServiceUnavailable, httpx.Envelope{Code: httpx.CodeInternal, Message: "not ready", Data: resp, RequestID: httpx.RequestID(c)})
		return
	}
	httpx.OK(c, resp)
}
