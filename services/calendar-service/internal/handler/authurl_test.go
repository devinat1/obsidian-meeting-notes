// services/calendar-service/internal/handler/authurl_test.go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
)

func testOAuth() *oauth.GoogleOAuth {
	return oauth.NewGoogleOAuth(oauth.NewGoogleOAuthParams{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "https://api.liteagent.net/api/auth/calendar/callback",
	})
}

func TestAuthURLHandler_Success(t *testing.T) {
	h := NewAuthURLHandler(testOAuth())

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/auth-url?user_id=42", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d.", rec.Code)
	}

	var resp model.AuthURLResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v.", err)
	}

	if !strings.Contains(resp.URL, "accounts.google.com") {
		t.Error("URL should contain Google accounts domain.")
	}
	if !strings.Contains(resp.URL, "42") {
		t.Error("URL state should contain user_id.")
	}
}

func TestAuthURLHandler_MissingUserID(t *testing.T) {
	h := NewAuthURLHandler(testOAuth())

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/auth-url", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestAuthURLHandler_InvalidUserID(t *testing.T) {
	h := NewAuthURLHandler(testOAuth())

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/auth-url?user_id=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
