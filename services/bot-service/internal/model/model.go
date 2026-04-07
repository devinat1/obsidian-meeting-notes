package model

import "time"

type Bot struct {
	ID                        int        `json:"id"`
	UserID                    int        `json:"user_id"`
	BotID                     string     `json:"bot_id"`
	CalendarEventID           string     `json:"calendar_event_id"`
	MeetingURL                string     `json:"meeting_url"`
	MeetingTitle              string     `json:"meeting_title"`
	BotStatus                 string     `json:"bot_status"`
	BotWebhookUpdatedAt       *time.Time `json:"bot_webhook_updated_at,omitempty"`
	RecordingID               *string    `json:"recording_id,omitempty"`
	RecordingStatus           *string    `json:"recording_status,omitempty"`
	RecordingWebhookUpdatedAt *time.Time `json:"recording_webhook_updated_at,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type MeetingUpcomingEvent struct {
	UserID          int    `json:"user_id"`
	CalendarEventID string `json:"calendar_event_id"`
	MeetingURL      string `json:"meeting_url"`
	Title           string `json:"title"`
	StartTime       string `json:"start_time"`
}

type MeetingCancelledEvent struct {
	UserID          int    `json:"user_id"`
	CalendarEventID string `json:"calendar_event_id"`
	BotID           string `json:"bot_id"`
}

type BotStatusEvent struct {
	UserID       int    `json:"user_id"`
	BotID        string `json:"bot_id"`
	BotStatus    string `json:"bot_status"`
	MeetingTitle string `json:"meeting_title"`
}

type BotStatusResponse struct {
	UserID       int    `json:"user_id"`
	BotID        string `json:"bot_id"`
	BotStatus    string `json:"bot_status"`
	MeetingTitle string `json:"meeting_title"`
	MeetingURL   string `json:"meeting_url"`
}

type RecallWebhookPayload struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}

var TerminalBotStatuses = map[string]bool{
	"done":      true,
	"fatal":     true,
	"cancelled": true,
}
