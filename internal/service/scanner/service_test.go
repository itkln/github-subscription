package scanner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	subscriptionmodel "github.com/itkln/github-subscription/internal/model/subscription"
)

func TestServiceRunOnce(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		repository *spyRepository
		notifier   *spyReleaseSender
		github     *spyGitHubClient
		assertion  func(t *testing.T, repository *spyRepository, notifier *spyReleaseSender, github *spyGitHubClient)
	}{
		{
			name: "initializes last seen tag without notifying when empty",
			repository: &spyRepository{
				listConfirmedResult: []subscriptionmodel.DBSubscription{
					{ID: 1, Email: "user@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "", UnsubscribeToken: "unsubscribe-token"},
				},
			},
			notifier: &spyReleaseSender{},
			github: &spyGitHubClient{
				tagByRepo: map[string]string{"golang/go": "v1.24.0"},
			},
			assertion: func(t *testing.T, repository *spyRepository, notifier *spyReleaseSender, github *spyGitHubClient) {
				t.Helper()
				if github.calls != 1 {
					t.Fatalf("github calls = %d, want 1", github.calls)
				}
				if repository.updatedID != 1 || repository.updatedTag != "v1.24.0" {
					t.Fatalf("updated last seen = (%d, %q), want (%d, %q)", repository.updatedID, repository.updatedTag, 1, "v1.24.0")
				}
				if notifier.calls != 0 {
					t.Fatalf("notifier calls = %d, want 0", notifier.calls)
				}
			},
		},
		{
			name: "does not notify when tag is unchanged",
			repository: &spyRepository{
				listConfirmedResult: []subscriptionmodel.DBSubscription{
					{ID: 1, Email: "user@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.24.0", UnsubscribeToken: "unsubscribe-token"},
				},
			},
			notifier: &spyReleaseSender{},
			github: &spyGitHubClient{
				tagByRepo: map[string]string{"golang/go": "v1.24.0"},
			},
			assertion: func(t *testing.T, repository *spyRepository, notifier *spyReleaseSender, github *spyGitHubClient) {
				t.Helper()
				if repository.updateCalls != 0 {
					t.Fatalf("update calls = %d, want 0", repository.updateCalls)
				}
				if notifier.calls != 0 {
					t.Fatalf("notifier calls = %d, want 0", notifier.calls)
				}
			},
		},
		{
			name: "sends notification and updates tag when new release appears",
			repository: &spyRepository{
				listConfirmedResult: []subscriptionmodel.DBSubscription{
					{ID: 1, Email: "user@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.23.0", UnsubscribeToken: "unsubscribe-token"},
				},
			},
			notifier: &spyReleaseSender{},
			github: &spyGitHubClient{
				tagByRepo: map[string]string{"golang/go": "v1.24.0"},
			},
			assertion: func(t *testing.T, repository *spyRepository, notifier *spyReleaseSender, github *spyGitHubClient) {
				t.Helper()
				if notifier.calls != 1 {
					t.Fatalf("notifier calls = %d, want 1", notifier.calls)
				}
				if notifier.email != "user@example.com" || notifier.repo != "golang/go" || notifier.tag != "v1.24.0" || notifier.token != "unsubscribe-token" {
					t.Fatalf("notifier payload = (%q, %q, %q, %q)", notifier.email, notifier.repo, notifier.tag, notifier.token)
				}
				if repository.updatedID != 1 || repository.updatedTag != "v1.24.0" {
					t.Fatalf("updated last seen = (%d, %q), want (%d, %q)", repository.updatedID, repository.updatedTag, 1, "v1.24.0")
				}
			},
		},
		{
			name: "skips update when github has no release",
			repository: &spyRepository{
				listConfirmedResult: []subscriptionmodel.DBSubscription{
					{ID: 1, Email: "user@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.23.0", UnsubscribeToken: "unsubscribe-token"},
				},
			},
			notifier: &spyReleaseSender{},
			github: &spyGitHubClient{
				tagByRepo: map[string]string{"golang/go": ""},
			},
			assertion: func(t *testing.T, repository *spyRepository, notifier *spyReleaseSender, github *spyGitHubClient) {
				t.Helper()
				if repository.updateCalls != 0 {
					t.Fatalf("update calls = %d, want 0", repository.updateCalls)
				}
				if notifier.calls != 0 {
					t.Fatalf("notifier calls = %d, want 0", notifier.calls)
				}
			},
		},
		{
			name: "stops processing when notifier fails",
			repository: &spyRepository{
				listConfirmedResult: []subscriptionmodel.DBSubscription{
					{ID: 1, Email: "user@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.23.0", UnsubscribeToken: "unsubscribe-token"},
				},
			},
			notifier: &spyReleaseSender{err: errors.New("notify failed")},
			github: &spyGitHubClient{
				tagByRepo: map[string]string{"golang/go": "v1.24.0"},
			},
			assertion: func(t *testing.T, repository *spyRepository, notifier *spyReleaseSender, github *spyGitHubClient) {
				t.Helper()
				if repository.updateCalls != 0 {
					t.Fatalf("update calls = %d, want 0", repository.updateCalls)
				}
			},
		},
		{
			name: "stops processing when github fails",
			repository: &spyRepository{
				listConfirmedResult: []subscriptionmodel.DBSubscription{
					{ID: 1, Email: "user@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.23.0", UnsubscribeToken: "unsubscribe-token"},
				},
			},
			notifier: &spyReleaseSender{},
			github: &spyGitHubClient{
				errByRepo: map[string]error{"golang/go": errors.New("github failed")},
			},
			assertion: func(t *testing.T, repository *spyRepository, notifier *spyReleaseSender, github *spyGitHubClient) {
				t.Helper()
				if notifier.calls != 0 {
					t.Fatalf("notifier calls = %d, want 0", notifier.calls)
				}
				if repository.updateCalls != 0 {
					t.Fatalf("update calls = %d, want 0", repository.updateCalls)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(tc.repository, tc.notifier, tc.github, newTestLogger(), time.Second)
			service.runOnce(context.Background())

			tc.assertion(t, tc.repository, tc.notifier, tc.github)
		})
	}
}

func TestServiceRunOnceDeduplicatesRepositories(t *testing.T) {
	t.Parallel()

	repository := &spyRepository{
		listConfirmedResult: []subscriptionmodel.DBSubscription{
			{ID: 1, Email: "first@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.23.0", UnsubscribeToken: "token-1"},
			{ID: 2, Email: "second@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.23.0", UnsubscribeToken: "token-2"},
		},
	}
	notifier := &spyReleaseSender{}
	github := &spyGitHubClient{
		tagByRepo: map[string]string{"golang/go": "v1.24.0"},
	}

	service := NewService(repository, notifier, github, newTestLogger(), time.Second)
	service.runOnce(context.Background())

	if github.calls != 1 {
		t.Fatalf("github calls = %d, want 1", github.calls)
	}
	if notifier.calls != 2 {
		t.Fatalf("notifier calls = %d, want 2", notifier.calls)
	}
	if repository.updateCalls != 2 {
		t.Fatalf("update calls = %d, want 2", repository.updateCalls)
	}
}

func TestServiceRunOnceStopsEarlyOnRateLimit(t *testing.T) {
	t.Parallel()

	repository := &spyRepository{
		listConfirmedResult: []subscriptionmodel.DBSubscription{
			{ID: 1, Email: "first@example.com", Repo: "golang/go", Confirmed: true, LastSeenTag: "v1.23.0", UnsubscribeToken: "token-1"},
			{ID: 2, Email: "second@example.com", Repo: "kubernetes/kubernetes", Confirmed: true, LastSeenTag: "v1.31.0", UnsubscribeToken: "token-2"},
		},
	}
	notifier := &spyReleaseSender{}
	github := &spyGitHubClient{
		errByRepo: map[string]error{"golang/go": &stubRateLimitError{retryAfter: time.Minute}},
		tagByRepo: map[string]string{"kubernetes/kubernetes": "v1.32.0"},
	}

	service := NewService(repository, notifier, github, newTestLogger(), time.Second)
	service.runOnce(context.Background())

	if github.calls != 1 {
		t.Fatalf("github calls = %d, want 1", github.calls)
	}
	if notifier.calls != 0 {
		t.Fatalf("notifier calls = %d, want 0", notifier.calls)
	}
	if repository.updateCalls != 0 {
		t.Fatalf("update calls = %d, want 0", repository.updateCalls)
	}
}

type spyRepository struct {
	listConfirmedResult []subscriptionmodel.DBSubscription
	listConfirmedErr    error
	updateErr           error
	updateCalls         int
	updatedID           int64
	updatedTag          string
}

func (s *spyRepository) ListConfirmed(_ context.Context) ([]subscriptionmodel.DBSubscription, error) {
	if s.listConfirmedErr != nil {
		return nil, s.listConfirmedErr
	}
	return s.listConfirmedResult, nil
}

func (s *spyRepository) UpdateLastSeenTag(_ context.Context, id int64, tag string) error {
	s.updateCalls++
	s.updatedID = id
	s.updatedTag = tag
	return s.updateErr
}

type spyReleaseSender struct {
	calls int
	email string
	repo  string
	tag   string
	token string
	err   error
}

func (s *spyReleaseSender) SendReleaseNotification(_ context.Context, email, repo, tag, token string) error {
	s.calls++
	s.email = email
	s.repo = repo
	s.tag = tag
	s.token = token
	return s.err
}

type spyGitHubClient struct {
	calls     int
	tagByRepo map[string]string
	errByRepo map[string]error
}

func (s *spyGitHubClient) LatestReleaseTag(_ context.Context, repo string) (string, error) {
	s.calls++
	if err := s.errByRepo[repo]; err != nil {
		return "", err
	}
	return s.tagByRepo[repo], nil
}

type stubRateLimitError struct {
	retryAfter time.Duration
}

func (e *stubRateLimitError) Error() string {
	return "rate limited"
}

func (e *stubRateLimitError) IsRateLimit() bool {
	return true
}

func (e *stubRateLimitError) RetryAfterDuration() time.Duration {
	return e.retryAfter
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
