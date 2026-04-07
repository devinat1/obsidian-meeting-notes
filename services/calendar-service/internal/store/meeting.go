// services/calendar-service/internal/store/meeting.go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MeetingStore struct {
	pool *pgxpool.Pool
}

func NewMeetingStore(pool *pgxpool.Pool) *MeetingStore {
	return &MeetingStore{pool: pool}
}

type UpsertMeetingParams struct {
	UserID          int
	CalendarEventID string
	Title           string
	StartTime       time.Time
	EndTime         time.Time
	MeetingURL      string
	Attendees       json.RawMessage
}

// Upsert inserts or updates a tracked meeting. Returns the meeting and whether it was newly inserted.
func (s *MeetingStore) Upsert(ctx context.Context, params UpsertMeetingParams) (*model.TrackedMeeting, bool, error) {
	var meeting model.TrackedMeeting
	var isInsert bool

	err := s.pool.QueryRow(ctx, `
		INSERT INTO tracked_meetings (user_id, calendar_event_id, title, start_time, end_time, meeting_url, attendees)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, calendar_event_id) DO UPDATE SET
			title = EXCLUDED.title,
			start_time = EXCLUDED.start_time,
			end_time = EXCLUDED.end_time,
			meeting_url = EXCLUDED.meeting_url,
			attendees = EXCLUDED.attendees,
			updated_at = NOW()
		RETURNING id, user_id, calendar_event_id, title, start_time, end_time, meeting_url, attendees,
			bot_dispatched, cancelled, created_at, updated_at,
			(xmax = 0) AS is_insert
	`, params.UserID, params.CalendarEventID, params.Title, params.StartTime, params.EndTime, params.MeetingURL, params.Attendees).Scan(
		&meeting.ID, &meeting.UserID, &meeting.CalendarEventID, &meeting.Title,
		&meeting.StartTime, &meeting.EndTime, &meeting.MeetingURL, &meeting.Attendees,
		&meeting.BotDispatched, &meeting.Cancelled, &meeting.CreatedAt, &meeting.UpdatedAt,
		&isInsert,
	)
	if err != nil {
		return nil, false, fmt.Errorf("Failed to upsert tracked meeting: %w.", err)
	}

	return &meeting, isInsert, nil
}

func (s *MeetingStore) MarkBotDispatched(ctx context.Context, userID int, calendarEventID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tracked_meetings
		SET bot_dispatched = TRUE, updated_at = NOW()
		WHERE user_id = $1 AND calendar_event_id = $2
	`, userID, calendarEventID)
	if err != nil {
		return fmt.Errorf("Failed to mark bot dispatched: %w.", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("No tracked meeting found for user %d, event %s.", userID, calendarEventID)
	}
	return nil
}

func (s *MeetingStore) MarkCancelled(ctx context.Context, userID int, calendarEventID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tracked_meetings
		SET cancelled = TRUE, updated_at = NOW()
		WHERE user_id = $1 AND calendar_event_id = $2
	`, userID, calendarEventID)
	if err != nil {
		return fmt.Errorf("Failed to mark meeting cancelled: %w.", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("No tracked meeting found for user %d, event %s.", userID, calendarEventID)
	}
	return nil
}

// GetUpcomingByUserID returns non-cancelled meetings for a user starting within the given window.
func (s *MeetingStore) GetUpcomingByUserID(ctx context.Context, userID int, from time.Time, to time.Time) ([]model.TrackedMeeting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, calendar_event_id, title, start_time, end_time, meeting_url, attendees,
			bot_dispatched, cancelled, created_at, updated_at
		FROM tracked_meetings
		WHERE user_id = $1 AND cancelled = FALSE AND start_time >= $2 AND start_time <= $3
		ORDER BY start_time ASC
	`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("Failed to get upcoming meetings: %w.", err)
	}
	defer rows.Close()

	meetings := []model.TrackedMeeting{}
	for rows.Next() {
		var m model.TrackedMeeting
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.CalendarEventID, &m.Title,
			&m.StartTime, &m.EndTime, &m.MeetingURL, &m.Attendees,
			&m.BotDispatched, &m.Cancelled, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("Failed to scan tracked meeting: %w.", err)
		}
		meetings = append(meetings, m)
	}

	return meetings, nil
}

// GetDispatchedEventIDs returns calendar_event_ids for dispatched, non-cancelled meetings for a user.
func (s *MeetingStore) GetDispatchedEventIDs(ctx context.Context, userID int) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT calendar_event_id
		FROM tracked_meetings
		WHERE user_id = $1 AND bot_dispatched = TRUE AND cancelled = FALSE
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("Failed to get dispatched event IDs: %w.", err)
	}
	defer rows.Close()

	result := map[string]bool{}
	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return nil, fmt.Errorf("Failed to scan event ID: %w.", err)
		}
		result[eventID] = true
	}

	return result, nil
}

func (s *MeetingStore) DeleteByUserID(ctx context.Context, userID int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM tracked_meetings WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("Failed to delete tracked meetings: %w.", err)
	}
	return nil
}
