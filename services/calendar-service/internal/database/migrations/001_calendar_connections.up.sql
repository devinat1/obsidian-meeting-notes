-- services/calendar-service/internal/database/migrations/001_calendar_connections.up.sql
CREATE TABLE calendar_connections (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL UNIQUE,
    refresh_token   TEXT NOT NULL,
    poll_interval_minutes   INTEGER NOT NULL DEFAULT 3,
    bot_join_before_minutes INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_calendar_connections_user_id ON calendar_connections(user_id);
