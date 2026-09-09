// Package app holds the shared infrastructure handles every module receives.
// It exists so business modules do not import the server package (import cycle).
package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/internal/config"
)

type App struct {
	Cfg   *config.Config
	Log   zerolog.Logger
	DB    *pgxpool.Pool
	Redis *redis.Client
}
