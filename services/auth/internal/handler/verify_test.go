// services/auth/internal/handler/verify_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func TestVerifyHandler_MissingToken(t *testing.T) {
	h := handler.NewVerifyHandler(handler.VerifyHandlerParams{
		DB: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/auth/verify", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
