package notifier

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"path"
)

//go:embed templates/*.html.tmpl
var templateFS embed.FS

type ConfirmationSender interface {
	SendConfirmation(ctx context.Context, email, repo, token string) error
}

type ReleaseSender interface {
	SendReleaseNotification(ctx context.Context, email, repo, tag, token string) error
}

type EmailSender interface {
	SendHTML(ctx context.Context, to, subject, body string) error
}

type Service struct {
	logger           *slog.Logger
	sender           EmailSender
	publicURL        string
	confirmationTmpl *template.Template
	releaseTmpl      *template.Template
}

func NewService(logger *slog.Logger, sender EmailSender, publicURL string) (*Service, error) {
	confirmationTmpl, err := template.ParseFS(templateFS, "templates/confirmation.html.tmpl")
	if err != nil {
		logger.Error("parse confirmation template failed", "error", err)
		return nil, fmt.Errorf("parse confirmation template: %w", err)
	}

	releaseTmpl, err := template.ParseFS(templateFS, "templates/release.html.tmpl")
	if err != nil {
		logger.Error("parse release template failed", "error", err)
		return nil, fmt.Errorf("parse release template: %w", err)
	}

	return &Service{
		logger:           logger,
		sender:           sender,
		publicURL:        publicURL,
		confirmationTmpl: confirmationTmpl,
		releaseTmpl:      releaseTmpl,
	}, nil
}

func (s *Service) SendConfirmation(ctx context.Context, email, repo, token string) error {
	confirmURL := joinURL(s.publicURL, "/api/confirm/"+token)
	subject := "Confirm your GitHub release subscription"

	body, err := s.render(s.confirmationTmpl, struct {
		Repo       string
		ConfirmURL string
	}{
		Repo:       repo,
		ConfirmURL: confirmURL,
	})
	if err != nil {
		s.logger.Error("render confirmation email failed", "email", email, "repo", repo, "error", err)
		return err
	}

	if err := s.sender.SendHTML(ctx, email, subject, body); err != nil {
		s.logger.Error("send confirmation email failed", "email", email, "repo", repo, "error", err)
		return err
	}

	s.logger.Info("confirmation email prepared", "email", email, "repo", repo, "confirm_url", confirmURL)
	return nil
}

func (s *Service) SendReleaseNotification(ctx context.Context, email, repo, tag, token string) error {
	unsubscribeURL := joinURL(s.publicURL, "/api/unsubscribe/"+token)
	subject := fmt.Sprintf("New release for %s: %s", repo, tag)

	body, err := s.render(s.releaseTmpl, struct {
		Repo           string
		Tag            string
		UnsubscribeURL string
	}{
		Repo:           repo,
		Tag:            tag,
		UnsubscribeURL: unsubscribeURL,
	})
	if err != nil {
		s.logger.Error("render release notification failed", "email", email, "repo", repo, "tag", tag, "error", err)
		return err
	}

	if err := s.sender.SendHTML(ctx, email, subject, body); err != nil {
		s.logger.Error("send release notification failed", "email", email, "repo", repo, "tag", tag, "error", err)
		return err
	}

	s.logger.Info("release notification prepared", "email", email, "repo", repo, "tag", tag)
	return nil
}

func (s *Service) render(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		s.logger.Error("render html template failed", "error", err)
		return "", fmt.Errorf("render template: %w", err)
	}

	return buf.String(), nil
}

func joinURL(baseURL, suffix string) string {
	if baseURL == "" {
		return suffix
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return suffix
	}

	parsed.Path = path.Join(parsed.Path, suffix)
	return parsed.String()
}
