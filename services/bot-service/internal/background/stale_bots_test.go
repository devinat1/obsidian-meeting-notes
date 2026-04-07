package background

import (
	"testing"
	"time"
)

func TestStaleBotThreshold(t *testing.T) {
	if staleBotThreshold != 3*time.Hour {
		t.Errorf("expected stale bot threshold of 3 hours, got %v", staleBotThreshold)
	}
}

func TestStaleCheckInterval(t *testing.T) {
	if staleCheckInterval != 15*time.Minute {
		t.Errorf("expected stale check interval of 15 minutes, got %v", staleCheckInterval)
	}
}
