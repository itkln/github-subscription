package config

import (
	"os"
)

const (
	defaultHTTPAddress = ":8080"
	defaultDatabaseDSN = "postgres://postgres:postgres@localhost:5432/github_subscription?sslmode=disable"
)

type Config struct {
	HTTPAddress string
	Database    DatabaseConfig
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

func Load() Config {
	return Config{
		HTTPAddress: getEnv("HTTP_ADDRESS", defaultHTTPAddress),
		Database: DatabaseConfig{
			Driver: "postgres",
			DSN:    getEnv("DATABASE_DSN", defaultDatabaseDSN),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
