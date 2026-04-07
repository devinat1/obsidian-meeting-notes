// services/gateway/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                string
	AuthServiceURL      string
	CalendarServiceURL  string
	BotServiceURL       string
	TranscriptServiceURL string
	WebhookServiceURL   string
	KafkaBrokers        string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                 getEnvOrDefault("PORT", "8080"),
		AuthServiceURL:       os.Getenv("AUTH_SERVICE_URL"),
		CalendarServiceURL:   os.Getenv("CALENDAR_SERVICE_URL"),
		BotServiceURL:        os.Getenv("BOT_SERVICE_URL"),
		TranscriptServiceURL: os.Getenv("TRANSCRIPT_SERVICE_URL"),
		WebhookServiceURL:    os.Getenv("WEBHOOK_SERVICE_URL"),
		KafkaBrokers:         os.Getenv("KAFKA_BROKERS"),
	}

	if c.AuthServiceURL == "" {
		return nil, fmt.Errorf("AUTH_SERVICE_URL is required")
	}
	if c.KafkaBrokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
