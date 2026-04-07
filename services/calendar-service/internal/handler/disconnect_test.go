// services/calendar-service/internal/handler/disconnect_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisconnectHandler_MissingUserID(t *testing.T) {
	h := &DisconnectHandler{}

	req := httptest.NewRequest(http.MethodDelete, "/internal/calendar/disconnect", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestDisconnectHandler_InvalidUserID(t *testing.T) {
	h := &DisconnectHandler{}

	req := httptest.NewRequest(http.MethodDelete, "/internal/calendar/disconnect?user_id=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
