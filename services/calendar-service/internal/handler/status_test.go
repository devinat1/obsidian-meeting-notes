// services/calendar-service/internal/handler/status_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusHandler_MissingUserID(t *testing.T) {
	h := &StatusHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/status", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestStatusHandler_InvalidUserID(t *testing.T) {
	h := &StatusHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/status?user_id=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
