package handler

import (
	"encoding/json"
	"net/http"

	"github.com/itkln/github-subscription/internal/transport/httpapi/dto"
)

type SubscriptionHandler struct{}

func NewSubscriptionHandler() *SubscriptionHandler {
	return &SubscriptionHandler{}
}

func (h *SubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var request dto.SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNotImplemented)
}

func (h *SubscriptionHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("token")
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *SubscriptionHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("token")
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	_ = r.URL.Query().Get("email")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode([]dto.SubscriptionResponse{})
}
