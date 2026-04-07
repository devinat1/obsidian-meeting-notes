// services/gateway/cmd/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/config"
	gatewaykafka "github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/middleware"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/proxy"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/ratelimit"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Loading config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authClient := authclient.New(authclient.NewParams{
		AuthServiceURL: cfg.AuthServiceURL,
	})

	hub := sse.NewHub()

	brokers := gatewaykafka.BrokersFromString(cfg.KafkaBrokers)
	gatewaykafka.StartConsumers(ctx, gatewaykafka.ConsumerGroupParams{
		Brokers: brokers,
		Hub:     hub,
	})

	authProxy := proxy.New(proxy.NewParams{TargetURL: cfg.AuthServiceURL})
	calendarProxy := proxy.New(proxy.NewParams{TargetURL: cfg.CalendarServiceURL})
	botProxy := proxy.New(proxy.NewParams{TargetURL: cfg.BotServiceURL})
	transcriptProxy := proxy.New(proxy.NewParams{TargetURL: cfg.TranscriptServiceURL})
	webhookProxy := proxy.New(proxy.NewParams{TargetURL: cfg.WebhookServiceURL})

	registerLimiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     5.0 / 3600.0,
		Capacity: 5,
	})

	generalLimiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     60.0 / 60.0,
		Capacity: 60,
	})

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Post("/api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if !registerLimiter.Allow(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		authProxy.Forward(w, r, "/internal/auth/register")
	})

	r.Get("/api/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		authProxy.Forward(w, r, "/internal/auth/verify")
	})

	r.Post("/api/webhooks/recallai", func(w http.ResponseWriter, r *http.Request) {
		webhookProxy.Forward(w, r, "/webhooks/recallai")
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authClient))
		r.Use(ratelimit.Middleware(ratelimit.MiddlewareParams{
			Limiter: generalLimiter,
			KeyFunc: func(r *http.Request) string {
				return fmt.Sprintf("user-%d", middleware.UserIDFromContext(r.Context()))
			},
		}))

		r.Post("/api/auth/rotate-key", func(w http.ResponseWriter, r *http.Request) {
			authProxy.Forward(w, r, "/internal/auth/rotate-key")
		})

		r.Get("/api/auth/calendar/connect", func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.UserIDFromContext(r.Context())
			calendarProxy.Forward(w, r, fmt.Sprintf("/internal/calendar/auth-url?user_id=%d", userID))
		})

		r.Get("/api/auth/calendar/callback", func(w http.ResponseWriter, r *http.Request) {
			calendarProxy.Forward(w, r, "/internal/calendar/callback")
		})

		r.Get("/api/events/stream", sse.NewHandler(sse.NewHandlerParams{
			Hub: hub,
			UserIDFromContext: func(ctx context.Context) int {
				return middleware.UserIDFromContext(ctx)
			},
			HeartbeatInterval: 30 * time.Second,
		}).ServeHTTP)

		r.Get("/api/meetings", func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.UserIDFromContext(r.Context())
			calendarProxy.Forward(w, r, fmt.Sprintf("/internal/calendar/meetings?user_id=%d", userID))
		})

		r.Get("/api/bots/{botId}/transcript", func(w http.ResponseWriter, r *http.Request) {
			botID := chi.URLParam(r, "botId")
			transcriptProxy.Forward(w, r, fmt.Sprintf("/internal/transcripts/%s", botID))
		})

		r.Delete("/api/bots/{botId}", func(w http.ResponseWriter, r *http.Request) {
			botID := chi.URLParam(r, "botId")
			userID := middleware.UserIDFromContext(r.Context())
			botProxy.Forward(w, r, fmt.Sprintf("/internal/bots/%s?user_id=%d", botID, userID))
		})
	})

	r.Get("/api/events/stream", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, `{"error":"token query parameter required for SSE"}`, http.StatusUnauthorized)
			return
		}

		userID, err := authClient.Validate(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		sseHandler := sse.NewHandler(sse.NewHandlerParams{
			Hub: hub,
			UserIDFromContext: func(ctx context.Context) int {
				return userID
			},
			HeartbeatInterval: 30 * time.Second,
		})
		sseHandler.ServeHTTP(w, r)
	})

	_ = authProxy
	_ = calendarProxy
	_ = botProxy
	_ = transcriptProxy
	_ = webhookProxy
	_ = strconv.Itoa

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Gateway service listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down gateway service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Gateway service stopped.")
}
