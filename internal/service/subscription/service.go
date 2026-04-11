package subscription

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/mail"
	"regexp"

	subscriptionmodel "github.com/itkln/github-subscription/internal/model/subscription"
	"github.com/itkln/github-subscription/internal/service/notifier"
)

var (
	ErrInvalidEmail      = errors.New("invalid email")
	ErrInvalidRepo       = errors.New("invalid repo")
	ErrInvalidToken      = errors.New("invalid token")
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrNotFound          = errors.New("not found")
)

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type Repository interface {
	Create(ctx context.Context, params subscriptionmodel.CreateParams) (subscriptionmodel.DBSubscription, error)
	ExistsByEmailAndRepo(ctx context.Context, email, repo string) (bool, error)
	GetByConfirmToken(ctx context.Context, token string) (subscriptionmodel.DBSubscription, error)
	GetByUnsubscribeToken(ctx context.Context, token string) (subscriptionmodel.DBSubscription, error)
	ConfirmByToken(ctx context.Context, token string) error
	DeleteByUnsubscribeToken(ctx context.Context, token string) error
	ListActiveByEmail(ctx context.Context, email string) ([]subscriptionmodel.DBSubscription, error)
}

type Service struct {
	repository Repository
	notifier   notifier.ConfirmationSender
	logger     *slog.Logger
}

func NewService(
	repository Repository,
	notifier notifier.ConfirmationSender,
	logger *slog.Logger,
) *Service {
	return &Service{
		repository: repository,
		notifier:   notifier,
		logger:     logger,
	}
}

func (s *Service) Subscribe(ctx context.Context, email, repo string) error {
	s.logger.Debug("starting subscription flow", "email", email, "repo", repo)
	if !isValidEmail(email) {
		return ErrInvalidEmail
	}
	if !repoPattern.MatchString(repo) {
		return ErrInvalidRepo
	}

	exists, err := s.repository.ExistsByEmailAndRepo(ctx, email, repo)
	if err != nil {
		s.logger.Error("check subscription existence failed", "email", email, "repo", repo, "error", err)
		return err
	}
	if exists {
		return ErrAlreadySubscribed
	}

	s.logger.Debug("creating subscription record", "email", email, "repo", repo)
	created, err := s.repository.Create(ctx, subscriptionmodel.CreateParams{
		Email:            email,
		Repo:             repo,
		ConfirmToken:     newToken(),
		UnsubscribeToken: newToken(),
	})
	if err != nil {
		s.logger.Error("create subscription failed", "email", email, "repo", repo, "error", err)
		return err
	}

	if err := s.notifier.SendConfirmation(ctx, created.Email, created.Repo, created.ConfirmToken); err != nil {
		s.logger.Error("send confirmation notification failed", "email", created.Email, "repo", created.Repo, "error", err)
		return err
	}

	s.logger.Info("subscription created", "email", created.Email, "repo", created.Repo, "confirmed", created.Confirmed)
	return nil
}

func (s *Service) Confirm(ctx context.Context, token string) error {
	s.logger.Debug("starting confirmation flow")
	if token == "" {
		return ErrInvalidToken
	}

	if _, err := s.repository.GetByConfirmToken(ctx, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		s.logger.Error("load subscription by confirmation token failed", "token", token, "error", err)
		return err
	}

	if err := s.repository.ConfirmByToken(ctx, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		s.logger.Error("confirm subscription failed", "token", token, "error", err)
		return err
	}

	s.logger.Info("subscription confirmed", "token", token)
	return nil
}

func (s *Service) Unsubscribe(ctx context.Context, token string) error {
	s.logger.Debug("starting unsubscribe flow")
	if token == "" {
		return ErrInvalidToken
	}

	if _, err := s.repository.GetByUnsubscribeToken(ctx, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		s.logger.Error("load subscription by unsubscribe token failed", "token", token, "error", err)
		return err
	}

	if err := s.repository.DeleteByUnsubscribeToken(ctx, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		s.logger.Error("delete subscription failed", "token", token, "error", err)
		return err
	}

	s.logger.Info("subscription removed", "token", token)
	return nil
}

func (s *Service) ListSubscriptions(ctx context.Context, email string) ([]subscriptionmodel.DBSubscription, error) {
	s.logger.Debug("starting list subscriptions flow", "email", email)
	if !isValidEmail(email) {
		return nil, ErrInvalidEmail
	}

	subscriptions, err := s.repository.ListActiveByEmail(ctx, email)
	if err != nil {
		s.logger.Error("list subscriptions failed", "email", email, "error", err)
		return nil, err
	}

	s.logger.Info("subscriptions fetched", "email", email, "count", len(subscriptions))
	return subscriptions, nil
}

func isValidEmail(value string) bool {
	_, err := mail.ParseAddress(value)
	return err == nil
}

func newToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "token-generation-fallback"
	}

	return hex.EncodeToString(buf)
}
