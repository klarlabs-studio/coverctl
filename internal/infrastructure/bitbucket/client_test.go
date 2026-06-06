package bitbucket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

// newTestServer creates an httptest.Server and returns a Client wired to it.
func newTestServer(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClientWithHTTP("testuser", "testpass", srv.Client(), srv.URL)
	return client, srv
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
		json.NewEncoder(w).Encode(commentList{})
	})

	client, _ := newTestServer(t, handler)

	_, _ = client.FindCoverageComment(context.Background(), "ws", "repo", 1)

	require.True(t, authOK, "basic auth header must be present")
	assert.Equal(t, "testuser", capturedUsername)
	assert.Equal(t, "testpass", capturedPassword)
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
					json.NewEncoder(w).Encode(tt.response)
				} else {
					w.Write([]byte("error body"))
				}
			})

			client, _ := newTestServer(t, handler)
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
					json.NewEncoder(w).Encode(tt.response)
				} else {
					w.Write([]byte("bad request"))
				}
			})

			client, _ := newTestServer(t, handler)
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
					w.Write([]byte("error"))
				}
			})

			client, _ := newTestServer(t, handler)
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
		json.NewEncoder(w).Encode(commentList{})
	})

	client, _ := newTestServer(t, handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.FindCoverageComment(ctx, "ws", "repo", 1)
	require.Error(t, err)
}

func TestCreateComment_ContextCancelled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(comment{})
	})

	client, _ := newTestServer(t, handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := client.CreateComment(ctx, "ws", "repo", 1, "body")
	require.Error(t, err)
}

func TestUpdateComment_ContextCancelled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client, _ := newTestServer(t, handler)

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
		json.NewEncoder(w).Encode(commentList{})
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
