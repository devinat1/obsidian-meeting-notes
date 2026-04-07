// services/auth/internal/background/cleanup_test.go
package background_test

import (
	"context"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/background"
)

func TestStartUnverifiedUserCleanup_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		background.StartUnverifiedUserCleanup(ctx, nil)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup goroutine did not stop after context cancellation")
	}
}
