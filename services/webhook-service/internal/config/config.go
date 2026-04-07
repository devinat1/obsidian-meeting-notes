package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                              string
	KafkaBrokers                      string
	RecallWorkspaceVerificationSecret string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                              getEnvOrDefault("PORT", "8080"),
		KafkaBrokers:                      os.Getenv("KAFKA_BROKERS"),
		RecallWorkspaceVerificationSecret: os.Getenv("RECALL_WORKSPACE_VERIFICATION_SECRET"),
	}

	if c.KafkaBrokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.RecallWorkspaceVerificationSecret == "" {
		return nil, fmt.Errorf("RECALL_WORKSPACE_VERIFICATION_SECRET is required")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
