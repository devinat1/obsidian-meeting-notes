package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/background"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/consumer"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/recall"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	recallClient := recall.NewClient(recall.NewClientParams{
		Region: cfg.RecallRegion,
		APIKey: cfg.RecallAPIKey,
	})

	brokers := strings.Split(cfg.KafkaBrokers, ",")

	statusWriter := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        "bot.status",
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
	}
	defer statusWriter.Close()

	meetingUpcomingReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "meeting.upcoming",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer meetingUpcomingReader.Close()

	meetingCancelledReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "meeting.cancelled",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer meetingCancelledReader.Close()

	botEventsReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "bot.events",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer botEventsReader.Close()

	recordingEventsReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "recording.events",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer recordingEventsReader.Close()

	meetingUpcomingConsumer := consumer.NewMeetingUpcomingConsumer(consumer.NewMeetingUpcomingConsumerParams{
		Reader:       meetingUpcomingReader,
		Pool:         pool,
		RecallClient: recallClient,
		StatusWriter: statusWriter,
	})

	meetingCancelledConsumer := consumer.NewMeetingCancelledConsumer(consumer.NewMeetingCancelledConsumerParams{
		Reader:       meetingCancelledReader,
		Pool:         pool,
		RecallClient: recallClient,
		StatusWriter: statusWriter,
	})

	botEventsConsumer := consumer.NewBotEventsConsumer(consumer.NewBotEventsConsumerParams{
		Reader:       botEventsReader,
		Pool:         pool,
		StatusWriter: statusWriter,
	})

	recordingEventsConsumer := consumer.NewRecordingEventsConsumer(consumer.NewRecordingEventsConsumerParams{
		Reader: recordingEventsReader,
		Pool:   pool,
	})

	staleBotCleanup := background.NewStaleBotCleanup(background.NewStaleBotCleanupParams{
		Pool:         pool,
		StatusWriter: statusWriter,
	})

	go meetingUpcomingConsumer.Start(ctx)
	go meetingCancelledConsumer.Start(ctx)
	go botEventsConsumer.Start(ctx)
	go recordingEventsConsumer.Start(ctx)
	go staleBotCleanup.Start(ctx)

	botStatusHandler := handler.NewBotStatusHandler(handler.NewBotStatusHandlerParams{Pool: pool})
	deleteBotHandler := handler.NewDeleteBotHandler(handler.NewDeleteBotHandlerParams{
		Pool:         pool,
		RecallClient: recallClient,
		StatusWriter: statusWriter,
	})

	r := router.New(router.NewParams{
		BotStatusHandler: botStatusHandler,
		DeleteBotHandler: deleteBotHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Bot service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down bot service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
