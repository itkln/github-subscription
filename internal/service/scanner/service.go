package scanner

import (
	"context"
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

	for _, subscription := range subscriptions {
		s.processSubscription(ctx, subscription)
	}

	s.logger.Debug("scanner cycle completed", "subscriptions", len(subscriptions))
}

func (s *Service) processSubscription(ctx context.Context, subscription subscriptionmodel.DBSubscription) {
	tag, err := s.github.LatestReleaseTag(ctx, subscription.Repo)
	if err != nil {
		s.logger.Error("fetch latest github release failed", "repo", subscription.Repo, "error", err)
		return
	}

	if tag == "" {
		s.logger.Debug("no release found for repository", "repo", subscription.Repo)
		return
	}

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
