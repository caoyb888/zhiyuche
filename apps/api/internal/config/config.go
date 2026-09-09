// Package config loads runtime configuration from environment variables
// (optionally seeded from a .env file). All keys carry the ZY_ prefix.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Env      string `env:"ZY_ENV" envDefault:"dev"` // dev | test | prod
	LogLevel string `env:"ZY_LOG_LEVEL" envDefault:"debug"`

	HTTP struct {
		Addr            string        `env:"ZY_HTTP_ADDR" envDefault:":20080"`
		ReadTimeout     time.Duration `env:"ZY_HTTP_READ_TIMEOUT" envDefault:"15s"`
		WriteTimeout    time.Duration `env:"ZY_HTTP_WRITE_TIMEOUT" envDefault:"30s"`
		ShutdownTimeout time.Duration `env:"ZY_HTTP_SHUTDOWN_TIMEOUT" envDefault:"10s"`
		CORSOrigins     []string      `env:"ZY_HTTP_CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:20173"`
	}

	Database struct {
		URL         string `env:"ZY_DATABASE_URL" envDefault:"postgres://zhiyuche:zhiyuche@localhost:20432/zhiyuche?sslmode=disable"`
		MaxConns    int32  `env:"ZY_DATABASE_MAX_CONNS" envDefault:"10"`
		AutoMigrate bool   `env:"ZY_DATABASE_AUTO_MIGRATE" envDefault:"false"`
	}

	Redis struct {
		Addr     string `env:"ZY_REDIS_ADDR" envDefault:"localhost:20379"`
		Password string `env:"ZY_REDIS_PASSWORD"`
		DB       int    `env:"ZY_REDIS_DB" envDefault:"0"`
	}

	JWT struct {
		Secret     string        `env:"ZY_JWT_SECRET" envDefault:"change-me-in-production"`
		AccessTTL  time.Duration `env:"ZY_JWT_ACCESS_TTL" envDefault:"15m"`
		RefreshTTL time.Duration `env:"ZY_JWT_REFRESH_TTL" envDefault:"168h"`
	}
}

func (c *Config) IsProd() bool { return c.Env == "prod" }

// Load reads .env (if present, never overriding real env vars) and parses the environment.
func Load() (*Config, error) {
	_ = godotenv.Load() // best effort; missing file is fine

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	if cfg.IsProd() && cfg.JWT.Secret == "change-me-in-production" {
		return nil, fmt.Errorf("ZY_JWT_SECRET must be set in prod")
	}
	return &cfg, nil
}
