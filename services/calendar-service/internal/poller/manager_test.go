// services/calendar-service/internal/poller/manager_test.go
package poller

import (
	"testing"
)

func TestManager_ActiveUserCount_StartsAtZero(t *testing.T) {
	m := NewManager(NewManagerParams{})

	if m.ActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users, got %d.", m.ActiveUserCount())
	}
}

func TestManager_IsActive_ReturnsFalseForUnknownUser(t *testing.T) {
	m := NewManager(NewManagerParams{})

	if m.IsActive(999) {
		t.Error("Expected IsActive to return false for unknown user.")
	}
}

func TestManager_StopForUser_NoopForUnknownUser(t *testing.T) {
	m := NewManager(NewManagerParams{})

	// Should not panic.
	m.StopForUser(999)

	if m.ActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users after noop stop, got %d.", m.ActiveUserCount())
	}
}

func TestManager_StopAll_EmptyManager(t *testing.T) {
	m := NewManager(NewManagerParams{})

	// Should not panic.
	m.StopAll()

	if m.ActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users after StopAll, got %d.", m.ActiveUserCount())
	}
}
