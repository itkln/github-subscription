package dto

type SubscribeRequest struct {
	Email string `json:"email"`
	Repo  string `json:"repo"`
}

type SubscriptionResponse struct {
	Email       string `json:"email"`
	Repo        string `json:"repo"`
	Confirmed   bool   `json:"confirmed"`
	LastSeenTag string `json:"last_seen_tag,omitempty"`
}
