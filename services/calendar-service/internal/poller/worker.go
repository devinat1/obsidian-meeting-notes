// services/calendar-service/internal/poller/worker.go
package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/calendar"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
	"golang.org/x/oauth2"
)

type Worker struct {
	userID         int
	pollInterval   time.Duration
	botJoinBefore  time.Duration
	tokenSource    oauth2.TokenSource
	calendarClient *calendar.Client
	meetingStore   *store.MeetingStore
	producer       *kafka.Producer
}

type NewWorkerParams struct {
	UserID               int
	PollIntervalMinutes  int
	BotJoinBeforeMinutes int
	TokenSource          oauth2.TokenSource
	CalendarClient       *calendar.Client
	MeetingStore         *store.MeetingStore
	Producer             *kafka.Producer
}

func NewWorker(params NewWorkerParams) *Worker {
	return &Worker{
		userID:         params.UserID,
		pollInterval:   time.Duration(params.PollIntervalMinutes) * time.Minute,
		botJoinBefore:  time.Duration(params.BotJoinBeforeMinutes) * time.Minute,
		tokenSource:    params.TokenSource,
		calendarClient: params.CalendarClient,
		meetingStore:   params.MeetingStore,
		producer:       params.Producer,
	}
}

// Run starts the poll loop. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("Poller worker started.", "user_id", w.userID, "interval", w.pollInterval)

	// Poll immediately on start, then on interval.
	w.poll(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Poller worker stopped.", "user_id", w.userID)
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	slog.Debug("Polling calendar.", "user_id", w.userID)

	now := time.Now()
	timeMin := now
	timeMax := now.Add(2 * time.Hour)

	// Fetch upcoming events from Google Calendar.
	events, err := w.calendarClient.FetchUpcomingEvents(ctx, calendar.FetchEventsParams{
		TokenSource: w.tokenSource,
		TimeMin:     timeMin,
		TimeMax:     timeMax,
	})
	if err != nil {
		slog.Error("Failed to fetch calendar events.", "user_id", w.userID, "error", err)
		return
	}

	// Track which event IDs we see in this poll cycle.
	seenEventIDs := map[string]bool{}

	for _, event := range events {
		seenEventIDs[event.CalendarEventID] = true

		// Upsert the meeting (updates title/time/url/attendees if changed).
		meeting, _, err := w.meetingStore.Upsert(ctx, store.UpsertMeetingParams{
			UserID:          w.userID,
			CalendarEventID: event.CalendarEventID,
			Title:           event.Title,
			StartTime:       event.StartTime,
			EndTime:         event.EndTime,
			MeetingURL:      event.MeetingURL,
			Attendees:       event.Attendees,
		})
		if err != nil {
			slog.Error("Failed to upsert tracked meeting.", "user_id", w.userID, "event_id", event.CalendarEventID, "error", err)
			continue
		}

		// Check if we should dispatch the bot.
		if meeting.BotDispatched || meeting.Cancelled {
			continue
		}

		dispatchThreshold := event.StartTime.Add(-w.botJoinBefore)
		if now.Before(dispatchThreshold) {
			continue
		}

		// Publish meeting.upcoming to Kafka.
		publishErr := w.producer.PublishMeetingUpcoming(ctx, model.MeetingUpcomingEvent{
			UserID:          w.userID,
			CalendarEventID: event.CalendarEventID,
			MeetingURL:      event.MeetingURL,
			Title:           event.Title,
			StartTime:       event.StartTime,
		})
		if publishErr != nil {
			slog.Error("Failed to publish meeting.upcoming.", "user_id", w.userID, "event_id", event.CalendarEventID, "error", publishErr)
			continue
		}

		// Mark as dispatched.
		if err := w.meetingStore.MarkBotDispatched(ctx, w.userID, event.CalendarEventID); err != nil {
			slog.Error("Failed to mark bot dispatched.", "user_id", w.userID, "event_id", event.CalendarEventID, "error", err)
		}
	}

	// Detect cancelled/deleted events.
	w.detectCancellations(ctx, seenEventIDs, timeMin, timeMax)
}

func (w *Worker) detectCancellations(ctx context.Context, seenEventIDs map[string]bool, timeMin time.Time, timeMax time.Time) {
	// Get dispatched event IDs from DB.
	dispatchedIDs, err := w.meetingStore.GetDispatchedEventIDs(ctx, w.userID)
	if err != nil {
		slog.Error("Failed to get dispatched event IDs.", "user_id", w.userID, "error", err)
		return
	}

	// Also check the Google Calendar API for explicitly cancelled events.
	deletedIDs, err := w.calendarClient.FetchDeletedEventIDs(ctx, calendar.FetchEventsParams{
		TokenSource: w.tokenSource,
		TimeMin:     timeMin,
		TimeMax:     timeMax,
	})
	if err != nil {
		slog.Warn("Failed to fetch deleted event IDs.", "user_id", w.userID, "error", err)
	}

	deletedSet := map[string]bool{}
	for _, id := range deletedIDs {
		deletedSet[id] = true
	}

	// A dispatched event is considered cancelled if:
	// 1. It no longer appears in the non-cancelled events list AND
	// 2. It appears in the deleted events list from Google
	for eventID := range dispatchedIDs {
		if seenEventIDs[eventID] {
			continue
		}
		if !deletedSet[eventID] {
			continue
		}

		slog.Info("Detected cancelled meeting.", "user_id", w.userID, "event_id", eventID)

		publishErr := w.producer.PublishMeetingCancelled(ctx, model.MeetingCancelledEvent{
			UserID:          w.userID,
			CalendarEventID: eventID,
		})
		if publishErr != nil {
			slog.Error("Failed to publish meeting.cancelled.", "user_id", w.userID, "event_id", eventID, "error", publishErr)
			continue
		}

		if err := w.meetingStore.MarkCancelled(ctx, w.userID, eventID); err != nil {
			slog.Error("Failed to mark meeting cancelled.", "user_id", w.userID, "event_id", eventID, "error", err)
		}
	}
}
