package httpapi

import (
	"net/http"

	"github.com/itkln/github-subscription/internal/transport/httpapi/handler"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	subscriptionHandler := handler.NewSubscriptionHandler()

	mux.HandleFunc("POST /api/subscribe", subscriptionHandler.Subscribe)
	mux.HandleFunc("GET /api/confirm/{token}", subscriptionHandler.Confirm)
	mux.HandleFunc("GET /api/unsubscribe/{token}", subscriptionHandler.Unsubscribe)
	mux.HandleFunc("GET /api/subscriptions", subscriptionHandler.List)

	return mux
}
