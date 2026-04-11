package subscription

import (
	"context"
	"database/sql"
	"errors"
	"github.com/itkln/github-subscription/internal/service/notifier"
	"io"
	"log/slog"
	"strings"
	"testing"

	subscriptionmodel "github.com/itkln/github-subscription/internal/model/subscription"
)

func TestServiceSubscribe(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		email         string
		repo          string
		repository    *spyRepository
		notifier      *spyConfirmationSender
		wantErr       error
		assertionFunc func(t *testing.T, repo *spyRepository, notifier *spyConfirmationSender)
	}{
		{
			name:       "returns invalid email error",
			email:      "bad-email",
			repo:       "golang/go",
			repository: &spyRepository{},
			notifier:   &spyConfirmationSender{},
			wantErr:    ErrInvalidEmail,
		},
		{
			name:       "returns invalid repo error",
			email:      "user@example.com",
			repo:       "bad repo",
			repository: &spyRepository{},
			notifier:   &spyConfirmationSender{},
			wantErr:    ErrInvalidRepo,
		},
		{
			name:  "returns already subscribed error",
			email: "user@example.com",
			repo:  "golang/go",
			repository: &spyRepository{
				exists: true,
			},
			notifier: &spyConfirmationSender{},
			wantErr:  ErrAlreadySubscribed,
		},
		{
			name:  "propagates repository exists error",
			email: "user@example.com",
			repo:  "golang/go",
			repository: &spyRepository{
				existsErr: errors.New("exists failed"),
			},
			notifier: &spyConfirmationSender{},
			wantErr:  errors.New("exists failed"),
		},
		{
			name:  "propagates create error",
			email: "user@example.com",
			repo:  "golang/go",
			repository: &spyRepository{
				createErr: errors.New("create failed"),
			},
			notifier: &spyConfirmationSender{},
			wantErr:  errors.New("create failed"),
		},
		{
			name:  "propagates notifier error",
			email: "user@example.com",
			repo:  "golang/go",
			repository: &spyRepository{
				createResult: subscriptionmodel.DBSubscription{
					Email:        "user@example.com",
					Repo:         "golang/go",
					ConfirmToken: "confirm-token",
				},
			},
			notifier: &spyConfirmationSender{
				err: errors.New("notify failed"),
			},
			wantErr: errors.New("notify failed"),
		},
		{
			name:  "creates subscription and sends confirmation",
			email: "user@example.com",
			repo:  "golang/go",
			repository: &spyRepository{
				createFn: func(params subscriptionmodel.CreateParams) subscriptionmodel.DBSubscription {
					return subscriptionmodel.DBSubscription{
						Email:            params.Email,
						Repo:             params.Repo,
						ConfirmToken:     params.ConfirmToken,
						UnsubscribeToken: params.UnsubscribeToken,
					}
				},
			},
			notifier: &spyConfirmationSender{},
			assertionFunc: func(t *testing.T, repo *spyRepository, notifier *spyConfirmationSender) {
				t.Helper()

				if repo.existsCalls != 1 {
					t.Fatalf("existsCalls = %d, want 1", repo.existsCalls)
				}
				if repo.createCalls != 1 {
					t.Fatalf("createCalls = %d, want 1", repo.createCalls)
				}
				if repo.createdParams.Email != "user@example.com" {
					t.Fatalf("created email = %q", repo.createdParams.Email)
				}
				if repo.createdParams.Repo != "golang/go" {
					t.Fatalf("created repo = %q", repo.createdParams.Repo)
				}
				if len(repo.createdParams.ConfirmToken) != 32 {
					t.Fatalf("confirm token length = %d, want 32", len(repo.createdParams.ConfirmToken))
				}
				if len(repo.createdParams.UnsubscribeToken) != 32 {
					t.Fatalf("unsubscribe token length = %d, want 32", len(repo.createdParams.UnsubscribeToken))
				}
				if repo.createdParams.ConfirmToken == repo.createdParams.UnsubscribeToken {
					t.Fatal("confirm and unsubscribe tokens should differ")
				}

				if notifier.calls != 1 {
					t.Fatalf("notifier calls = %d, want 1", notifier.calls)
				}
				if notifier.email != "user@example.com" {
					t.Fatalf("notifier email = %q", notifier.email)
				}
				if notifier.repo != "golang/go" {
					t.Fatalf("notifier repo = %q", notifier.repo)
				}
				if notifier.token != repo.createdParams.ConfirmToken {
					t.Fatalf("notifier token = %q, want %q", notifier.token, repo.createdParams.ConfirmToken)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(tc.repository, tc.notifier)
			err := service.Subscribe(context.Background(), tc.email, tc.repo)

			if tc.wantErr == nil && err != nil {
				t.Fatalf("Subscribe() unexpected error = %v", err)
			}
			if tc.wantErr != nil && !errorMatches(err, tc.wantErr) {
				t.Fatalf("Subscribe() error = %v, want %v", err, tc.wantErr)
			}

			if tc.assertionFunc != nil {
				tc.assertionFunc(t, tc.repository, tc.notifier)
			}
		})
	}
}

