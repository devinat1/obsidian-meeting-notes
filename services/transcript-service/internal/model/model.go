package model

import "time"

type Transcript struct {
	ID                 int              `json:"id"`
	BotID              string           `json:"bot_id"`
	UserID             int              `json:"user_id"`
	MeetingTitle       string           `json:"meeting_title"`
	TranscriptID       string           `json:"transcript_id"`
	Status             string           `json:"status"`
	FailureSubCode     *string          `json:"failure_sub_code,omitempty"`
	RawTranscript      *[]RawSegment    `json:"raw_transcript,omitempty"`
	ReadableTranscript *[]ReadableBlock `json:"readable_transcript,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
}

type RawSegment struct {
	Speaker string    `json:"speaker"`
	Words   []RawWord `json:"words"`
}

type RawWord struct {
	Text       string  `json:"text"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	Confidence float64 `json:"confidence"`
}

type ReadableBlock struct {
	Speaker   string `json:"speaker"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

type TranscriptReadyEvent struct {
	UserID             int             `json:"user_id"`
	BotID              string          `json:"bot_id"`
	MeetingTitle       string          `json:"meeting_title"`
	RawTranscript      []RawSegment    `json:"raw_transcript"`
	ReadableTranscript []ReadableBlock `json:"readable_transcript"`
}

type TranscriptFailedEvent struct {
	UserID         int    `json:"user_id"`
	BotID          string `json:"bot_id"`
	MeetingTitle   string `json:"meeting_title"`
	FailureSubCode string `json:"failure_sub_code"`
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
