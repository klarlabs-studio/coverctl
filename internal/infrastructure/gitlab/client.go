// Package gitlab provides GitLab API client for MR comments.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	// DefaultAPIURL is the default GitLab API endpoint
	DefaultAPIURL = "https://gitlab.com/api/v4"
	// CommentMarker identifies coverctl comments for updates
	CommentMarker = "<!-- coverctl-coverage-report -->"
	// maxResponseBytes caps how much of an API response body we buffer, to
	// avoid unbounded memory use if a server returns a huge (or malicious) body.
	maxResponseBytes = 5 << 20 // 5 MiB
)

// checkRedirect refuses to follow a redirect that crosses to a different host.
//
// Go's http.Client strips Authorization/Cookie on a cross-host redirect but
// NOT custom headers such as GitLab's PRIVATE-TOKEN, so an open redirect to an
// attacker-controlled host would otherwise forward the token. Same-host
// redirects (path-only changes) remain allowed.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if req.URL.Host != via[0].URL.Host {
		return fmt.Errorf("refusing cross-host redirect to %q", req.URL.Host)
	}
	return nil
}

// Client implements the PRClient interface for GitLab API.
type Client struct {
	httpClient *http.Client
	apiURL     string
	token      string
}

// Provider returns the provider type.
func (c *Client) Provider() application.PRProvider {
	return application.ProviderGitLab
}

// NewClient creates a new GitLab client.
// Token is read from GITLAB_TOKEN or CI_JOB_TOKEN environment variable if not provided.
func NewClient(token string) *Client {
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
		if token == "" {
			token = os.Getenv("CI_JOB_TOKEN")
		}
	}
	return &Client{
		httpClient: &http.Client{Timeout: DefaultHTTPTimeout, CheckRedirect: checkRedirect},
		apiURL:     DefaultAPIURL,
		token:      token,
	}
}

// NewClientWithHTTP creates a client with a custom HTTP client and API URL.
//
// SECURITY: this constructor accepts an arbitrary apiURL and is FOR TESTS
// ONLY. Production callers must use NewClient, which pins the URL to
// gitlab.com. Allowing user-controlled apiURL in a code path that also
// receives a token would be an SSRF / token-exfiltration sink: an attacker
// who can influence the URL (e.g. via MCP input or a config field) could
// redirect the request to a host they control and harvest the token.
//
// A fitness function in internal/architecture/architecture_test.go enforces
// that no production code (cli, mcp, application) references this function.
func NewClientWithHTTP(token string, httpClient *http.Client, apiURL string) *Client {
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
		if token == "" {
			token = os.Getenv("CI_JOB_TOKEN")
		}
	}
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	// Apply the same cross-host redirect protection as NewClient. Guard against
	// http.DefaultClient so we never mutate the process-wide shared client.
	if httpClient != nil && httpClient != http.DefaultClient {
		httpClient.CheckRedirect = checkRedirect
	}
	return &Client{
		httpClient: httpClient,
		apiURL:     apiURL,
		token:      token,
	}
}

// note represents a GitLab MR note (comment).
type note struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	NoteURL string `json:"noteable_iid"` //nolint:misspell // GitLab API field name
}

// projectPath returns the URL-encoded project path for API calls.
func projectPath(owner, repo string) string {
	return url.PathEscape(owner + "/" + repo)
}

// FindCoverageComment finds an existing coverage comment on a MR.
// Returns 0 if no comment found.
func (c *Client) FindCoverageComment(ctx context.Context, owner, repo string, mrNumber int) (int64, error) {
	project := projectPath(owner, repo)
	apiURL := fmt.Sprintf("%s/projects/%s/merge_requests/%d/notes", c.apiURL, project, mrNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		return 0, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(body))
	}

	var notes []note
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&notes); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	// Find note with our marker
	for _, n := range notes {
		if strings.Contains(n.Body, CommentMarker) {
			return n.ID, nil
		}
	}

	return 0, nil
}

// CreateComment creates a new comment on a MR.
// Returns the comment ID and URL.
func (c *Client) CreateComment(ctx context.Context, owner, repo string, mrNumber int, body string) (int64, string, error) {
	project := projectPath(owner, repo)
	apiURL := fmt.Sprintf("%s/projects/%s/merge_requests/%d/notes", c.apiURL, project, mrNumber)

	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(jsonBody))
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
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		return 0, "", fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	var n note
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&n); err != nil {
		return 0, "", fmt.Errorf("decode response: %w", err)
	}

	// Construct URL to the comment
	commentURL := fmt.Sprintf("https://gitlab.com/%s/%s/-/merge_requests/%d#note_%d", owner, repo, mrNumber, n.ID)
	if c.apiURL != DefaultAPIURL {
		// For self-hosted GitLab, extract base URL
		baseURL := strings.TrimSuffix(c.apiURL, "/api/v4")
		commentURL = fmt.Sprintf("%s/%s/%s/-/merge_requests/%d#note_%d", baseURL, owner, repo, mrNumber, n.ID)
	}

	return n.ID, commentURL, nil
}

// UpdateComment updates an existing comment.
func (c *Client) UpdateComment(ctx context.Context, owner, repo string, noteID int64, body string) error {
	project := projectPath(owner, repo)
	// For GitLab, we need the MR number to update a note, but we don't have it here
	// We'll use a workaround by finding the note first in the MR context
	// Actually, GitLab API allows updating notes with just the project and note ID
	apiURL := fmt.Sprintf("%s/projects/%s/notes/%d", c.apiURL, project, noteID)

	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(jsonBody))
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
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		return fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// setHeaders sets common headers for GitLab API requests.
func (c *Client) setHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
}
