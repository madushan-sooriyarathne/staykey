// Command migrate applies the embedded SQL migrations with goose.
//
// Usage:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down
//	go run ./cmd/migrate status
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"staykey.direct/api/internal/config"
	"staykey.direct/api/internal/db"
)

func main() {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	if err := run(command, os.Args[min(len(os.Args), 2):]); err != nil {
		slog.Error("migrate failed", "command", command, "error", err)
		os.Exit(1)
	}
}

func run(command string, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	conn, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.RunContext(context.Background(), command, conn, db.MigrationsDir, args...)
}
