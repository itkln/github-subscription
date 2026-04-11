package scanner

import (
	"context"
	"errors"
	"log/slog"
	"time"

	subscriptionmodel "github.com/itkln/github-subscription/internal/model/subscription"
	notifier "github.com/itkln/github-subscription/internal/service/notifier"
)

type Repository interface {
	ListConfirmed(ctx context.Context) ([]subscriptionmodel.DBSubscription, error)
	UpdateLastSeenTag(ctx context.Context, id int64, tag string) error
}

type GitHubClient interface {
	LatestReleaseTag(ctx context.Context, repo string) (string, error)
}

type rateLimitError interface {
	IsRateLimit() bool
	RetryAfterDuration() time.Duration
}

type Service struct {
	repository Repository
	notifier   notifier.ReleaseSender
	github     GitHubClient
	logger     *slog.Logger
	interval   time.Duration
}

func NewService(
	repository Repository,
	notifier notifier.ReleaseSender,
	github GitHubClient,
	logger *slog.Logger,
	interval time.Duration,
) *Service {
	return &Service{
		repository: repository,
		notifier:   notifier,
		github:     github,
		logger:     logger,
		interval:   interval,
	}
}

func (s *Service) Start(ctx context.Context) {
	s.logger.Info("scanner started", "interval", s.interval.String())
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scanner stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Service) runOnce(ctx context.Context) {
	s.logger.Debug("scanner cycle started")

	subscriptions, err := s.repository.ListConfirmed(ctx)
	if err != nil {
		s.logger.Error("load confirmed subscriptions failed", "error", err)
		return
	}

	repositories := groupSubscriptionsByRepo(subscriptions)
	if len(repositories) == 0 {
		s.logger.Debug("scanner cycle completed", "subscriptions", 0, "repositories", 0)
		return
	}

	for _, group := range repositories {
		if err := s.processRepository(ctx, group); err != nil {
			if isRateLimitError(err) {
				s.logRateLimit(err, len(repositories))
				break
			}
		}
	}

	s.logger.Debug("scanner cycle completed", "subscriptions", len(subscriptions), "repositories", len(repositories))
}

func (s *Service) processRepository(ctx context.Context, subscriptions []subscriptionmodel.DBSubscription) error {
	if len(subscriptions) == 0 {
		return nil
	}

	repo := subscriptions[0].Repo
	tag, err := s.github.LatestReleaseTag(ctx, repo)
	if err != nil {
		if isRateLimitError(err) {
			return err
		}

		s.logger.Error("fetch latest github release failed", "repo", repo, "error", err)
		return nil
	}

	if tag == "" {
		s.logger.Debug("no release found for repository", "repo", repo)
		return nil
	}

	for _, subscription := range subscriptions {
		s.processSubscriptionWithTag(ctx, subscription, tag)
	}

	return nil
}

func (s *Service) processSubscriptionWithTag(ctx context.Context, subscription subscriptionmodel.DBSubscription, tag string) {
	if subscription.LastSeenTag == "" {
		s.logger.Debug("initializing last seen tag", "repo", subscription.Repo, "tag", tag, "subscription_id", subscription.ID)
		if err := s.repository.UpdateLastSeenTag(ctx, subscription.ID, tag); err != nil {
			s.logger.Error("initialize last seen tag failed", "repo", subscription.Repo, "subscription_id", subscription.ID, "error", err)
		}
		return
	}

	if subscription.LastSeenTag == tag {
		s.logger.Debug("no new release detected", "repo", subscription.Repo, "tag", tag, "subscription_id", subscription.ID)
		return
	}

	if err := s.notifier.SendReleaseNotification(ctx, subscription.Email, subscription.Repo, tag, subscription.UnsubscribeToken); err != nil {
		s.logger.Error("send release notification failed", "email", subscription.Email, "repo", subscription.Repo, "tag", tag, "error", err)
		return
	}

	if err := s.repository.UpdateLastSeenTag(ctx, subscription.ID, tag); err != nil {
		s.logger.Error("update last seen tag failed", "repo", subscription.Repo, "subscription_id", subscription.ID, "tag", tag, "error", err)
		return
	}

	s.logger.Info("new release notification sent", "email", subscription.Email, "repo", subscription.Repo, "tag", tag)
}

func groupSubscriptionsByRepo(subscriptions []subscriptionmodel.DBSubscription) [][]subscriptionmodel.DBSubscription {
	grouped := make(map[string][]subscriptionmodel.DBSubscription, len(subscriptions))
	order := make([]string, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		if _, ok := grouped[subscription.Repo]; !ok {
			order = append(order, subscription.Repo)
		}
		grouped[subscription.Repo] = append(grouped[subscription.Repo], subscription)
	}

	result := make([][]subscriptionmodel.DBSubscription, 0, len(order))
	for _, repo := range order {
		result = append(result, grouped[repo])
	}

	return result
}

func isRateLimitError(err error) bool {
	var target rateLimitError
	return errors.As(err, &target) && target.IsRateLimit()
}

func (s *Service) logRateLimit(err error, repositories int) {
	var target rateLimitError
	if !errors.As(err, &target) {
		return
	}

	s.logger.Warn(
		"github rate limit reached, scanner cycle stopped early",
		"repositories", repositories,
		"retry_after", target.RetryAfterDuration().String(),
		"error", err,
	)
}
