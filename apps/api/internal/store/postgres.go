// Package store implements persistence for the API on PostgreSQL.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"staykey.direct/api/internal/domain"
)

const uniqueViolation = "23505"

const propertyColumns = `id, slug, name, booking_type, coalesce(location, ''), currency, base_rate_minor, created_at`

// Postgres is a store backed by a pgx connection pool.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres creates a pool for databaseURL. Connections are opened lazily, so
// the API starts even when the database is briefly unavailable.
func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close releases all connections.
func (s *Postgres) Close() { s.pool.Close() }

// Ping checks that the database is reachable.
func (s *Postgres) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// ListProperties returns properties, newest first.
func (s *Postgres) ListProperties(ctx context.Context) ([]domain.Property, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+propertyColumns+` FROM properties ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("list properties: %w", err)
	}
	return pgx.CollectRows(rows, scanProperty)
}

// CreateProperty inserts a property. It returns domain.ErrSlugTaken when the
// booking page address is already used.
func (s *Postgres) CreateProperty(ctx context.Context, p domain.NewProperty) (domain.Property, error) {
	rows, err := s.pool.Query(ctx, `
		INSERT INTO properties (slug, name, booking_type, location, currency, base_rate_minor)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)
		RETURNING `+propertyColumns,
		p.Slug, p.Name, string(p.BookingType), p.Location, p.Currency, p.BaseRateMinor)
	if err != nil {
		return domain.Property{}, fmt.Errorf("create property: %w", err)
	}
	created, err := pgx.CollectExactlyOneRow(rows, scanProperty)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.Property{}, domain.ErrSlugTaken
		}
		return domain.Property{}, fmt.Errorf("create property: %w", err)
	}
	return created, nil
}

// GetPropertyBySlug returns the property behind a booking page address.
func (s *Postgres) GetPropertyBySlug(ctx context.Context, slug string) (domain.Property, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+propertyColumns+` FROM properties WHERE slug = $1`, slug)
	if err != nil {
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}
	p, err := pgx.CollectExactlyOneRow(rows, scanProperty)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Property{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}
	return p, nil
}

func scanProperty(row pgx.CollectableRow) (domain.Property, error) {
	var p domain.Property
	var bookingType string
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &bookingType, &p.Location, &p.Currency, &p.BaseRateMinor, &p.CreatedAt)
	p.BookingType = domain.BookingType(bookingType)
	return p, err
}
