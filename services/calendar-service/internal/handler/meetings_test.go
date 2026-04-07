// services/calendar-service/internal/handler/meetings_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMeetingsHandler_MissingUserID(t *testing.T) {
	h := &MeetingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/meetings", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestMeetingsHandler_InvalidUserID(t *testing.T) {
	h := &MeetingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/meetings?user_id=xyz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
