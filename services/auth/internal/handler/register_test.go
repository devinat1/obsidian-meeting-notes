// services/auth/internal/handler/register_test.go
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/model"
)

func TestRegisterHandler_MissingEmail(t *testing.T) {
	h := handler.NewRegisterHandler(handler.RegisterHandlerParams{
		DB:          nil,
		EmailSender: nil,
		BaseURL:     "https://api.liteagent.net",
	})

	body, _ := json.Marshal(model.RegisterRequest{Email: ""})
	req := httptest.NewRequest(http.MethodPost, "/internal/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegisterHandler_InvalidEmail(t *testing.T) {
	h := handler.NewRegisterHandler(handler.RegisterHandlerParams{
		DB:          nil,
		EmailSender: nil,
		BaseURL:     "https://api.liteagent.net",
	})

	body, _ := json.Marshal(model.RegisterRequest{Email: "not-an-email"})
	req := httptest.NewRequest(http.MethodPost, "/internal/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