func TestServiceConfirm(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		token      string
		repository *spyRepository
		wantErr    error
		assertFunc func(t *testing.T, repo *spyRepository)
	}{
		{
			name:       "returns invalid token error",
			token:      "",
			repository: &spyRepository{},
			wantErr:    ErrInvalidToken,
		},
		{
			name:  "returns not found when token is missing",
			token: "missing",
			repository: &spyRepository{
				getByConfirmTokenErr: sql.ErrNoRows,
			},
			wantErr: ErrNotFound,
		},
		{
			name:  "propagates load error",
			token: "broken",
			repository: &spyRepository{
				getByConfirmTokenErr: errors.New("load failed"),
			},
			wantErr: errors.New("load failed"),
		},
		{
			name:  "returns not found on confirm update",
			token: "missing",
			repository: &spyRepository{
				getByConfirmTokenResult: subscriptionmodel.DBSubscription{ConfirmToken: "missing"},
				confirmErr:              sql.ErrNoRows,
			},
			wantErr: ErrNotFound,
		},
		{
			name:  "confirms subscription",
			token: "confirm-token",
			repository: &spyRepository{
				getByConfirmTokenResult: subscriptionmodel.DBSubscription{ConfirmToken: "confirm-token"},
			},
			assertFunc: func(t *testing.T, repo *spyRepository) {
				t.Helper()
				if repo.confirmToken != "confirm-token" {
					t.Fatalf("confirm token = %q, want %q", repo.confirmToken, "confirm-token")
				}
				if repo.confirmCalls != 1 {
					t.Fatalf("confirm calls = %d, want 1", repo.confirmCalls)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(tc.repository, &spyConfirmationSender{}).Confirm(context.Background(), tc.token)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("Confirm() unexpected error = %v", err)
			}
			if tc.wantErr != nil && !errorMatches(err, tc.wantErr) {
				t.Fatalf("Confirm() error = %v, want %v", err, tc.wantErr)
			}
			if tc.assertFunc != nil {
				tc.assertFunc(t, tc.repository)
			}
		})
	}
}

func TestServiceUnsubscribe(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		token      string
		repository *spyRepository
		wantErr    error
		assertFunc func(t *testing.T, repo *spyRepository)
	}{
		{
			name:       "returns invalid token error",
			token:      "",
			repository: &spyRepository{},
			wantErr:    ErrInvalidToken,
		},
		{
			name:  "returns not found when token is missing",
			token: "missing",
			repository: &spyRepository{
				getByUnsubscribeTokenErr: sql.ErrNoRows,
			},
			wantErr: ErrNotFound,
		},
		{
			name:  "propagates load error",
			token: "broken",
			repository: &spyRepository{
				getByUnsubscribeTokenErr: errors.New("load failed"),
			},
			wantErr: errors.New("load failed"),
		},
		{
			name:  "returns not found on delete",
			token: "missing",
			repository: &spyRepository{
				getByUnsubscribeTokenResult: subscriptionmodel.DBSubscription{UnsubscribeToken: "missing"},
				deleteErr:                   sql.ErrNoRows,
			},
			wantErr: ErrNotFound,
		},
		{
			name:  "deletes subscription",
			token: "unsubscribe-token",
			repository: &spyRepository{
				getByUnsubscribeTokenResult: subscriptionmodel.DBSubscription{UnsubscribeToken: "unsubscribe-token"},
			},
			assertFunc: func(t *testing.T, repo *spyRepository) {
				t.Helper()
				if repo.deleteToken != "unsubscribe-token" {
					t.Fatalf("delete token = %q, want %q", repo.deleteToken, "unsubscribe-token")
				}
				if repo.deleteCalls != 1 {
					t.Fatalf("delete calls = %d, want 1", repo.deleteCalls)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(tc.repository, &spyConfirmationSender{}).Unsubscribe(context.Background(), tc.token)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("Unsubscribe() unexpected error = %v", err)
			}
			if tc.wantErr != nil && !errorMatches(err, tc.wantErr) {
				t.Fatalf("Unsubscribe() error = %v, want %v", err, tc.wantErr)
			}
			if tc.assertFunc != nil {
				tc.assertFunc(t, tc.repository)
			}
		})
	}
}

