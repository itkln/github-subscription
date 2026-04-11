package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/itkln/github-subscription/internal/platform/metrics"
)

const (
	githubAPIVersion = "2026-03-10"
	acceptHeader     = "application/vnd.github+json"
)

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

type repositoryResponse struct {
	ID int64 `json:"id"`
}

var errNotFound = errors.New("github resource not found")

type Client struct {
	baseURL    string
	cache      Cache
	cacheTTL   time.Duration
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string, cache Cache, cacheTTL time.Duration) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		cache:    cache,
		cacheTTL: cacheTTL,
		token:    token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) LatestReleaseTag(ctx context.Context, repo string) (string, error) {
	var payload []releaseResponse
	if err := c.getJSON(ctx, "/repos/"+repo+"/releases?per_page=1", &payload); err != nil {
		if errors.Is(err, errNotFound) {
			return "", nil
		}

		return "", fmt.Errorf("get latest release: %w", err)
	}

	if len(payload) == 0 {
		return "", nil
	}

	return payload[0].TagName, nil
}

func (c *Client) RepositoryExists(ctx context.Context, repo string) (bool, error) {
	var payload repositoryResponse
	if err := c.getJSON(ctx, "/repos/"+repo, &payload); err != nil {
		if errors.Is(err, errNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("check repository: %w", err)
	}

	return payload.ID != 0, nil
}

func (c *Client) getJSON(ctx context.Context, path string, target any) (err error) {
	if err := c.loadFromCache(ctx, path, target); err == nil {
		return nil
	}

	request, err := c.newRequest(ctx, path)
	if err != nil {
		return err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("do github request: %w", err)
	}
	defer func() {
		closeErr := response.Body.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("close github response body: %w", closeErr)
		}
	}()

	endpoint := githubEndpointLabel(path)
	metrics.RecordGitHubRequest(endpoint, response.StatusCode)

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return errNotFound
	case http.StatusTooManyRequests:
		metrics.RecordGitHubRateLimit()
		return newRateLimitError(response)
	case http.StatusForbidden:
		if response.Header.Get("X-RateLimit-Remaining") == "0" {
			metrics.RecordGitHubRateLimit()
			return newRateLimitError(response)
		}

		return fmt.Errorf("unexpected github status: %s", response.Status)
	default:
		return fmt.Errorf("unexpected github status: %s", response.Status)
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}

	_ = c.storeInCache(ctx, path, target)

	return nil
}

func (c *Client) newRequest(ctx context.Context, path string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create github request: %w", err)
	}

	request.Header.Set("Accept", acceptHeader)
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	return request, nil
}

func githubEndpointLabel(path string) string {
	switch {
	case strings.Contains(path, "/releases"):
		return "releases"
	default:
		return "repository"
	}
}

func (c *Client) loadFromCache(ctx context.Context, path string, target any) error {
	if c.cache == nil {
		return errCacheMiss
	}

	value, err := c.cache.Get(ctx, cacheKey(path))
	if err != nil {
		return errCacheMiss
	}

	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("unmarshal github cache: %w", err)
	}

	return nil
}

func (c *Client) storeInCache(ctx context.Context, path string, value any) error {
	if c.cache == nil {
		return nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal github cache: %w", err)
	}

	if err := c.cache.Set(ctx, cacheKey(path), string(payload), c.cacheTTL); err != nil {
		return err
	}

	return nil
}
