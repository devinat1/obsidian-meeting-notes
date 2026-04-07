// services/gateway/internal/proxy/proxy_test.go
package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/proxy"
)

func TestReverseProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/calendar/meetings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meetings":[]}`))
	}))
	defer backend.Close()

	p := proxy.New(proxy.NewParams{
		TargetURL: backend.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/meetings", nil)
	w := httptest.NewRecorder()

	p.Forward(w, req, "/internal/calendar/meetings")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != `{"meetings":[]}` {
		t.Fatalf("unexpected body: %s", string(body))
	}
}
