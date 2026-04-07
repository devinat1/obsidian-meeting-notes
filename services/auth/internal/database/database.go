// services/auth/internal/database/database.go
package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

func RunMigrations(databaseURL string) error {
	return migrate.Run(migrate.RunParams{
		MigrationsFS: migrationsFS,
		DatabaseURL:  databaseURL,
	})
}
