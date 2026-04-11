package notifier

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	t.Parallel()

	service := newTestService(t, &spyEmailSender{})
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestServiceSendConfirmation(t *testing.T) {
	t.Parallel()

	sender := &spyEmailSender{}
	service := newTestService(t, sender)

	if err := service.SendConfirmation(context.Background(), "user@example.com", "golang/go", "confirm-token"); err != nil {
		t.Fatalf("SendConfirmation() error = %v", err)
	}

	assertEmailRequest(t, sender, emailExpectation{
		to:      "user@example.com",
		subject: "Confirm your GitHub release subscription",
		bodyContains: []string{
			"<html",
			"golang/go",
			"http://localhost:8080/api/confirm/confirm-token",
			"Confirm subscription",
		},
	})
}

func TestServiceSendReleaseNotification(t *testing.T) {
	t.Parallel()

	sender := &spyEmailSender{}
	service := newTestService(t, sender)

	if err := service.SendReleaseNotification(context.Background(), "user@example.com", "golang/go", "v1.24.0", "unsubscribe-token"); err != nil {
		t.Fatalf("SendReleaseNotification() error = %v", err)
	}

	assertEmailRequest(t, sender, emailExpectation{
		to:      "user@example.com",
		subject: "New release for golang/go: v1.24.0",
		bodyContains: []string{
			"<html",
			"golang/go",
			"v1.24.0",
			"http://localhost:8080/api/unsubscribe/unsubscribe-token",
			"Unsubscribe",
		},
	})
}

func TestServicePropagatesSenderErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		call func(context.Context, *Service) error
	}{
		{
			name: "confirmation",
			call: func(ctx context.Context, service *Service) error {
				return service.SendConfirmation(ctx, "user@example.com", "golang/go", "confirm-token")
			},
		},
		{
			name: "release notification",
			call: func(ctx context.Context, service *Service) error {
				return service.SendReleaseNotification(ctx, "user@example.com", "golang/go", "v1.24.0", "unsubscribe-token")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wantErr := errors.New("send failed")
			service := newTestService(t, &spyEmailSender{err: wantErr})

			err := tc.call(context.Background(), service)
			if !errors.Is(err, wantErr) {
				t.Fatalf("call error = %v, want %v", err, wantErr)
			}
		})
	}
}

func TestJoinURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		baseURL string
		suffix  string
		want    string
	}{
		{
			name:    "joins valid base url",
			baseURL: "http://localhost:8080",
			suffix:  "/api/confirm/token",
			want:    "http://localhost:8080/api/confirm/token",
		},
		{
			name:    "falls back on invalid base url",
			baseURL: "://bad-url",
			suffix:  "/api/confirm/token",
			want:    "/api/confirm/token",
		},
		{
			name:    "falls back on empty base url",
			baseURL: "",
			suffix:  "/api/confirm/token",
			want:    "/api/confirm/token",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := joinURL(tc.baseURL, tc.suffix); got != tc.want {
				t.Fatalf("joinURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

type spyEmailSender struct {
	to      string
	subject string
	body    string
	err     error
}

func (s *spyEmailSender) SendHTML(_ context.Context, to, subject, body string) error {
	s.to = to
	s.subject = subject
	s.body = body

	return s.err
}

type emailExpectation struct {
	to           string
	subject      string
	bodyContains []string
}

func newTestService(t *testing.T, sender EmailSender) *Service {
	t.Helper()

	service, err := NewService(newTestLogger(), sender, "http://localhost:8080")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func assertEmailRequest(t *testing.T, sender *spyEmailSender, want emailExpectation) {
	t.Helper()

	if sender.to != want.to {
		t.Fatalf("email to = %q, want %q", sender.to, want.to)
	}

	if sender.subject != want.subject {
		t.Fatalf("email subject = %q, want %q", sender.subject, want.subject)
	}

	for _, fragment := range want.bodyContains {
		if !strings.Contains(sender.body, fragment) {
			t.Fatalf("email body does not contain %q: %q", fragment, sender.body)
		}
	}
}
