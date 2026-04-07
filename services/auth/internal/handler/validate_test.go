// services/auth/internal/handler/validate_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func TestValidateHandler_MissingAuth(t *testing.T) {
	h := handler.NewValidateHandler(handler.ValidateHandlerParams{
		DB: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/auth/validate", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
