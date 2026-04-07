// services/calendar-service/cmd/calendar/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/calendar"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/poller"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("Calendar service starting.")

	// Load config.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config.", "error", err)
		os.Exit(1)
	}

	// Initialize database.
	ctx := context.Background()

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("Failed to run migrations.", "error", err)
		os.Exit(1)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database.", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize encryption.
	encryptor, err := encryption.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		slog.Error("Failed to initialize encryptor.", "error", err)
		os.Exit(1)
	}

	// Initialize stores.
	connectionStore := store.NewConnectionStore(pool, encryptor)
	meetingStore := store.NewMeetingStore(pool)

	// Initialize Google OAuth.
	googleOAuth := oauth.NewGoogleOAuth(oauth.NewGoogleOAuthParams{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURI:  cfg.GoogleRedirectURI,
	})

	// Initialize Kafka producer.
	producer := kafka.NewProducer(cfg.KafkaBroker)
	defer producer.Close()

	// Initialize calendar client.
	calendarClient := calendar.NewClient()

	// Initialize poller manager.
	pollerManager := poller.NewManager(poller.NewManagerParams{
		CalendarClient:  calendarClient,
		MeetingStore:    meetingStore,
		ConnectionStore: connectionStore,
		Producer:        producer,
		GoogleOAuth:     googleOAuth,
		Encryptor:       encryptor,
	})

	// Start all existing pollers.
	if err := pollerManager.StartAll(ctx); err != nil {
		slog.Error("Failed to start pollers.", "error", err)
		os.Exit(1)
	}
	defer pollerManager.StopAll()

	// Initialize handlers.
	authURLHandler := handler.NewAuthURLHandler(googleOAuth)
	callbackHandler := handler.NewCallbackHandler(handler.NewCallbackHandlerParams{
		GoogleOAuth:          googleOAuth,
		ConnectionStore:      connectionStore,
		PollerManager:        pollerManager,
		DefaultPollInterval:  cfg.DefaultPollInterval,
		DefaultBotJoinBefore: cfg.DefaultBotJoinBefore,
	})
	disconnectHandler := handler.NewDisconnectHandler(handler.NewDisconnectHandlerParams{
		ConnectionStore: connectionStore,
		MeetingStore:    meetingStore,
		PollerManager:   pollerManager,
	})
	meetingsHandler := handler.NewMeetingsHandler(meetingStore)
	statusHandler := handler.NewStatusHandler(connectionStore)

	// Set up router.
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	r.Route("/internal/calendar", func(r chi.Router) {
		r.Get("/auth-url", authURLHandler.ServeHTTP)
		r.Post("/callback", callbackHandler.ServeHTTP)
		r.Delete("/disconnect", disconnectHandler.ServeHTTP)
		r.Get("/meetings", meetingsHandler.ServeHTTP)
		r.Get("/status", statusHandler.ServeHTTP)
	})

	// Start server.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Listening.", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed.", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down.")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error.", "error", err)
	}

	slog.Info("Calendar service stopped.")
}
