package httputil_test

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

func TestParseJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	body := bytes.NewBufferString(`{"name":"alice"}`)
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/json")
	var p payload
	err := httputil.ParseJSON(r, &p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "alice" {
		t.Fatalf("expected name 'alice', got %q", p.Name)
	}
}

func TestParseJSONEmptyBody(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Content-Type", "application/json")
	var p payload
	err := httputil.ParseJSON(r, &p)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}
