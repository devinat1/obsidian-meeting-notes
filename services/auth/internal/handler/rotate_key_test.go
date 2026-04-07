// services/auth/internal/handler/rotate_key_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func TestRotateKeyHandler_MissingAuth(t *testing.T) {
	h := handler.NewRotateKeyHandler(handler.RotateKeyHandlerParams{
		DB: nil,
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/auth/rotate-key", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
