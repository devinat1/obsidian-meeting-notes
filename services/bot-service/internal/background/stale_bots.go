package background

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

const (
	staleCheckInterval = 15 * time.Minute
	staleBotThreshold  = 3 * time.Hour
)

type StaleBotCleanup struct {
	pool         *pgxpool.Pool
	statusWriter *kafkago.Writer
}

type NewStaleBotCleanupParams struct {
	Pool         *pgxpool.Pool
	StatusWriter *kafkago.Writer
}

func NewStaleBotCleanup(params NewStaleBotCleanupParams) *StaleBotCleanup {
	return &StaleBotCleanup{
		pool:         params.Pool,
		statusWriter: params.StatusWriter,
	}
}

func (s *StaleBotCleanup) Start(ctx context.Context) {
	log.Println("Starting stale bot cleanup goroutine (every 15 minutes).")
	ticker := time.NewTicker(staleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stale bot cleanup shutting down.")
			return
		case <-ticker.C:
			s.cleanup(ctx)
		}
	}
}

func (s *StaleBotCleanup) cleanup(ctx context.Context) {
	threshold := time.Now().Add(-staleBotThreshold)

	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, bot_id, meeting_title FROM bots
		 WHERE bot_status NOT IN ('done', 'fatal', 'cancelled')
		 AND created_at < $1`,
		threshold,
	)
	if err != nil {
		log.Printf("Failed to query stale bots: %v.", err)
		return
	}
	defer rows.Close()

	staleCount := 0
	for rows.Next() {
		var bot model.Bot
		if err := rows.Scan(&bot.ID, &bot.UserID, &bot.BotID, &bot.MeetingTitle); err != nil {
			log.Printf("Failed to scan stale bot row: %v.", err)
			continue
		}

		_, err := s.pool.Exec(ctx,
			"UPDATE bots SET bot_status = 'fatal', updated_at = NOW() WHERE id = $1",
			bot.ID,
		)
		if err != nil {
			log.Printf("Failed to mark bot %s as fatal: %v.", bot.BotID, err)
			continue
		}

		statusEvent := model.BotStatusEvent{
			UserID:       bot.UserID,
			BotID:        bot.BotID,
			BotStatus:    "fatal",
			MeetingTitle: bot.MeetingTitle,
		}
		statusBytes, _ := json.Marshal(statusEvent)
		s.statusWriter.WriteMessages(ctx, kafkago.Message{
			Key:   []byte(strconv.Itoa(bot.UserID)),
			Value: statusBytes,
		})

		staleCount++
		log.Printf("Marked bot %s as fatal (stale for >3 hours).", bot.BotID)
	}

	if staleCount > 0 {
		log.Printf("Cleaned up %d stale bots.", staleCount)
	}
}
