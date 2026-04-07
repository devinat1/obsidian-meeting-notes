// services/calendar-service/internal/model/model.go
package model

import (
	"encoding/json"
	"time"
)

// CalendarConnection represents a user's Google Calendar OAuth connection.
type CalendarConnection struct {
	ID                   int       `json:"id"`
	UserID               int       `json:"user_id"`
	RefreshToken         string    `json:"-"` // encrypted, never serialized
	PollIntervalMinutes  int       `json:"poll_interval_minutes"`
	BotJoinBeforeMinutes int       `json:"bot_join_before_minutes"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// TrackedMeeting represents a calendar event with a video meeting link.
type TrackedMeeting struct {
	ID              int             `json:"id"`
	UserID          int             `json:"user_id"`
	CalendarEventID string          `json:"calendar_event_id"`
	Title           string          `json:"title"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         time.Time       `json:"end_time"`
	MeetingURL      string          `json:"meeting_url"`
	Attendees       json.RawMessage `json:"attendees"`
	BotDispatched   bool            `json:"bot_dispatched"`
	Cancelled       bool            `json:"cancelled"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// MeetingUpcomingEvent is the Kafka payload for meeting.upcoming.
type MeetingUpcomingEvent struct {
	UserID          int       `json:"user_id"`
	CalendarEventID string    `json:"calendar_event_id"`
	MeetingURL      string    `json:"meeting_url"`
	Title           string    `json:"title"`
	StartTime       time.Time `json:"start_time"`
}

// MeetingCancelledEvent is the Kafka payload for meeting.cancelled.
type MeetingCancelledEvent struct {
	UserID          int    `json:"user_id"`
	CalendarEventID string `json:"calendar_event_id"`
	BotID           string `json:"bot_id,omitempty"`
}

// AuthURLResponse is the response from GET /internal/calendar/auth-url.
type AuthURLResponse struct {
	URL string `json:"url"`
}

// CallbackRequest is the request body for POST /internal/calendar/callback.
type CallbackRequest struct {
	Code   string `json:"code"`
	UserID int    `json:"user_id"`
}

// StatusResponse is the response from GET /internal/calendar/status.
type StatusResponse struct {
	Connected bool `json:"connected"`
}

// Attendee represents a meeting participant from Google Calendar.
type Attendee struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Self        bool   `json:"self,omitempty"`
}
