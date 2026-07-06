package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

// rawContent builds the anonymous content struct used by comment values.
func rawContent(raw string) struct {
	Raw string `json:"raw"`
} {
	return struct {
		Raw string `json:"raw"`
	}{Raw: raw}
}

// newTestServer creates an httptest.Server and returns a Client wired to it.
// The server is registered for cleanup via t.Cleanup, so callers only need
// the Client.
func newTestServer(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClientWithHTTP("testuser", "testpass", srv.Client(), srv.URL)
}

func TestProvider(t *testing.T) {
	c := NewClient("user", "pass")
	assert.Equal(t, application.ProviderBitbucket, c.Provider())
}

func TestNewClient_EnvFallback(t *testing.T) {
	t.Setenv("BITBUCKET_USERNAME", "envuser")
	t.Setenv("BITBUCKET_APP_PASSWORD", "envpass")

	c := NewClient("", "")
	assert.Equal(t, "envuser", c.username)
	assert.Equal(t, "envpass", c.appPassword)
}

func TestNewClient_TokenFallback(t *testing.T) {
	t.Setenv("BITBUCKET_USERNAME", "envuser")
	t.Setenv("BITBUCKET_APP_PASSWORD", "")
	t.Setenv("BITBUCKET_TOKEN", "tokenpass")

	c := NewClient("", "")
	assert.Equal(t, "tokenpass", c.appPassword)
}

func TestNewClientWithHTTP_DefaultAPIURL(t *testing.T) {
	c := NewClientWithHTTP("user", "pass", http.DefaultClient, "")
	assert.Equal(t, DefaultAPIURL, c.apiURL)
}

func TestBasicAuthHeader(t *testing.T) {
	var capturedUsername, capturedPassword string
	var authOK bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUsername, capturedPassword, authOK = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(commentList{})
	})

	client := newTestServer(t, handler)

	_, _ = client.FindCoverageComment(context.Background(), "ws", "repo", 1)

	require.True(t, authOK, "basic auth header must be present")
	assert.Equal(t, "testuser", capturedUsername)
	assert.Equal(t, "testpass", capturedPassword)
}

func TestFindCoverageCommentRejectsCrossHostRedirect(t *testing.T) {
	var attackerHit bool
	var attackerUser, attackerPass string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attackerHit = true
		attackerUser, attackerPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(commentList{})
	}))
	defer attacker.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL+"/steal", http.StatusFound)
	}))
	defer primary.Close()

	client := NewClientWithHTTP("secretuser", "secretpass", primary.Client(), primary.URL)
	_, err := client.FindCoverageComment(context.Background(), "ws", "repo", 1)

	require.Error(t, err)
	assert.False(t, attackerHit, "client must not follow a redirect to a different host")
	assert.Empty(t, attackerUser, "credentials must not be forwarded to the attacker host")
	assert.Empty(t, attackerPass, "credentials must not be forwarded to the attacker host")
}

func TestFindCoverageCommentEscapesPathSegments(t *testing.T) {
	var gotEscapedPath, gotRawQuery string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(commentList{})
	})

	client := newTestServer(t, handler)
	_, err := client.FindCoverageComment(context.Background(), "ws", "../evil?injected=1", 1)
	require.NoError(t, err)
	assert.Contains(t, gotEscapedPath, "..%2Fevil%3Finjected=1")
	assert.Empty(t, gotRawQuery, "query characters must not leak into the request query")
}

func TestFindCoverageCommentBoundsResponseBody(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), maxResponseBytes+1024)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"values":[{"id":1,"content":{"raw":"`))
		_, _ = w.Write(oversized)
		// Intentionally never closed: the body exceeds maxResponseBytes.
	})

	client := newTestServer(t, handler)
	_, err := client.FindCoverageComment(context.Background(), "ws", "repo", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestFindCoverageCommentFollowsPagination(t *testing.T) {
	var requestCount int
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("page") == "2" {
			// Page 2 carries the existing coverctl comment.
			_ = json.NewEncoder(w).Encode(commentList{
				Values: []comment{
					{ID: 77, Content: rawContent("Coverage report\n" + CommentMarker)},
				},
			})
			return
		}
		// Page 1: no marker, but points to page 2 on the same host.
		_ = json.NewEncoder(w).Encode(commentList{
			Values: []comment{{ID: 10, Content: rawContent("unrelated comment")}},
			Next:   baseURL + "/repositories/ws/repo/pullrequests/5/comments?page=2",
		})
	}))
	defer srv.Close()
	baseURL = srv.URL

	client := NewClientWithHTTP("testuser", "testpass", srv.Client(), srv.URL)
	id, err := client.FindCoverageComment(context.Background(), "ws", "repo", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(77), id, "must find the comment on page 2 via the next link")
	assert.Equal(t, 2, requestCount, "must follow pagination to the second page")
}

