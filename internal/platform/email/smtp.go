package email

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"

	"github.com/itkln/github-subscription/internal/config"
)

type Sender interface {
	SendHTML(ctx context.Context, to, subject, body string) error
}

type SMTPClient interface {
	SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

type SMTPSender struct {
	logger *slog.Logger
	smtp   SMTPClient
	cfg    config.SMTPConfig
}

type client struct{}

func (c *client) SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	return smtp.SendMail(addr, a, from, to, msg)
}

func NewSMTPSender(logger *slog.Logger, cfg config.SMTPConfig) *SMTPSender {
	return &SMTPSender{
		logger: logger,
		smtp:   &client{},
		cfg:    cfg,
	}
}

func (s *SMTPSender) SendHTML(ctx context.Context, to, subject, body string) error {
	_ = ctx

	address := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	message := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.cfg.From,
		to,
		subject,
		body,
	))

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	if err := s.smtp.SendMail(address, auth, s.cfg.From, []string{to}, message); err != nil {
		s.logger.Error("send email failed", "to", to, "subject", subject, "error", err)
		return err
	}

	s.logger.Info("email sent", "to", to, "subject", subject)
	return nil
}
