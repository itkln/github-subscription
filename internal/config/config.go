package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultHTTPAddress   = ":8080"
	defaultDatabaseDSN   = "postgres://postgres:postgres@localhost:5432/github_subscription?sslmode=disable"
	defaultPublicBaseURL = "http://localhost:8080"
	defaultSMTPHost      = "localhost"
	defaultSMTPPort      = "1025"
	defaultSMTPFrom      = "noreply@github-subscription.local"
	defaultScanInterval  = "1m"
)

type Config struct {
	HTTPAddress   string
	Database      DatabaseConfig
	PublicBaseURL string
	SMTP          SMTPConfig
	Redis         RedisConfig
	LogLevel      string
	Scanner       ScannerConfig
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

type RedisConfig struct {
	Addr     string
	DB       int
	TTL      time.Duration
	Password string
}

type ScannerConfig struct {
	Interval  string
	GitHubAPI string
	Token     string
}

var loadDotenvOnce sync.Once

func Load() Config {
	loadDotenvOnce.Do(func() {
		loadDotenv(".env")
	})

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
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       getEnvInt("REDIS_DB", 0),
			TTL:      getEnvDuration("GITHUB_CACHE_TTL", 10*time.Minute),
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
		Scanner: ScannerConfig{
			Interval:  getEnv("SCAN_INTERVAL", defaultScanInterval),
			GitHubAPI: getEnv("GITHUB_API_BASE_URL", "https://api.github.com"),
			Token:     os.Getenv("GITHUB_TOKEN"),
		},
	}
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func loadDotenv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		value = strings.Trim(strings.TrimSpace(value), `"'`)
		_ = os.Setenv(key, value)
	}
}