func TestFindCoverageComment(t *testing.T) {
	tests := []struct {
		name       string
		response   commentList
		statusCode int
		wantID     int64
		wantErr    bool
		errContain string
	}{
		{
			name: "comment found",
			response: commentList{
				Values: []comment{
					{ID: 10, Content: struct {
						Raw string `json:"raw"`
					}{Raw: "unrelated comment"}},
					{ID: 42, Content: struct {
						Raw string `json:"raw"`
					}{Raw: "Coverage report\n" + CommentMarker}},
				},
			},
			statusCode: http.StatusOK,
			wantID:     42,
		},
		{
			name: "comment not found",
			response: commentList{
				Values: []comment{
					{ID: 10, Content: struct {
						Raw string `json:"raw"`
					}{Raw: "some other comment"}},
				},
			},
			statusCode: http.StatusOK,
			wantID:     0,
		},
		{
			name:       "empty comment list",
			response:   commentList{},
			statusCode: http.StatusOK,
			wantID:     0,
		},
		{
			name:       "API error 403",
			statusCode: http.StatusForbidden,
			wantErr:    true,
			errContain: "bitbucket API error",
		},
		{
			name:       "API error 500",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			errContain: "bitbucket API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/repositories/ws/repo/pullrequests/5/comments")

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.response)
				} else {
					_, _ = w.Write([]byte("error body"))
				}
			})

			client := newTestServer(t, handler)
			id, err := client.FindCoverageComment(context.Background(), "ws", "repo", 5)

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
		response   comment
		wantID     int64
		wantURL    string
		wantErr    bool
		errContain string
	}{
		{
			name:       "success",
			statusCode: http.StatusCreated,
			response: comment{
				ID: 99,
				Content: struct {
					Raw string `json:"raw"`
				}{Raw: "test body"},
				Links: struct {
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				}{HTML: struct {
					Href string `json:"href"`
				}{Href: "https://bitbucket.org/ws/repo/pull-requests/3#comment-99"}},
			},
			wantID:  99,
			wantURL: "https://bitbucket.org/ws/repo/pull-requests/3#comment-99",
		},
		{
			name:       "API error 400",
			statusCode: http.StatusBadRequest,
			wantErr:    true,
			errContain: "bitbucket API error",
		},
		{
			name:       "API error 401",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			errContain: "bitbucket API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Contains(t, r.URL.Path, "/repositories/ws/repo/pullrequests/3/comments")

				// Verify request body structure
				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				content, ok := payload["content"].(map[string]any)
				require.True(t, ok, "payload must contain content object")
				assert.Equal(t, "test body", content["raw"])

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusCreated {
					_ = json.NewEncoder(w).Encode(tt.response)
				} else {
					_, _ = w.Write([]byte("bad request"))
				}
			})

			client := newTestServer(t, handler)
			id, url, err := client.CreateComment(context.Background(), "ws", "repo", 3, "test body")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantURL, url)
		})
	}
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
			name:       "API error 404",
			statusCode: http.StatusNotFound,
			wantErr:    true,
			errContain: "bitbucket API error",
		},
		{
			name:       "API error 500",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			errContain: "bitbucket API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Contains(t, r.URL.Path, "/repositories/ws/repo/pullrequests/comments/55")

				// Verify request body structure
				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				content, ok := payload["content"].(map[string]any)
				require.True(t, ok, "payload must contain content object")
				assert.Equal(t, "updated body", content["raw"])

				w.WriteHeader(tt.statusCode)
				if tt.statusCode != http.StatusOK {
					_, _ = w.Write([]byte("error"))
				}
			})

			client := newTestServer(t, handler)
			err := client.UpdateComment(context.Background(), "ws", "repo", 55, "updated body")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestFindCoverageComment_ContextCancelled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(commentList{})
	})

	client := newTestServer(t, handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.FindCoverageComment(ctx, "ws", "repo", 1)
	require.Error(t, err)
}

func TestCreateComment_ContextCancelled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(comment{})
	})

	client := newTestServer(t, handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := client.CreateComment(ctx, "ws", "repo", 1, "body")
	require.Error(t, err)
}

func TestUpdateComment_ContextCancelled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := newTestServer(t, handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.UpdateComment(ctx, "ws", "repo", 1, "body")
	require.Error(t, err)
}

func TestSetHeaders_NoAuthWhenCredentialsEmpty(t *testing.T) {
	var hasAuth bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, hasAuth = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(commentList{})
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// Clear env vars so empty strings stay empty
	t.Setenv("BITBUCKET_USERNAME", "")
	t.Setenv("BITBUCKET_APP_PASSWORD", "")
	t.Setenv("BITBUCKET_TOKEN", "")

	client := NewClientWithHTTP("", "", srv.Client(), srv.URL)
	_, _ = client.FindCoverageComment(context.Background(), "ws", "repo", 1)

	assert.False(t, hasAuth, "no auth header should be set when credentials are empty")
}
