package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/itkln/github-subscription/internal/config"
	"github.com/itkln/github-subscription/internal/platform/cache"
	dbbootstrap "github.com/itkln/github-subscription/internal/platform/database"
	"github.com/itkln/github-subscription/internal/platform/email"
	"github.com/itkln/github-subscription/internal/platform/github"
	"github.com/itkln/github-subscription/internal/platform/metrics"
	subscriptionrepository "github.com/itkln/github-subscription/internal/repository/subscription"
	notifierservice "github.com/itkln/github-subscription/internal/service/notifier"
	scannerservice "github.com/itkln/github-subscription/internal/service/scanner"
	subscriptionservice "github.com/itkln/github-subscription/internal/service/subscription"
	"github.com/itkln/github-subscription/internal/transport/httpapi"
)

func Start(logger *slog.Logger) error {
	cfg := config.Load()
	metrics.SetAppUp()
	logger.Info("application starting", "http_address", cfg.HTTPAddress, "database_driver", cfg.Database.Driver)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	initCtx, cancel := context.WithTimeout(rootCtx, 15*time.Second)
	defer cancel()

	if err := dbbootstrap.Migrate(initCtx, cfg.Database, logger); err != nil {
		logger.Error("database initialization failed", "stage", "migrate", "error", err)
		return err
	}

	db, err := dbbootstrap.Open(initCtx, cfg.Database, logger)
	if err != nil {
		logger.Error("database initialization failed", "stage", "open", "error", err)
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("database close failed", "error", err)
			return
		}
		logger.Info("database connection closed")
	}()

	subscriptionRepository := subscriptionrepository.NewRepository(db)
	redisCache := cache.NewRedis(cfg.Redis)
	if err := redisCache.Ping(initCtx); err != nil {
		logger.Error("redis initialization failed", "error", err)
		return err
	}
	defer func() {
		if err := redisCache.Close(); err != nil {
			logger.Warn("redis close failed", "error", err)
		}
	}()

	smtpSender := email.NewSMTPSender(logger, cfg.SMTP)
	notificationService, err := notifierservice.NewService(logger, smtpSender, cfg.PublicBaseURL)
	if err != nil {
		logger.Error("notification service initialization failed", "error", err)
		return err
	}

	scanInterval, err := time.ParseDuration(cfg.Scanner.Interval)
	if err != nil {
		logger.Error("scanner initialization failed", "field", "SCAN_INTERVAL", "value", cfg.Scanner.Interval, "error", err)
		return err
	}

	githubClient := github.NewClient(cfg.Scanner.GitHubAPI, cfg.Scanner.Token, redisCache, cfg.Redis.TTL)
	scannerService := scannerservice.NewService(subscriptionRepository, notificationService, githubClient, logger, scanInterval)
	subscriptionService := subscriptionservice.NewService(subscriptionRepository, notificationService, githubClient, logger)

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.NewRouter(subscriptionService, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go scannerService.Start(rootCtx)

	logger.Info("http server listening", "address", cfg.HTTPAddress)
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("http server shutdown failed", "error", err)
			return err
		}

		logger.Info("http server shutdown completed")
		return nil
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped with error", "error", err)
			return err
		}

		return nil
	}
}
