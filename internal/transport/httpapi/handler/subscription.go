package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	subscriptionmodel "github.com/itkln/github-subscription/internal/model/subscription"
	subscriptionservice "github.com/itkln/github-subscription/internal/service/subscription"
	"github.com/itkln/github-subscription/internal/transport/httpapi/dto"
)

type SubscriptionHandler struct {
	service Service
	logger  *slog.Logger
}

type Service interface {
	Subscribe(ctx context.Context, email, repo string) error
	Confirm(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
	ListSubscriptions(ctx context.Context, email string) ([]subscriptionmodel.DBSubscription, error)
}

func NewSubscriptionHandler(service Service, logger *slog.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{service: service, logger: logger}
}

func (h *SubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var request dto.SubscribeRequest
	h.logger.Debug("handling subscribe request")
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Error("decode subscribe request failed", "error", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err := h.service.Subscribe(r.Context(), request.Email, request.Repo)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, subscriptionservice.ErrInvalidEmail), errors.Is(err, subscriptionservice.ErrInvalidRepo):
		h.logger.Error("subscribe request validation failed", "email", request.Email, "repo", request.Repo, "error", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	case errors.Is(err, subscriptionservice.ErrRepoNotFound):
		h.logger.Error("subscribe repository not found", "email", request.Email, "repo", request.Repo, "error", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	case errors.Is(err, subscriptionservice.ErrAlreadySubscribed):
		h.logger.Error("subscribe request conflicted", "email", request.Email, "repo", request.Repo, "error", err)
		http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
	default:
		h.logger.Error("subscribe request failed", "email", request.Email, "repo", request.Repo, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *SubscriptionHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	h.logger.Debug("handling confirm request")
	err := h.service.Confirm(r.Context(), token)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, subscriptionservice.ErrInvalidToken):
		h.logger.Error("confirm request validation failed", "token", token, "error", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	case errors.Is(err, subscriptionservice.ErrNotFound):
		h.logger.Error("confirm request token not found", "token", token, "error", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	default:
		h.logger.Error("confirm request failed", "token", token, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *SubscriptionHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	h.logger.Debug("handling unsubscribe request")
	err := h.service.Unsubscribe(r.Context(), token)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, subscriptionservice.ErrInvalidToken):
		h.logger.Error("unsubscribe request validation failed", "token", token, "error", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	case errors.Is(err, subscriptionservice.ErrNotFound):
		h.logger.Error("unsubscribe request token not found", "token", token, "error", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	default:
		h.logger.Error("unsubscribe request failed", "token", token, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	h.logger.Debug("handling list subscriptions request", "email", email)
	subscriptions, err := h.service.ListSubscriptions(r.Context(), email)
	switch {
	case err == nil:
		response := make([]dto.SubscriptionResponse, 0, len(subscriptions))
		for _, subscription := range subscriptions {
			response = append(response, dto.SubscriptionResponse{
				Email:       subscription.Email,
				Repo:        subscription.Repo,
				Confirmed:   subscription.Confirmed,
				LastSeenTag: subscription.LastSeenTag,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("encode subscriptions response failed", "email", email, "error", err)
		}
	case errors.Is(err, subscriptionservice.ErrInvalidEmail):
		h.logger.Error("list subscriptions validation failed", "email", email, "error", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	default:
		h.logger.Error("list subscriptions failed", "email", email, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
