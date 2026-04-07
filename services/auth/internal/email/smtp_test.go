// services/auth/internal/email/smtp_test.go
package email_test

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/email"
)

func TestNewSender(t *testing.T) {
	s := email.NewSender(email.NewSenderParams{
		Host: "smtp.example.com",
		Port: "587",
		User: "user",
		Pass: "pass",
		From: "noreply@example.com",
	})
	if s == nil {
		t.Fatal("expected non-nil sender")
	}
}
