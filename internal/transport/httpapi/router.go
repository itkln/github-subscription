package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/itkln/github-subscription/internal/platform/metrics"
	"github.com/itkln/github-subscription/internal/transport/httpapi/handler"
)

func NewRouter(service handler.Service, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	subscriptionHandler := handler.NewSubscriptionHandler(service, logger)

	mux.Handle("GET /metrics", metrics.MetricsHandler())
	mux.HandleFunc("POST /api/subscribe", subscriptionHandler.Subscribe)
	mux.HandleFunc("GET /api/confirm/{token}", subscriptionHandler.Confirm)
	mux.HandleFunc("GET /api/unsubscribe/{token}", subscriptionHandler.Unsubscribe)
	mux.HandleFunc("GET /api/subscriptions", subscriptionHandler.List)

	return metrics.TrackHTTPRequest(mux)
}
