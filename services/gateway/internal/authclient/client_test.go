// services/gateway/internal/authclient/client_test.go
package authclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
)

func TestValidate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/auth/validate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"user_id": 42})
	}))
	defer server.Close()

	client := authclient.New(authclient.NewParams{
		AuthServiceURL: server.URL,
	})

	userID, err := client.Validate("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != 42 {
		t.Fatalf("expected user_id 42, got %d", userID)
	}
}

func TestValidate_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid API key."})
	}))
	defer server.Close()

	client := authclient.New(authclient.NewParams{
		AuthServiceURL: server.URL,
	})

	_, err := client.Validate("bad-key")
	if err == nil {
		t.Fatal("expected error for unauthorized key")
	}
}
