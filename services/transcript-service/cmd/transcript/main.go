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

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/consumer"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/recall"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/router"
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

	readyWriter := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        "transcript.ready",
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
	}
	defer readyWriter.Close()

	failedWriter := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        "transcript.failed",
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
	}
	defer failedWriter.Close()

	transcriptEventsReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "transcript.events",
		GroupID:  "transcript-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer transcriptEventsReader.Close()

	transcriptEventsConsumer := consumer.NewTranscriptEventsConsumer(consumer.NewTranscriptEventsConsumerParams{
		Reader:        transcriptEventsReader,
		Pool:          pool,
		RecallClient:  recallClient,
		ReadyWriter:   readyWriter,
		FailedWriter:  failedWriter,
		BotServiceURL: cfg.BotServiceInternalURL,
	})

	go transcriptEventsConsumer.Start(ctx)

	getTranscriptHandler := handler.NewGetTranscriptHandler(handler.NewGetTranscriptHandlerParams{Pool: pool})

	r := router.New(router.NewParams{
		GetTranscriptHandler: getTranscriptHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Transcript service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down transcript service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
