// services/auth/internal/background/cleanup.go
package background

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func StartUnverifiedUserCleanup(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Unverified user cleanup goroutine stopping.")
			return
		case <-ticker.C:
			cleanupUnverifiedUsers(ctx, db)
		}
	}
}

func cleanupUnverifiedUsers(ctx context.Context, db *pgxpool.Pool) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	tag, err := db.Exec(ctx,
		"DELETE FROM users WHERE verified = FALSE AND created_at < $1",
		cutoff,
	)
	if err != nil {
		log.Printf("Error cleaning up unverified users: %v", err)
		return
	}

	if tag.RowsAffected() > 0 {
		log.Printf("Cleaned up %d unverified users older than 7 days.", tag.RowsAffected())
	}
}
