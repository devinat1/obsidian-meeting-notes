// services/calendar-service/internal/handler/callback_test.go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallbackHandler_MissingCode(t *testing.T) {
	h := &CallbackHandler{}

	body, _ := json.Marshal(map[string]interface{}{"user_id": 42})
	req := httptest.NewRequest(http.MethodPost, "/internal/calendar/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestCallbackHandler_MissingUserID(t *testing.T) {
	h := &CallbackHandler{}

	body, _ := json.Marshal(map[string]interface{}{"code": "auth-code-123"})
	req := httptest.NewRequest(http.MethodPost, "/internal/calendar/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestCallbackHandler_InvalidBody(t *testing.T) {
	h := &CallbackHandler{}

	req := httptest.NewRequest(http.MethodPost, "/internal/calendar/callback", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
