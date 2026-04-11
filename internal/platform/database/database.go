package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/itkln/github-subscription/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, cfg config.DatabaseConfig, logger *slog.Logger) (*sql.DB, error) {
	if cfg.Driver == "" {
		return nil, errors.New("database driver is required")
	}
	if cfg.DSN == "" {
		return nil, errors.New("database dsn is required")
	}

	if cfg.Driver != "postgres" {
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	logger.Info("opening database connection", "driver", cfg.Driver)
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	logger.Info("database connection established", "driver", cfg.Driver)
	return db, nil
}

func Migrate(ctx context.Context, cfg config.DatabaseConfig, logger *slog.Logger) error {
	if cfg.Driver != "postgres" {
		return fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
	if cfg.DSN == "" {
		return errors.New("database dsn is required")
	}

	db, err := Open(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("migration database close failed", "error", err)
			return
		}
		logger.Info("migration database connection closed")
	}()

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate database instance: %w", err)
	}

	logger.Info("running database migrations")
	migrator, err := migrate.NewWithDatabaseInstance("file://migration", "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if err := migrator.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("database migrations are up to date")
			return nil
		}

		return fmt.Errorf("run migrations: %w", err)
	}

	logger.Info("database migrations applied successfully")
	return nil
}
