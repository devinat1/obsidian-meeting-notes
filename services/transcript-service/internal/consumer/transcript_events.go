package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/recall"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/transcript"
)

type TranscriptEventsConsumer struct {
	reader        *kafkago.Reader
	pool          *pgxpool.Pool
	recallClient  *recall.Client
	readyWriter   *kafkago.Writer
	failedWriter  *kafkago.Writer
	botServiceURL string
	httpClient    *http.Client
}

type NewTranscriptEventsConsumerParams struct {
	Reader        *kafkago.Reader
	Pool          *pgxpool.Pool
	RecallClient  *recall.Client
	ReadyWriter   *kafkago.Writer
	FailedWriter  *kafkago.Writer
	BotServiceURL string
}

func NewTranscriptEventsConsumer(params NewTranscriptEventsConsumerParams) *TranscriptEventsConsumer {
	return &TranscriptEventsConsumer{
		reader:        params.Reader,
		pool:          params.Pool,
		recallClient:  params.RecallClient,
		readyWriter:   params.ReadyWriter,
		failedWriter:  params.FailedWriter,
		botServiceURL: params.BotServiceURL,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *TranscriptEventsConsumer) Start(ctx context.Context) {
	log.Println("Starting transcript.events consumer.")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Transcript events consumer shutting down.")
				return
			}
			log.Printf("Error reading transcript.events message: %v.", err)
			continue
		}

		if err := c.handleMessage(ctx, msg); err != nil {
			log.Printf("Error handling transcript.events message: %v.", err)
		}
	}
}

type recallTranscriptWebhook struct {
	Event string `json:"event"`
	Data  struct {
		BotID          string `json:"bot_id"`
		TranscriptID   string `json:"transcript_id"`
		FailureSubCode string `json:"failure_sub_code"`
	} `json:"data"`
}

func (c *TranscriptEventsConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var webhook recallTranscriptWebhook
	if err := json.Unmarshal(msg.Value, &webhook); err != nil {
		return fmt.Errorf("failed to unmarshal transcript webhook: %w", err)
	}

	botID := string(msg.Key)
	if botID == "" || botID == "unknown" {
		botID = webhook.Data.BotID
	}

	switch webhook.Event {
	case "transcript.done":
		return c.handleTranscriptDone(ctx, botID, webhook.Data.TranscriptID)
	case "transcript.failed":
		return c.handleTranscriptFailed(ctx, botID, webhook.Data.FailureSubCode)
	default:
		log.Printf("Ignoring transcript event type: %s.", webhook.Event)
		return nil
	}
}

func (c *TranscriptEventsConsumer) handleTranscriptDone(ctx context.Context, botID string, transcriptID string) error {
	log.Printf("Processing transcript.done for bot %s, transcript %s.", botID, transcriptID)

	botStatus, err := c.fetchBotStatus(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to resolve bot info for bot %s: %w", botID, err)
	}

	metadata, err := c.recallClient.FetchTranscriptMetadata(ctx, recall.FetchTranscriptMetadataParams{
		TranscriptID: transcriptID,
	})
	if err != nil {
		return fmt.Errorf("failed to fetch transcript metadata: %w", err)
	}

	rawSegments, err := c.recallClient.DownloadTranscript(ctx, recall.DownloadTranscriptParams{
		DownloadURL: metadata.DownloadURL,
	})
	if err != nil {
		return fmt.Errorf("failed to download transcript: %w", err)
	}

	readableBlocks := transcript.ConvertToReadable(transcript.ConvertToReadableParams{
		Segments: rawSegments,
	})

	rawJSON, err := json.Marshal(rawSegments)
	if err != nil {
		return fmt.Errorf("failed to marshal raw transcript: %w", err)
	}
	readableJSON, err := json.Marshal(readableBlocks)
	if err != nil {
		return fmt.Errorf("failed to marshal readable transcript: %w", err)
	}

	_, err = c.pool.Exec(ctx,
		`INSERT INTO transcripts (bot_id, user_id, meeting_title, transcript_id, status, raw_transcript, readable_transcript)
		 VALUES ($1, $2, $3, $4, 'done', $5, $6)
		 ON CONFLICT (bot_id) DO UPDATE SET
		   status = 'done',
		   transcript_id = $4,
		   raw_transcript = $5,
		   readable_transcript = $6`,
		botID, botStatus.UserID, botStatus.MeetingTitle, transcriptID, rawJSON, readableJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to store transcript: %w", err)
	}

	readyEvent := model.TranscriptReadyEvent{
		UserID:             botStatus.UserID,
		BotID:              botID,
		MeetingTitle:       botStatus.MeetingTitle,
		RawTranscript:      rawSegments,
		ReadableTranscript: readableBlocks,
	}
	readyBytes, err := json.Marshal(readyEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal transcript.ready event: %w", err)
	}

	err = c.readyWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.Itoa(botStatus.UserID)),
		Value: readyBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to publish transcript.ready: %w", err)
	}

	log.Printf("Published transcript.ready for bot %s (user %d, meeting '%s').", botID, botStatus.UserID, botStatus.MeetingTitle)
	return nil
}

func (c *TranscriptEventsConsumer) handleTranscriptFailed(ctx context.Context, botID string, failureSubCode string) error {
	log.Printf("Processing transcript.failed for bot %s, failure: %s.", botID, failureSubCode)

	botStatus, err := c.fetchBotStatus(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to resolve bot info for bot %s: %w", botID, err)
	}

	_, err = c.pool.Exec(ctx,
		`INSERT INTO transcripts (bot_id, user_id, meeting_title, status, failure_sub_code)
		 VALUES ($1, $2, $3, 'failed', $4)
		 ON CONFLICT (bot_id) DO UPDATE SET
		   status = 'failed',
		   failure_sub_code = $4`,
		botID, botStatus.UserID, botStatus.MeetingTitle, failureSubCode,
	)
	if err != nil {
		return fmt.Errorf("failed to store transcript failure: %w", err)
	}

	failedEvent := model.TranscriptFailedEvent{
		UserID:         botStatus.UserID,
		BotID:          botID,
		MeetingTitle:   botStatus.MeetingTitle,
		FailureSubCode: failureSubCode,
	}
	failedBytes, err := json.Marshal(failedEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal transcript.failed event: %w", err)
	}

	err = c.failedWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.Itoa(botStatus.UserID)),
		Value: failedBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to publish transcript.failed: %w", err)
	}

	log.Printf("Published transcript.failed for bot %s (user %d).", botID, botStatus.UserID)
	return nil
}

func (c *TranscriptEventsConsumer) fetchBotStatus(ctx context.Context, botID string) (*model.BotStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.botServiceURL+"/internal/bots/"+botID+"/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot status request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bot status request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bot status request returned %d: %s", resp.StatusCode, string(body))
	}

	var botStatus model.BotStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&botStatus); err != nil {
		return nil, fmt.Errorf("failed to decode bot status response: %w", err)
	}

	return &botStatus, nil
}