func TestServiceListSubscriptions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		email      string
		repository *spyRepository
		wantErr    error
		wantCount  int
	}{
		{
			name:       "returns invalid email error",
			email:      "bad-email",
			repository: &spyRepository{},
			wantErr:    ErrInvalidEmail,
		},
		{
			name:  "propagates repository error",
			email: "user@example.com",
			repository: &spyRepository{
				listErr: errors.New("list failed"),
			},
			wantErr: errors.New("list failed"),
		},
		{
			name:  "returns subscriptions",
			email: "user@example.com",
			repository: &spyRepository{
				listResult: []subscriptionmodel.DBSubscription{
					{Email: "user@example.com", Repo: "golang/go", Confirmed: true},
					{Email: "user@example.com", Repo: "kubernetes/kubernetes", Confirmed: true},
				},
			},
			wantCount: 2,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(tc.repository, &spyConfirmationSender{}).ListSubscriptions(context.Background(), tc.email)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("ListSubscriptions() unexpected error = %v", err)
			}
			if tc.wantErr != nil && !errorMatches(err, tc.wantErr) {
				t.Fatalf("ListSubscriptions() error = %v, want %v", err, tc.wantErr)
			}
			if len(got) != tc.wantCount {
				t.Fatalf("ListSubscriptions() len = %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}

type spyRepository struct {
	createdParams               subscriptionmodel.CreateParams
	createResult                subscriptionmodel.DBSubscription
	listResult                  []subscriptionmodel.DBSubscription
	exists                      bool
	existsErr                   error
	createErr                   error
	getByConfirmTokenResult     subscriptionmodel.DBSubscription
	getByConfirmTokenErr        error
	getByUnsubscribeTokenResult subscriptionmodel.DBSubscription
	getByUnsubscribeTokenErr    error
	confirmErr                  error
	deleteErr                   error
	listErr                     error
	existsCalls                 int
	createCalls                 int
	confirmCalls                int
	deleteCalls                 int
	confirmToken                string
	deleteToken                 string
	createFn                    func(subscriptionmodel.CreateParams) subscriptionmodel.DBSubscription
}

func (s *spyRepository) Create(_ context.Context, params subscriptionmodel.CreateParams) (subscriptionmodel.DBSubscription, error) {
	s.createCalls++
	s.createdParams = params
	if s.createErr != nil {
		return subscriptionmodel.DBSubscription{}, s.createErr
	}
	if s.createFn != nil {
		return s.createFn(params), nil
	}
	if s.createResult != (subscriptionmodel.DBSubscription{}) {
		return s.createResult, nil
	}

	return subscriptionmodel.DBSubscription{
		Email:            params.Email,
		Repo:             params.Repo,
		ConfirmToken:     params.ConfirmToken,
		UnsubscribeToken: params.UnsubscribeToken,
	}, nil
}

func (s *spyRepository) ExistsByEmailAndRepo(_ context.Context, _, _ string) (bool, error) {
	s.existsCalls++
	return s.exists, s.existsErr
}

func (s *spyRepository) GetByConfirmToken(_ context.Context, _ string) (subscriptionmodel.DBSubscription, error) {
	if s.getByConfirmTokenErr != nil {
		return subscriptionmodel.DBSubscription{}, s.getByConfirmTokenErr
	}
	return s.getByConfirmTokenResult, nil
}

func (s *spyRepository) GetByUnsubscribeToken(_ context.Context, _ string) (subscriptionmodel.DBSubscription, error) {
	if s.getByUnsubscribeTokenErr != nil {
		return subscriptionmodel.DBSubscription{}, s.getByUnsubscribeTokenErr
	}
	return s.getByUnsubscribeTokenResult, nil
}

func (s *spyRepository) ConfirmByToken(_ context.Context, token string) error {
	s.confirmCalls++
	s.confirmToken = token
	return s.confirmErr
}

func (s *spyRepository) DeleteByUnsubscribeToken(_ context.Context, token string) error {
	s.deleteCalls++
	s.deleteToken = token
	return s.deleteErr
}

func (s *spyRepository) ListActiveByEmail(_ context.Context, _ string) ([]subscriptionmodel.DBSubscription, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listResult, nil
}

type spyConfirmationSender struct {
	email string
	repo  string
	token string
	calls int
	err   error
}

func (s *spyConfirmationSender) SendConfirmation(_ context.Context, email, repo, token string) error {
	s.calls++
	s.email = email
	s.repo = repo
	s.token = token
	return s.err
}

func newTestService(repository Repository, notifier notifier.ConfirmationSender) *Service {
	return NewService(repository, notifier, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

func errorMatches(got, want error) bool {
	if want == nil {
		return got == nil
	}
	if errors.Is(got, want) {
		return true
	}
	return got != nil && got.Error() == want.Error()
}

func TestNewToken(t *testing.T) {
	t.Parallel()

	token := newToken()
	if len(token) != 32 {
		t.Fatalf("newToken() length = %d, want 32", len(token))
	}

	if strings.Trim(token, "0123456789abcdef") != "" {
		t.Fatalf("newToken() = %q, want lowercase hex", token)
	}
}
