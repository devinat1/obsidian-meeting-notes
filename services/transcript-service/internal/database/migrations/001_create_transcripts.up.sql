CREATE TABLE transcripts (
    id SERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    meeting_title TEXT NOT NULL DEFAULT '',
    transcript_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    failure_sub_code TEXT,
    raw_transcript JSONB,
    readable_transcript JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transcripts_bot_id ON transcripts (bot_id);
CREATE INDEX idx_transcripts_user_id ON transcripts (user_id);
