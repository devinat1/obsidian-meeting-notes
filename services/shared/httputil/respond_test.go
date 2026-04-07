package httputil_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	payload := map[string]string{"message": "hello"}
	httputil.RespondJSON(w, http.StatusOK, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}
	expected := `{"message":"hello"}` + "\n"
	if w.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, w.Body.String())
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.RespondError(w, http.StatusBadRequest, "bad input")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
	expected := `{"error":"bad input"}` + "\n"
	if w.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, w.Body.String())
	}
}
