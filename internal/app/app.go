package app

import (
	"net/http"
	"time"

	"github.com/itkln/github-subscription/internal/config"
	"github.com/itkln/github-subscription/internal/transport/httpapi"
)

func Start() error {
	cfg := config.Load()

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server.ListenAndServe()
}
