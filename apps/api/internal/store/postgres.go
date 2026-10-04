// Package store implements persistence for the API on PostgreSQL. It is the only package that
// touches SQL: queries live in internal/db/queries and sqlc turns them into internal/store/queries.
//
// Owner data is always read and written through a tenant.Tenant, so every query is scoped to
// the caller's account.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
)

const uniqueViolation = "23505"

// Postgres is a store backed by a pgx connection pool.
type Postgres struct {
	pool *pgxpool.Pool
	q    *queries.Queries
}

// NewPostgres creates a pool for databaseURL. Connections are opened lazily, so the API starts
// even when the database is briefly unavailable.
func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	return NewPostgresWithConfig(ctx, cfg)
}

// NewPostgresWithConfig creates a pool from a parsed config, for callers that need to adjust it.
func NewPostgresWithConfig(ctx context.Context, cfg *pgxpool.Config) (*Postgres, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &Postgres{pool: pool, q: queries.New(pool)}, nil
}

// Close releases all connections.
func (s *Postgres) Close() { s.pool.Close() }

// Ping checks that the database is reachable.
func (s *Postgres) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Pool exposes the pool for migrations and tests.
func (s *Postgres) Pool() *pgxpool.Pool { return s.pool }

type txKey struct{}

// InTx runs fn in a transaction. Store calls made with the ctx passed to fn join it; a nested
// InTx becomes a savepoint. The transaction commits when fn returns nil.
func (s *Postgres) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return pgx.BeginFunc(ctx, tx, func(sp pgx.Tx) error {
			return fn(context.WithValue(ctx, txKey{}, sp))
		})
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// db returns queries bound to the transaction in ctx, or to the pool.
func (s *Postgres) db(ctx context.Context) *queries.Queries {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return s.q.WithTx(tx)
	}
	return s.q
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
