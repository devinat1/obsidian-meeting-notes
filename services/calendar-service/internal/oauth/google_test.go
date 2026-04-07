// services/calendar-service/internal/oauth/google_test.go
package oauth

import (
	"strings"
	"testing"
)

func TestAuthURL_ContainsRequiredParams(t *testing.T) {
	g := NewGoogleOAuth(NewGoogleOAuthParams{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "https://api.liteagent.net/api/auth/calendar/callback",
	})

	url := g.AuthURL("user-state-123")

	if !strings.Contains(url, "accounts.google.com") {
		t.Error("Auth URL should point to Google accounts.")
	}
	if !strings.Contains(url, "test-client-id") {
		t.Error("Auth URL should contain client ID.")
	}
	if !strings.Contains(url, "user-state-123") {
		t.Error("Auth URL should contain state parameter.")
	}
	if !strings.Contains(url, "access_type=offline") {
		t.Error("Auth URL should request offline access for refresh token.")
	}
	if !strings.Contains(url, "calendar.readonly") {
		t.Error("Auth URL should request calendar.readonly scope.")
	}
}

func TestNewGoogleOAuth_SetsConfig(t *testing.T) {
	g := NewGoogleOAuth(NewGoogleOAuthParams{
		ClientID:     "my-id",
		ClientSecret: "my-secret",
		RedirectURI:  "https://example.com/callback",
	})

	cfg := g.Config()
	if cfg.ClientID != "my-id" {
		t.Errorf("Expected ClientID 'my-id', got %q.", cfg.ClientID)
	}
	if cfg.RedirectURL != "https://example.com/callback" {
		t.Errorf("Expected RedirectURL, got %q.", cfg.RedirectURL)
	}
}
