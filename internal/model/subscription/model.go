package subscription

type DBSubscription struct {
	ID               int64
	Email            string
	Repo             string
	Confirmed        bool
	ConfirmToken     string
	UnsubscribeToken string
	LastSeenTag      string
}

type CreateParams struct {
	Email            string
	Repo             string
	ConfirmToken     string
	UnsubscribeToken string
}
