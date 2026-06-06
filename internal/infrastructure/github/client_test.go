package github

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

func TestProvider(t *testing.T) {
	client := NewClient("")
	assert.Equal(t, application.ProviderGitHub, client.Provider())
}

func TestNewClient(t *testing.T) {
	t.Run("uses provided token", func(t *testing.T) {
		client := NewClient("test-token")
		assert.Equal(t, "test-token", client.token)
		assert.Equal(t, DefaultAPIURL, client.apiURL)
	})

	t.Run("defaults apiURL when empty in NewClientWithHTTP", func(t *testing.T) {
		client := NewClientWithHTTP("tok", http.DefaultClient, "")
		assert.Equal(t, DefaultAPIURL, client.apiURL)
	})

	t.Run("uses custom apiURL", func(t *testing.T) {
		client := NewClientWithHTTP("tok", http.DefaultClient, "https://custom.api")
		assert.Equal(t, "https://custom.api", client.apiURL)
	})
}

func TestAuthHeader(t *testing.T) {
	var capturedAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]issueComment{})
	}))
	defer server.Close()

	t.Run("sets bearer token when provided", func(t *testing.T) {
		client := NewClientWithHTTP("my-secret-token", server.Client(), server.URL)
		_, err := client.FindCoverageComment(context.Background(), "owner", "repo", 1)
		require.NoError(t, err)
		assert.Equal(t, "Bearer my-secret-token", capturedAuthHeader)
	})

	t.Run("no auth header when token is empty", func(t *testing.T) {
		capturedAuthHeader = ""
		client := NewClientWithHTTP("", server.Client(), server.URL)
		// Clear GITHUB_TOKEN to avoid env leakage
		t.Setenv("GITHUB_TOKEN", "")
		client.token = ""
		_, err := client.FindCoverageComment(context.Background(), "owner", "repo", 1)
		require.NoError(t, err)
		assert.Empty(t, capturedAuthHeader)
	})
}

func TestFindCoverageComment(t *testing.T) {
	tests := []struct {
		name       string
		comments   []issueComment
		statusCode int
		wantID     int64
		wantErr    bool
	}{
		{
			name: "finds comment with marker",
			comments: []issueComment{
				{ID: 10, Body: "unrelated comment"},
				{ID: 42, Body: "Coverage report\n" + CommentMarker + "\ndetails here"},
				{ID: 50, Body: "another comment"},
			},
			statusCode: http.StatusOK,
			wantID:     42,
		},
		{
			name: "returns zero when no comment has marker",
			comments: []issueComment{
				{ID: 10, Body: "unrelated comment"},
				{ID: 20, Body: "no marker here"},
			},
			statusCode: http.StatusOK,
			wantID:     0,
		},
		{
			name:       "returns zero for empty comment list",
			comments:   []issueComment{},
			statusCode: http.StatusOK,
			wantID:     0,
		},
		{
			name:       "returns error on API failure",
			statusCode: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/repos/owner/repo/issues/7/comments")

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.comments)
				} else {
					w.Write([]byte(`{"message":"forbidden"}`))
				}
			}))
			defer server.Close()

			client := NewClientWithHTTP("token", server.Client(), server.URL)
			id, err := client.FindCoverageComment(context.Background(), "owner", "repo", 7)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "GitHub API error")
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
		response   issueComment
		wantID     int64
		wantURL    string
		wantErr    bool
	}{
		{
			name:       "creates comment successfully",
			statusCode: http.StatusCreated,
			response:   issueComment{ID: 99, HTMLURL: "https://github.com/owner/repo/pull/5#issuecomment-99"},
			wantID:     99,
			wantURL:    "https://github.com/owner/repo/pull/5#issuecomment-99",
		},
		{
			name:       "returns error on API failure",
			statusCode: http.StatusUnprocessableEntity,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.URL.Path, "/repos/owner/repo/issues/5/comments")
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var payload map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				assert.Equal(t, "test body", payload["body"])

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusCreated {
					json.NewEncoder(w).Encode(tt.response)
				} else {
					w.Write([]byte(`{"message":"validation failed"}`))
				}
			}))
			defer server.Close()

			client := NewClientWithHTTP("token", server.Client(), server.URL)
			id, url, err := client.CreateComment(context.Background(), "owner", "repo", 5, "test body")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "GitHub API error")
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
	}{
		{
			name:       "updates comment successfully",
			statusCode: http.StatusOK,
		},
		{
			name:       "returns error on API failure",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				assert.Contains(t, r.URL.Path, "/repos/owner/repo/issues/comments/42")
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var payload map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				assert.Equal(t, "updated body", payload["body"])

				w.WriteHeader(tt.statusCode)
				if tt.statusCode != http.StatusOK {
					w.Write([]byte(`{"message":"not found"}`))
				}
			}))
			defer server.Close()

			client := NewClientWithHTTP("token", server.Client(), server.URL)
			err := client.UpdateComment(context.Background(), "owner", "repo", 42, "updated body")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "GitHub API error")
				return
			}

			require.NoError(t, err)
		})
	}
}
