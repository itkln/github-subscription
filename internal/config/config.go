package config

import (
	"os"
)

const defaultHTTPAddress = ":8080"

type Config struct {
	HTTPAddress string
}

func Load() Config {
	return Config{
		HTTPAddress: getEnv("HTTP_ADDRESS", defaultHTTPAddress),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
