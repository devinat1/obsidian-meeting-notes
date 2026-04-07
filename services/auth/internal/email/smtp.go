// services/auth/internal/email/smtp.go
package email

import (
	"fmt"
	"net/smtp"
)

type Sender struct {
	host string
	port string
	user string
	pass string
	from string
}

type NewSenderParams struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func NewSender(params NewSenderParams) *Sender {
	return &Sender{
		host: params.Host,
		port: params.Port,
		user: params.User,
		pass: params.Pass,
		from: params.From,
	}
}

func (s *Sender) SendVerificationEmail(params SendVerificationEmailParams) error {
	subject := "Verify your email - Obsidian Meeting Notes"
	body := fmt.Sprintf(
		"Verify your email by clicking the link below:\n\n%s/api/auth/verify?token=%s\n\nThis link expires in 24 hours.",
		params.BaseURL,
		params.Token,
	)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from,
		params.To,
		subject,
		body,
	)

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	if err := smtp.SendMail(addr, auth, s.from, []string{params.To}, []byte(msg)); err != nil {
		return fmt.Errorf("sending verification email to %s: %w", params.To, err)
	}

	return nil
}

type SendVerificationEmailParams struct {
	To      string
	Token   string
	BaseURL string
}
