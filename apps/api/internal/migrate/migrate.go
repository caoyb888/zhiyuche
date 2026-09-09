// Package migrate runs the embedded SQL migrations with goose.
package migrate

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // register pgx as database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/caoyb888/zhiyuche/apps/api/migrations"
)

// Run executes a goose command (up, down, status, version, up-by-one, redo).
func Run(ctx context.Context, databaseURL, command string, args ...string) error {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.RunContext(ctx, command, sqlDB, ".", args...)
}
