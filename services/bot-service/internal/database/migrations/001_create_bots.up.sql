CREATE TABLE bots (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    bot_id TEXT NOT NULL UNIQUE,
    calendar_event_id TEXT NOT NULL,
    meeting_url TEXT NOT NULL,
    meeting_title TEXT NOT NULL DEFAULT '',
    bot_status TEXT NOT NULL DEFAULT 'scheduled',
    bot_webhook_updated_at TIMESTAMPTZ,
    recording_id TEXT,
    recording_status TEXT,
    recording_webhook_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bots_user_id ON bots (user_id);
CREATE INDEX idx_bots_bot_id ON bots (bot_id);
CREATE INDEX idx_bots_calendar_event_id ON bots (calendar_event_id);
CREATE INDEX idx_bots_bot_status ON bots (bot_status);
