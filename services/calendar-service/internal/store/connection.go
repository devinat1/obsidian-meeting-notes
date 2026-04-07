// services/calendar-service/internal/store/connection.go
package store

import (
	"context"
	"fmt"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectionStore struct {
	pool      *pgxpool.Pool
	encryptor *encryption.Encryptor
}

func NewConnectionStore(pool *pgxpool.Pool, encryptor *encryption.Encryptor) *ConnectionStore {
	return &ConnectionStore{pool: pool, encryptor: encryptor}
}

func (s *ConnectionStore) Upsert(ctx context.Context, params UpsertConnectionParams) (*model.CalendarConnection, error) {
	encryptedToken, err := s.encryptor.Encrypt(params.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("Failed to encrypt refresh token: %w.", err)
	}

	var conn model.CalendarConnection
	err = s.pool.QueryRow(ctx, `
		INSERT INTO calendar_connections (user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			refresh_token = EXCLUDED.refresh_token,
			updated_at = NOW()
		RETURNING id, user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes, created_at, updated_at
	`, params.UserID, encryptedToken, params.PollIntervalMinutes, params.BotJoinBeforeMinutes).Scan(
		&conn.ID, &conn.UserID, &conn.RefreshToken,
		&conn.PollIntervalMinutes, &conn.BotJoinBeforeMinutes,
		&conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to upsert calendar connection: %w.", err)
	}

	return &conn, nil
}

type UpsertConnectionParams struct {
	UserID               int
	RefreshToken         string
	PollIntervalMinutes  int
	BotJoinBeforeMinutes int
}

func (s *ConnectionStore) GetByUserID(ctx context.Context, userID int) (*model.CalendarConnection, error) {
	var conn model.CalendarConnection
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes, created_at, updated_at
		FROM calendar_connections
		WHERE user_id = $1
	`, userID).Scan(
		&conn.ID, &conn.UserID, &conn.RefreshToken,
		&conn.PollIntervalMinutes, &conn.BotJoinBeforeMinutes,
		&conn.CreatedAt, &conn.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to get calendar connection: %w.", err)
	}

	return &conn, nil
}

// GetDecryptedRefreshToken retrieves and decrypts the refresh token for a user.
func (s *ConnectionStore) GetDecryptedRefreshToken(ctx context.Context, userID int) (string, error) {
	conn, err := s.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if conn == nil {
		return "", fmt.Errorf("No calendar connection found for user %d.", userID)
	}

	decrypted, err := s.encryptor.Decrypt(conn.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("Failed to decrypt refresh token: %w.", err)
	}

	return decrypted, nil
}

func (s *ConnectionStore) GetAll(ctx context.Context) ([]model.CalendarConnection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes, created_at, updated_at
		FROM calendar_connections
	`)
	if err != nil {
		return nil, fmt.Errorf("Failed to list calendar connections: %w.", err)
	}
	defer rows.Close()

	connections := []model.CalendarConnection{}
	for rows.Next() {
		var conn model.CalendarConnection
		if err := rows.Scan(
			&conn.ID, &conn.UserID, &conn.RefreshToken,
			&conn.PollIntervalMinutes, &conn.BotJoinBeforeMinutes,
			&conn.CreatedAt, &conn.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("Failed to scan calendar connection: %w.", err)
		}
		connections = append(connections, conn)
	}

	return connections, nil
}

func (s *ConnectionStore) DeleteByUserID(ctx context.Context, userID int) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM calendar_connections WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("Failed to delete calendar connection: %w.", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("No calendar connection found for user %d.", userID)
	}
	return nil
}
