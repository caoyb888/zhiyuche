// Command api is the main REST/WebSocket service.
//
//	api                 run the HTTP server
//	api migrate <cmd>   run goose: up | down | status | version | up-by-one | redo
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/caoyb888/zhiyuche/apps/api/internal/app"
	"github.com/caoyb888/zhiyuche/apps/api/internal/billing"
	"github.com/caoyb888/zhiyuche/apps/api/internal/bootstrap"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ocpp"
	"github.com/caoyb888/zhiyuche/apps/api/internal/config"
	"github.com/caoyb888/zhiyuche/apps/api/internal/migrate"
	"github.com/caoyb888/zhiyuche/apps/api/internal/server"
	"github.com/caoyb888/zhiyuche/apps/api/internal/ws"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/cache"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/db"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.Setup(cfg.LogLevel, !cfg.IsProd())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		cmd := "up"
		var args []string
		if len(os.Args) > 2 {
			cmd, args = os.Args[2], os.Args[3:]
		}
		log.Info().Str("command", cmd).Msg("running migrations")
		return migrate.Run(ctx, cfg.Database.URL, cmd, args...)
	}

	if cfg.Database.AutoMigrate {
		log.Info().Msg("auto-migrate enabled, applying pending migrations")
		if err := migrate.Run(ctx, cfg.Database.URL, "up"); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
	}

	pool, err := db.Connect(ctx, cfg.Database.URL, cfg.Database.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Info().Msg("postgres connected")

	rdb, err := cache.Connect(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return err
	}
	defer rdb.Close()
	log.Info().Msg("redis connected")

	if err := bootstrap.Run(ctx, pool, cfg, log); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	log.Info().Msg("bootstrap complete")

	a := &app.App{Cfg: cfg, Log: log, DB: pool, Redis: rdb, Hub: ws.NewHub(log)}

	// 充电桩 OCPP 接入：独立监听端口，与 HTTP 服务同进程
	ocppSrv := ocpp.New(a, billing.NewChargingHook(a))
	go func() {
		if err := ocppSrv.Run(ctx); err != nil {
			log.Error().Err(err).Msg("ocpp server stopped")
			stop()
		}
	}()

	srv := server.New(a)
	return srv.Run(ctx)
}
