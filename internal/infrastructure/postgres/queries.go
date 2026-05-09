package postgres

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate -f ../../../sqlc.yaml

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"cassyork.dev/platform/internal/infrastructure/postgres/sqlcgen"
)

// NewQueries builds sqlc-generated queries against a pool (sqlc + pgx/v5).
func NewQueries(pool *pgxpool.Pool) *sqlcgen.Queries {
	return sqlcgen.New(pool)
}
