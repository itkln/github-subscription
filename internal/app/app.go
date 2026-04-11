package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/itkln/github-subscription/internal/config"
	dbbootstrap "github.com/itkln/github-subscription/internal/platform/database"
	"github.com/itkln/github-subscription/internal/platform/email"
	"github.com/itkln/github-subscription/internal/platform/github"
	subscriptionrepository "github.com/itkln/github-subscription/internal/repository/subscription"
	notifierservice "github.com/itkln/github-subscription/internal/service/notifier"
	scannerservice "github.com/itkln/github-subscription/internal/service/scanner"
	subscriptionservice "github.com/itkln/github-subscription/internal/service/subscription"
	"github.com/itkln/github-subscription/internal/transport/httpapi"
)

func Start(logger *slog.Logger) error {
	cfg := config.Load()
	logger.Info("application starting", "http_address", cfg.HTTPAddress, "database_driver", cfg.Database.Driver)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := dbbootstrap.Migrate(ctx, cfg.Database, logger); err != nil {
		logger.Error("database initialization failed", "stage", "migrate", "error", err)
		return err
	}

	db, err := dbbootstrap.Open(ctx, cfg.Database, logger)
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

	githubClient := github.NewClient(cfg.Scanner.GitHubAPI, cfg.Scanner.Token)
	scannerService := scannerservice.NewService(subscriptionRepository, notificationService, githubClient, logger, scanInterval)
	subscriptionService := subscriptionservice.NewService(subscriptionRepository, notificationService, logger)

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.NewRouter(subscriptionService, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go scannerService.Start(context.Background())

	logger.Info("http server listening", "address", cfg.HTTPAddress)
	return server.ListenAndServe()
}
