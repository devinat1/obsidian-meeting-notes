// services/calendar-service/internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestLoad_AllRequiredVars(t *testing.T) {
	// 32-byte key as 64 hex chars
	testKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	t.Setenv("DATABASE_URL", "postgres://localhost:5433/calendar_service")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	t.Setenv("ENCRYPTION_KEY", testKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got: %v.", err)
	}

	if cfg.Port != "8081" {
		t.Errorf("Expected default port 8081, got %s.", cfg.Port)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("Expected 32-byte encryption key, got %d bytes.", len(cfg.EncryptionKey))
	}
	if cfg.DefaultPollInterval != 3 {
		t.Errorf("Expected default poll interval 3, got %d.", cfg.DefaultPollInterval)
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	testKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("ENCRYPTION_KEY", testKey)
	t.Setenv("GOOGLE_CLIENT_ID", "test")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test")
	os.Unsetenv("DATABASE_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for missing DATABASE_URL.")
	}
}

func TestLoad_InvalidEncryptionKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("GOOGLE_CLIENT_ID", "test")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test")
	t.Setenv("ENCRYPTION_KEY", "tooshort")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for invalid ENCRYPTION_KEY.")
	}
}
