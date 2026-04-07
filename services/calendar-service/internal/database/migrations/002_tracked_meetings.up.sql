-- services/calendar-service/internal/database/migrations/002_tracked_meetings.up.sql
CREATE TABLE tracked_meetings (
    id                  SERIAL PRIMARY KEY,
    user_id             INTEGER NOT NULL,
    calendar_event_id   TEXT NOT NULL,
    title               TEXT NOT NULL,
    start_time          TIMESTAMPTZ NOT NULL,
    end_time            TIMESTAMPTZ NOT NULL,
    meeting_url         TEXT NOT NULL,
    attendees           JSONB NOT NULL DEFAULT '[]'::jsonb,
    bot_dispatched      BOOLEAN NOT NULL DEFAULT FALSE,
    cancelled           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, calendar_event_id)
);

CREATE INDEX idx_tracked_meetings_user_id ON tracked_meetings(user_id);
CREATE INDEX idx_tracked_meetings_start_time ON tracked_meetings(start_time);
CREATE INDEX idx_tracked_meetings_user_dispatched ON tracked_meetings(user_id, bot_dispatched) WHERE bot_dispatched = FALSE;
