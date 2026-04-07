// services/calendar-service/internal/poller/worker_test.go
package poller

import (
	"testing"
	"time"
)

func TestNewWorker_SetsIntervals(t *testing.T) {
	w := NewWorker(NewWorkerParams{
		UserID:               42,
		PollIntervalMinutes:  5,
		BotJoinBeforeMinutes: 2,
		// TokenSource, CalendarClient, MeetingStore, Producer left nil for unit test.
	})

	if w.userID != 42 {
		t.Errorf("Expected userID 42, got %d.", w.userID)
	}
	if w.pollInterval != 5*time.Minute {
		t.Errorf("Expected 5m poll interval, got %v.", w.pollInterval)
	}
	if w.botJoinBefore != 2*time.Minute {
		t.Errorf("Expected 2m bot join before, got %v.", w.botJoinBefore)
	}
}

func TestDispatchThreshold_Logic(t *testing.T) {
	botJoinBefore := 1 * time.Minute
	startTime := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)
	threshold := startTime.Add(-botJoinBefore)

	// 2 minutes before start: should NOT dispatch yet.
	twoMinBefore := startTime.Add(-2 * time.Minute)
	if !twoMinBefore.Before(threshold) {
		t.Error("2 minutes before start should be before the dispatch threshold.")
	}

	// 30 seconds before start: should dispatch.
	thirtySecBefore := startTime.Add(-30 * time.Second)
	if thirtySecBefore.Before(threshold) {
		t.Error("30 seconds before start should be after the dispatch threshold.")
	}

	// At start time: should dispatch.
	if startTime.Before(threshold) {
		t.Error("Start time should be after the dispatch threshold.")
	}
}
