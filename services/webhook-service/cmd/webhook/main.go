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

	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	topics := []string{"bot.events", "recording.events", "transcript.events"}
	producers := make(map[string]*kafkago.Writer)

	for _, topic := range topics {
		producers[topic] = &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafkago.LeastBytes{},
			RequiredAcks: kafkago.RequireOne,
			BatchTimeout: 10 * time.Millisecond,
		}
	}

	webhookHandler := handler.NewWebhookHandler(handler.NewWebhookHandlerParams{
		Secret:    cfg.RecallWorkspaceVerificationSecret,
		Producers: producers,
	})

	r := router.New(router.NewParams{
		WebhookHandler: webhookHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Webhook service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down webhook service.")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for topic, producer := range producers {
		if err := producer.Close(); err != nil {
			log.Printf("Failed to close producer for topic %s: %v.", topic, err)
		}
	}

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
