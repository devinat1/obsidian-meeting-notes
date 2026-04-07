// services/calendar-service/internal/config/config.go
package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	KafkaBroker          string
	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURI    string
	EncryptionKey        []byte // 32 bytes for AES-256
	DefaultPollInterval  int    // minutes
	DefaultBotJoinBefore int    // minutes
}

func Load() (*Config, error) {
	encKeyHex := os.Getenv("ENCRYPTION_KEY")
	if encKeyHex == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required (64-char hex string for AES-256).")
	}
	encKey, err := hex.DecodeString(encKeyHex)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be valid hex: %w.", err)
	}
	if len(encKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes (64 hex chars), got %d bytes.", len(encKey))
	}

	c := &Config{
		Port:                 getEnvOrDefault("PORT", "8081"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		KafkaBroker:          getEnvOrDefault("KAFKA_BROKER", "kafka.liteagent.svc.cluster.local:9092"),
		GoogleClientID:       os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:    getEnvOrDefault("GOOGLE_REDIRECT_URI", "https://api.liteagent.net/api/auth/calendar/callback"),
		EncryptionKey:        encKey,
		DefaultPollInterval:  parseIntOrDefault("DEFAULT_POLL_INTERVAL", 3),
		DefaultBotJoinBefore: parseIntOrDefault("DEFAULT_BOT_JOIN_BEFORE", 1),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required.")
	}
	if c.GoogleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is required.")
	}
	if c.GoogleClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET is required.")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
