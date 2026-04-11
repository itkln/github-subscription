package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	githubAPIVersion = "2026-03-10"
	acceptHeader     = "application/vnd.github+json"
)

var errNotFound = errors.New("github resource not found")

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

type repositoryResponse struct {
	ID int64 `json:"id"`
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
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

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	request, err := c.newRequest(ctx, path)
	if err != nil {
		return err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("do github request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return errNotFound
	default:
		return fmt.Errorf("unexpected github status: %s", response.Status)
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}

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
