// Package storetest gives each test its own PostgreSQL schema with every migration applied, so
// tests run in parallel against a real database without seeing each other's rows.
//
// The database comes from STAYKEY_TEST_DATABASE_URL, then DATABASE_URL, then the development
// default. When it cannot be reached the test is skipped locally and fails in CI.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"staykey.direct/api/internal/db"
	"staykey.direct/api/internal/secret"
	"staykey.direct/api/internal/store"
)

const devURL = "postgres://staykey:staykey@localhost:5432/staykey?sslmode=disable"

// URL is the database tests connect to.
func URL() string {
	for _, key := range []string{"STAYKEY_TEST_DATABASE_URL", "DATABASE_URL"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return devURL
}

// New returns a store on a fresh, fully migrated schema that is dropped when the test ends.
func New(t testing.TB) *store.Postgres {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, URL())
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("connect to test database: %v", err)
		}
		t.Skipf("no test database (%v); start one with `bun run db:up`", err)
	}

	role, err := appRole(ctx, admin)
	if err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("test role: %v", err)
	}
	schema := "test_" + randomHex(6)
	create := "CREATE SCHEMA " + schema
	if role != "" {
		create += " AUTHORIZATION " + role
	}
	if _, err := admin.Exec(ctx, create); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close(ctx)
	})

	cfg, err := pgxpool.ParseConfig(URL())
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	if role != "" {
		cfg.ConnConfig.User, cfg.ConnConfig.Password = role, role
	}
	cfg.MaxConns = 8
	s, err := store.NewPostgresWithConfig(ctx, cfg, store.WithSecrets(secret.FromPassphrase("storetest")))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(s.Close)

	migrate(t, ctx, s.Pool())
	return s
}

// rlsRole is an ordinary role the store connects as when the test database's user is a
// superuser, since superusers skip row-level security and the tests should run behind it.
const rlsRole = "staykey_rls_test"

// appRole makes sure rlsRole exists when admin is a superuser and returns it, or returns "" to
// connect as admin's own (ordinary) role.
func appRole(ctx context.Context, admin *pgx.Conn) (string, error) {
	var super bool
	if err := admin.QueryRow(ctx, "SELECT rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&super); err != nil {
		return "", err
	}
	if !super {
		return "", nil
	}
	// Parallel tests may race to create the role; either error means it exists.
	_, err := admin.Exec(ctx, `DO $$ BEGIN
		CREATE ROLE `+rlsRole+` LOGIN PASSWORD '`+rlsRole+`' NOSUPERUSER NOBYPASSRLS;
	EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL;
	END $$`)
	return rlsRole, err
}

func migrate(t testing.TB, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migrations, err := fs.Sub(db.Migrations, db.MigrationsDir)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		t.Fatalf("goose provider: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
