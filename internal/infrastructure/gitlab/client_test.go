package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

// roundTripFunc adapts a function to the http.RoundTripper interface.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProvider(t *testing.T) {
	client := NewClient("test-token")
	assert.Equal(t, application.ProviderGitLab, client.Provider())
}

func TestAuthHeader(t *testing.T) {
	var receivedHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("PRIVATE-TOKEN")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	client := NewClientWithHTTP("my-secret-token", srv.Client(), srv.URL)
	_, _ = client.FindCoverageComment(context.Background(), "owner", "repo", 1)

	assert.Equal(t, "my-secret-token", receivedHeader)
}

func TestAuthHeaderOmittedWhenEmpty(t *testing.T) {
	var headerPresent bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, headerPresent = r.Header["Private-Token"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	// Unset env vars to ensure token stays empty.
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")

	client := NewClientWithHTTP("", srv.Client(), srv.URL)
	_, _ = client.FindCoverageComment(context.Background(), "owner", "repo", 1)

	assert.False(t, headerPresent, "PRIVATE-TOKEN header should not be set when token is empty")
}

func TestFindCoverageCommentRejectsCrossHostRedirect(t *testing.T) {
	var attackerHit bool
	var attackerToken string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attackerHit = true
		attackerToken = r.Header.Get("PRIVATE-TOKEN")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer attacker.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect to a different host, as an open-redirect attacker would.
		http.Redirect(w, r, attacker.URL+"/steal", http.StatusFound)
	}))
	defer primary.Close()

	client := NewClientWithHTTP("super-secret-token", primary.Client(), primary.URL)
	_, err := client.FindCoverageComment(context.Background(), "owner", "repo", 1)

	require.Error(t, err)
	assert.False(t, attackerHit, "client must not follow a redirect to a different host")
	assert.Empty(t, attackerToken, "PRIVATE-TOKEN must not be forwarded to the attacker host")
}

func TestFindCoverageCommentBoundsResponseBody(t *testing.T) {
	// A single JSON value larger than the cap is truncated by the LimitReader,
	// producing a decode error rather than buffering an unbounded body.
	oversized := bytes.Repeat([]byte("a"), maxResponseBytes+1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"body":"`))
		_, _ = w.Write(oversized)
		// Intentionally never closed: the body exceeds maxResponseBytes.
	}))
	defer srv.Close()

	client := NewClientWithHTTP("token", srv.Client(), srv.URL)
	_, err := client.FindCoverageComment(context.Background(), "owner", "repo", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestFindCoverageComment(t *testing.T) {
	tests := []struct {
		name       string
		notes      []note
		statusCode int
		wantID     int64
		wantErr    bool
		errContain string
	}{
		{
			name: "found comment with marker",
			notes: []note{
				{ID: 10, Body: "unrelated comment"},
				{ID: 42, Body: "Coverage report\n" + CommentMarker},
				{ID: 99, Body: "another comment"},
			},
			statusCode: http.StatusOK,
			wantID:     42,
		},
		{
			name: "no comment with marker",
			notes: []note{
				{ID: 10, Body: "unrelated comment"},
				{ID: 20, Body: "no marker here"},
			},
			statusCode: http.StatusOK,
			wantID:     0,
		},
		{
			name:       "empty notes list",
			notes:      []note{},
			statusCode: http.StatusOK,
			wantID:     0,
		},
		{
			name:       "API error returns error",
			statusCode: http.StatusForbidden,
			wantErr:    true,
			errContain: "GitLab API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.RequestURI, "/projects/my-group%2Fmy-repo/merge_requests/5/notes")

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					resp, _ := json.Marshal(tt.notes)
					_, _ = w.Write(resp)
				} else {
					_, _ = w.Write([]byte(`{"error":"forbidden"}`))
				}
			}))
			defer srv.Close()

			client := NewClientWithHTTP("token", srv.Client(), srv.URL)
			id, err := client.FindCoverageComment(context.Background(), "my-group", "my-repo", 5)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestCreateComment(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		respNote   note
		wantID     int64
		wantURLSub string
		wantErr    bool
		errContain string
	}{
		{
			name:       "success",
			statusCode: http.StatusCreated,
			respNote:   note{ID: 77},
			wantID:     77,
			wantURLSub: "merge_requests/3#note_77",
		},
		{
			name:       "API error",
			statusCode: http.StatusUnprocessableEntity,
			wantErr:    true,
			errContain: "GitLab API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Contains(t, r.RequestURI, "/projects/owner%2Frepo/merge_requests/3/notes")

				_ = json.NewDecoder(r.Body).Decode(&receivedBody)

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusCreated {
					resp, _ := json.Marshal(tt.respNote)
					_, _ = w.Write(resp)
				} else {
					_, _ = w.Write([]byte(`{"error":"validation failed"}`))
				}
			}))
			defer srv.Close()

			client := NewClientWithHTTP("token", srv.Client(), srv.URL)
			id, commentURL, err := client.CreateComment(context.Background(), "owner", "repo", 3, "test body")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Contains(t, commentURL, tt.wantURLSub)
			assert.Equal(t, "test body", receivedBody["body"])
		})
	}
}

func TestCreateCommentSelfHostedURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		resp, _ := json.Marshal(note{ID: 55})
		_, _ = w.Write(resp)
	}))
	defer srv.Close()

	// The test server URL acts as a self-hosted GitLab instance.
	// NewClientWithHTTP receives apiURL directly (not with /api/v4 suffix in this test),
	// but to test the self-hosted URL logic, we need an apiURL that differs from DefaultAPIURL
	// and ends with /api/v4 so TrimSuffix works correctly.
	selfHostedAPI := srv.URL + "/api/v4"
	client := NewClientWithHTTP("token", srv.Client(), selfHostedAPI)

	_, commentURL, err := client.CreateComment(context.Background(), "myorg", "myproj", 10, "body")
	require.NoError(t, err)

	// Self-hosted URL should use the base URL from apiURL, not gitlab.com.
	expected := srv.URL + "/myorg/myproj/-/merge_requests/10#note_55"
	assert.Equal(t, expected, commentURL)
}

func TestCreateCommentDefaultGitLabURL(t *testing.T) {
	// To test the default gitlab.com URL construction branch, we use a transport
	// that intercepts the outgoing request and returns a canned response, so the
	// client's apiURL can remain set to DefaultAPIURL (gitlab.com).
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				resp := &http.Response{
					StatusCode: http.StatusCreated,
					Header:     make(http.Header),
				}
				body, _ := json.Marshal(note{ID: 33})
				resp.Body = io.NopCloser(bytes.NewReader(body))
				return resp, nil
			}),
		},
		apiURL: DefaultAPIURL,
		token:  "token",
	}

	_, commentURL, err := client.CreateComment(context.Background(), "myorg", "myproj", 7, "body")
	require.NoError(t, err)
	assert.Equal(t, "https://gitlab.com/myorg/myproj/-/merge_requests/7#note_33", commentURL)
}

func TestUpdateComment(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		errContain string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
		},
		{
			name:       "API error",
			statusCode: http.StatusNotFound,
			wantErr:    true,
			errContain: "GitLab API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Contains(t, r.RequestURI, "/projects/ns%2Frepo/notes/99")

				_ = json.NewDecoder(r.Body).Decode(&receivedBody)

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_, _ = w.Write([]byte(`{}`))
				} else {
					_, _ = w.Write([]byte(`{"error":"not found"}`))
				}
			}))
			defer srv.Close()

			client := NewClientWithHTTP("token", srv.Client(), srv.URL)
			err := client.UpdateComment(context.Background(), "ns", "repo", 99, "updated body")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "updated body", receivedBody["body"])
		})
	}
}

func TestProjectPathEncoding(t *testing.T) {
	// Verify that owner/repo with special characters is properly encoded.
	result := projectPath("my-group/sub-group", "my-repo")
	assert.Equal(t, "my-group%2Fsub-group%2Fmy-repo", result)
}

func TestNewClientEnvFallback(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "env-token")
	t.Setenv("CI_JOB_TOKEN", "ci-token")

	client := NewClient("")
	assert.Equal(t, "env-token", client.token)
}

func TestNewClientCIJobTokenFallback(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "ci-token")

	client := NewClient("")
	assert.Equal(t, "ci-token", client.token)
}

func TestNewClientWithHTTPDefaults(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "")

	client := NewClientWithHTTP("", http.DefaultClient, "")
	assert.Equal(t, DefaultAPIURL, client.apiURL)
	assert.Equal(t, "", client.token)
}
