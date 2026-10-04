package store

// Regenerates internal/store/queries from internal/db/queries and the migrations.
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate -f ../../sqlc.yaml
