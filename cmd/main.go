package main

import (
	"log/slog"
	"os"

	"github.com/itkln/github-subscription/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := app.Start(logger); err != nil {
		logger.Error("application stopped with error", "error", err)
		os.Exit(1)
	}
}
