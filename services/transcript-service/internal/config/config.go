package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	KafkaBrokers          string
	RecallRegion          string
	RecallAPIKey          string
	BotServiceInternalURL string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                  getEnvOrDefault("PORT", "8082"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		KafkaBrokers:          os.Getenv("KAFKA_BROKERS"),
		RecallRegion:          os.Getenv("RECALL_REGION"),
		RecallAPIKey:          os.Getenv("RECALL_API_KEY"),
		BotServiceInternalURL: getEnvOrDefault("BOT_SERVICE_INTERNAL_URL", "http://meeting-bot.liteagent.svc.cluster.local:8081"),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.KafkaBrokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.RecallRegion == "" {
		return nil, fmt.Errorf("RECALL_REGION is required")
	}
	if c.RecallAPIKey == "" {
		return nil, fmt.Errorf("RECALL_API_KEY is required")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
