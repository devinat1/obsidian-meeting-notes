// services/auth/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	BaseURL     string
}

func Load() (*Config, error) {
	c := &Config{
		Port:        getEnvOrDefault("PORT", "8081"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		SMTPHost:    os.Getenv("SMTP_HOST"),
		SMTPPort:    getEnvOrDefault("SMTP_PORT", "587"),
		SMTPUser:    os.Getenv("SMTP_USER"),
		SMTPPass:    os.Getenv("SMTP_PASS"),
		SMTPFrom:    os.Getenv("SMTP_FROM"),
		BaseURL:     os.Getenv("BASE_URL"),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}
	if c.SMTPFrom == "" {
		return nil, fmt.Errorf("SMTP_FROM is required")
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
