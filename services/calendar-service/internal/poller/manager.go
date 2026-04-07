// services/calendar-service/internal/poller/manager.go
package poller

import (
	"context"
	"log/slog"
	"sync"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/calendar"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type Manager struct {
	mu              sync.Mutex
	workers         map[int]context.CancelFunc // user_id -> cancel func
	calendarClient  *calendar.Client
	meetingStore    *store.MeetingStore
	connectionStore *store.ConnectionStore
	producer        *kafka.Producer
	googleOAuth     *oauth.GoogleOAuth
	encryptor       *encryption.Encryptor
}

type NewManagerParams struct {
	CalendarClient  *calendar.Client
	MeetingStore    *store.MeetingStore
	ConnectionStore *store.ConnectionStore
	Producer        *kafka.Producer
	GoogleOAuth     *oauth.GoogleOAuth
	Encryptor       *encryption.Encryptor
}

func NewManager(params NewManagerParams) *Manager {
	return &Manager{
		workers:         map[int]context.CancelFunc{},
		calendarClient:  params.CalendarClient,
		meetingStore:    params.MeetingStore,
		connectionStore: params.ConnectionStore,
		producer:        params.Producer,
		googleOAuth:     params.GoogleOAuth,
		encryptor:       params.Encryptor,
	}
}

// StartAll loads all calendar connections from DB and starts a poller for each.
func (m *Manager) StartAll(ctx context.Context) error {
	connections, err := m.connectionStore.GetAll(ctx)
	if err != nil {
		return err
	}

	for _, conn := range connections {
		m.startWorker(ctx, conn)
	}

	slog.Info("Poller manager started all workers.", "count", len(connections))
	return nil
}

// StartForUser starts (or restarts) the poller for a specific user.
func (m *Manager) StartForUser(ctx context.Context, conn model.CalendarConnection) {
	m.StopForUser(conn.UserID)
	m.startWorker(ctx, conn)
}

// StopForUser stops the poller for a specific user.
func (m *Manager) StopForUser(userID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, exists := m.workers[userID]; exists {
		cancel()
		delete(m.workers, userID)
		slog.Info("Stopped poller for user.", "user_id", userID)
	}
}

// StopAll stops all active pollers.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for userID, cancel := range m.workers {
		cancel()
		slog.Info("Stopped poller for user.", "user_id", userID)
	}
	m.workers = map[int]context.CancelFunc{}
}

// ActiveUserCount returns the number of active poller goroutines.
func (m *Manager) ActiveUserCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.workers)
}

// IsActive returns whether a poller is running for the given user.
func (m *Manager) IsActive(userID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.workers[userID]
	return exists
}

func (m *Manager) startWorker(ctx context.Context, conn model.CalendarConnection) {
	// Decrypt the refresh token.
	refreshToken, err := m.encryptor.Decrypt(conn.RefreshToken)
	if err != nil {
		slog.Error("Failed to decrypt refresh token for user, skipping.", "user_id", conn.UserID, "error", err)
		return
	}

	tokenSource := m.googleOAuth.TokenSource(ctx, refreshToken)

	workerCtx, cancel := context.WithCancel(ctx)

	m.mu.Lock()
	m.workers[conn.UserID] = cancel
	m.mu.Unlock()

	worker := NewWorker(NewWorkerParams{
		UserID:               conn.UserID,
		PollIntervalMinutes:  conn.PollIntervalMinutes,
		BotJoinBeforeMinutes: conn.BotJoinBeforeMinutes,
		TokenSource:          tokenSource,
		CalendarClient:       m.calendarClient,
		MeetingStore:         m.meetingStore,
		Producer:             m.producer,
	})

	go worker.Run(workerCtx)
}
