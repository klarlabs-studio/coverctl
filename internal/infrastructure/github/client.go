// Package github provides GitHub API client for PR comments.
package github

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
	// DefaultAPIURL is the default GitHub API endpoint
	DefaultAPIURL = "https://api.github.com"
	// CommentMarker identifies coverctl comments for updates
	CommentMarker = "<!-- coverctl-coverage-report -->"
)

// Client implements the PRClient interface for GitHub API.
type Client struct {
	httpClient *http.Client
	apiURL     string
	token      string
}

// Provider returns the provider type.
func (c *Client) Provider() application.PRProvider {
	return application.ProviderGitHub
}

// NewClient creates a new GitHub client.
// Token is read from GITHUB_TOKEN environment variable if not provided.
func NewClient(token string) *Client {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return &Client{
		httpClient: &http.Client{Timeout: DefaultHTTPTimeout},
		apiURL:     DefaultAPIURL,
		token:      token,
	}
}

// NewClientWithHTTP creates a client with a custom HTTP client and API URL.
//
// SECURITY: this constructor accepts an arbitrary apiURL and is FOR TESTS
// ONLY. Production callers must use NewClient, which pins the URL to
// api.github.com. Allowing user-controlled apiURL in a code path that also
// receives a Bearer token would be an SSRF / token-exfiltration sink: an
// attacker who can influence the URL (e.g. via MCP input or a config field)
// could redirect the request to a host they control and harvest the token.
//
// A fitness function in internal/architecture/architecture_test.go enforces
// that no production code (cli, mcp, application) references this function.
func NewClientWithHTTP(token string, httpClient *http.Client, apiURL string) *Client {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	return &Client{
		httpClient: httpClient,
		apiURL:     apiURL,
		token:      token,
	}
}

// issueComment represents a GitHub issue/PR comment.
type issueComment struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

// FindCoverageComment finds an existing coverage comment on a PR.
// Returns 0 if no comment found.
func (c *Client) FindCoverageComment(ctx context.Context, owner, repo string, prNumber int) (int64, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.apiURL, owner, repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
	}

	var comments []issueComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	// Find comment with our marker
	for _, comment := range comments {
		if strings.Contains(comment.Body, CommentMarker) {
			return comment.ID, nil
		}
	}

	return 0, nil
}

// CreateComment creates a new comment on a PR.
// Returns the comment ID and URL.
func (c *Client) CreateComment(ctx context.Context, owner, repo string, prNumber int, body string) (int64, string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.apiURL, owner, repo, prNumber)

	payload := map[string]string{"body": body}
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
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, "", fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	var comment issueComment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return 0, "", fmt.Errorf("decode response: %w", err)
	}

	return comment.ID, comment.HTMLURL, nil
}

// UpdateComment updates an existing comment.
func (c *Client) UpdateComment(ctx context.Context, owner, repo string, commentID int64, body string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", c.apiURL, owner, repo, commentID)

	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// setHeaders sets common headers for GitHub API requests.
func (c *Client) setHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}
