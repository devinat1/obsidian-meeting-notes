// services/gateway/internal/middleware/auth_test.go
package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/middleware"
)

func TestAuthMiddleware_ValidKey(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"user_id": 7})
	}))
	defer authServer.Close()

	client := authclient.New(authclient.NewParams{
		AuthServiceURL: authServer.URL,
	})

	handler := middleware.Auth(client)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserIDFromContext(r.Context())
		if userID != 7 {
			t.Fatalf("expected user_id 7, got %d", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	client := authclient.New(authclient.NewParams{
		AuthServiceURL: "http://localhost:0",
	})

	handler := middleware.Auth(client)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
