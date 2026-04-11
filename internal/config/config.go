package config

import (
	"os"
)

const (
	defaultHTTPAddress   = ":8080"
	defaultDatabaseDSN   = "postgres://postgres:postgres@localhost:5432/github_subscription?sslmode=disable"
	defaultPublicBaseURL = "http://localhost:8080"
	defaultSMTPHost      = "localhost"
	defaultSMTPPort      = "1025"
	defaultSMTPFrom      = "noreply@github-subscription.local"
)

type Config struct {
	HTTPAddress   string
	Database      DatabaseConfig
	PublicBaseURL string
	SMTP          SMTPConfig
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func Load() Config {
	return Config{
		HTTPAddress: getEnv("HTTP_ADDRESS", defaultHTTPAddress),
		Database: DatabaseConfig{
			Driver: "postgres",
			DSN:    getEnv("DATABASE_DSN", defaultDatabaseDSN),
		},
		PublicBaseURL: getEnv("PUBLIC_BASE_URL", defaultPublicBaseURL),
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", defaultSMTPHost),
			Port:     getEnv("SMTP_PORT", defaultSMTPPort),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     getEnv("SMTP_FROM", defaultSMTPFrom),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
