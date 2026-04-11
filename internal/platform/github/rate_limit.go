package github

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type RateLimitError struct {
	StatusCode int
	RetryAfter time.Duration
	ResetAt    time.Time
}

func (e *RateLimitError) Error() string {
	if !e.ResetAt.IsZero() {
		return fmt.Sprintf("github rate limit exceeded: status=%d reset_at=%s", e.StatusCode, e.ResetAt.UTC().Format(time.RFC3339))
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("github rate limit exceeded: status=%d retry_after=%s", e.StatusCode, e.RetryAfter)
	}

	return fmt.Sprintf("github rate limit exceeded: status=%d", e.StatusCode)
}

func (e *RateLimitError) IsRateLimit() bool {
	return true
}

func (e *RateLimitError) RetryAfterDuration() time.Duration {
	return e.RetryAfter
}

func newRateLimitError(response *http.Response) error {
	err := &RateLimitError{StatusCode: response.StatusCode}

	if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
		seconds, convErr := strconv.Atoi(retryAfter)
		if convErr == nil && seconds > 0 {
			err.RetryAfter = time.Duration(seconds) * time.Second
		}
	}

	if resetAt := response.Header.Get("X-RateLimit-Reset"); resetAt != "" {
		seconds, convErr := strconv.ParseInt(resetAt, 10, 64)
		if convErr == nil && seconds > 0 {
			err.ResetAt = time.Unix(seconds, 0)
			if err.RetryAfter == 0 {
				err.RetryAfter = time.Until(err.ResetAt)
				if err.RetryAfter < 0 {
					err.RetryAfter = 0
				}
			}
		}
	}

	return err
}
