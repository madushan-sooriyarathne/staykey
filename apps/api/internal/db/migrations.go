// Package db embeds the SQL migrations, applied with goose by cmd/migrate.
package db

import "embed"

// Migrations holds the goose migration files under migrations/.
//
//go:embed migrations/*.sql
var Migrations embed.FS

// MigrationsDir is the directory inside Migrations that holds the files.
const MigrationsDir = "migrations"
