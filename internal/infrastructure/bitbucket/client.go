// Package bitbucket provides Bitbucket API client for PR comments.
package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.klarlabs.de/coverctl/internal/application"
)

const (
	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30 * time.Second
)

const (
	// DefaultAPIURL is the default Bitbucket API endpoint
	DefaultAPIURL = "https://api.bitbucket.org/2.0"
	// CommentMarker identifies coverctl comments for updates
	CommentMarker = "<!-- coverctl-coverage-report -->"
)

// Client implements the PRClient interface for Bitbucket API.
type Client struct {
	httpClient  *http.Client
	apiURL      string
	username    string
	appPassword string
}

// Provider returns the provider type.
func (c *Client) Provider() application.PRProvider {
	return application.ProviderBitbucket
}

// NewClient creates a new Bitbucket client.
// Credentials are read from BITBUCKET_USERNAME and BITBUCKET_APP_PASSWORD environment variables if not provided.
func NewClient(username, appPassword string) *Client {
	if username == "" {
		username = os.Getenv("BITBUCKET_USERNAME")
	}
	if appPassword == "" {
		appPassword = os.Getenv("BITBUCKET_APP_PASSWORD")
		if appPassword == "" {
			// Also check BITBUCKET_TOKEN for compatibility
			appPassword = os.Getenv("BITBUCKET_TOKEN")
		}
	}
	return &Client{
		httpClient:  &http.Client{Timeout: DefaultHTTPTimeout},
		apiURL:      DefaultAPIURL,
		username:    username,
		appPassword: appPassword,
	}
}

// NewClientWithHTTP creates a client with a custom HTTP client (for testing).
func NewClientWithHTTP(username, appPassword string, httpClient *http.Client, apiURL string) *Client {
	if username == "" {
		username = os.Getenv("BITBUCKET_USERNAME")
	}
	if appPassword == "" {
		appPassword = os.Getenv("BITBUCKET_APP_PASSWORD")
		if appPassword == "" {
			appPassword = os.Getenv("BITBUCKET_TOKEN")
		}
	}
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	return &Client{
		httpClient:  httpClient,
		apiURL:      apiURL,
		username:    username,
		appPassword: appPassword,
	}
}

// comment represents a Bitbucket PR comment.
type comment struct {
	ID      int64 `json:"id"`
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

// commentList represents the paginated response from Bitbucket.
type commentList struct {
	Values []comment `json:"values"`
	Next   string    `json:"next"`
}

// FindCoverageComment finds an existing coverage comment on a PR.
// Returns 0 if no comment found.
func (c *Client) FindCoverageComment(ctx context.Context, workspace, repoSlug string, prNumber int) (int64, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/comments", c.apiURL, workspace, repoSlug, prNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("bitbucket API error: %s - %s", resp.Status, string(body))
	}

	var comments commentList
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	// Find comment with our marker
	for _, comment := range comments.Values {
		if strings.Contains(comment.Content.Raw, CommentMarker) {
			return comment.ID, nil
		}
	}

	return 0, nil
}

// CreateComment creates a new comment on a PR.
// Returns the comment ID and URL.
func (c *Client) CreateComment(ctx context.Context, workspace, repoSlug string, prNumber int, body string) (int64, string, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/comments", c.apiURL, workspace, repoSlug, prNumber)

	payload := map[string]any{
		"content": map[string]string{
			"raw": body,
		},
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return 0, "", fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, "", fmt.Errorf("bitbucket API error: %s - %s", resp.Status, string(respBody))
	}

	var comment comment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return 0, "", fmt.Errorf("decode response: %w", err)
	}

	return comment.ID, comment.Links.HTML.Href, nil
}

// UpdateComment updates an existing comment.
func (c *Client) UpdateComment(ctx context.Context, workspace, repoSlug string, commentID int64, body string) error {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/comments/%d", c.apiURL, workspace, repoSlug, commentID)

	payload := map[string]any{
		"content": map[string]string{
			"raw": body,
		},
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bitbucket API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// setHeaders sets common headers for Bitbucket API requests.
func (c *Client) setHeaders(req *http.Request) {
	if c.username != "" && c.appPassword != "" {
		req.SetBasicAuth(c.username, c.appPassword)
	}
}
