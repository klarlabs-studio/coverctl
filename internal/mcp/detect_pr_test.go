package mcp

import (
	"context"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestDetectPRContextMCP_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "acme/widgets")
	t.Setenv("CI_PROJECT_NAMESPACE", "")
	t.Setenv("CI_PROJECT_NAME", "")
	t.Setenv("CI_MERGE_REQUEST_IID", "")
	t.Setenv("BITBUCKET_WORKSPACE", "")
	t.Setenv("BITBUCKET_REPO_SLUG", "")
	t.Setenv("BITBUCKET_PR_ID", "")

	o, r, _ := detectPRContextMCP(application.ProviderGitHub, "", "", 0)
	if o != "acme" || r != "widgets" {
		t.Fatalf("github repo parse: %s/%s", o, r)
	}

	t.Setenv("CI_PROJECT_NAMESPACE", "group")
	t.Setenv("CI_PROJECT_NAME", "proj")
	t.Setenv("CI_MERGE_REQUEST_IID", "42")
	o, r, n := detectPRContextMCP(application.ProviderGitLab, "", "", 0)
	if o != "group" || r != "proj" || n != 42 {
		t.Fatalf("gitlab: %s/%s#%d", o, r, n)
	}

	t.Setenv("BITBUCKET_WORKSPACE", "ws")
	t.Setenv("BITBUCKET_REPO_SLUG", "repo")
	t.Setenv("BITBUCKET_PR_ID", "9")
	o, r, n = detectPRContextMCP(application.ProviderBitbucket, "", "", 0)
	if o != "ws" || r != "repo" || n != 9 {
		t.Fatalf("bitbucket: %s/%s#%d", o, r, n)
	}
}

func TestHandleCompare_NegativeDeltaSummary(t *testing.T) {
	svc := &mockService{
		compareResult: application.CompareResult{
			BaseOverall: 90,
			HeadOverall: 80,
			Delta:       -10,
			Regressed:   []application.FileDelta{{File: "x.go", Delta: -10}},
		},
	}
	s := New(svc, DefaultConfig(), "test")
	out, err := s.handleCompare(context.Background(), CompareInput{BaseProfile: "b.out"})
	if err != nil {
		t.Fatal(err)
	}
	sum, _ := out["summary"].(string)
	if sum == "" {
		t.Fatalf("%#v", out)
	}
}
