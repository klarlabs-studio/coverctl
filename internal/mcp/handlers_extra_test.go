package mcp

import (
	"context"
	"testing"

	"go.klarlabs.de/coverctl/internal/application"
)

func TestHandleCompare_Badge_PRComment(t *testing.T) {
	svc := &mockService{
		compareResult: application.CompareResult{
			BaseOverall: 80,
			HeadOverall: 85,
			Delta:       5,
			Improved: []application.FileDelta{
				{File: "a.go", Delta: 2},
				{File: "b.go", Delta: 1},
			},
			Regressed: []application.FileDelta{{File: "c.go", Delta: -1}},
		},
		badgeResult: application.BadgeResult{Percent: 85},
	}
	// PRComment mock returns empty success; force dry-run path.
	s := New(svc, DefaultConfig(), "test")

	cmp, err := s.handleCompare(context.Background(), CompareInput{
		BaseProfile: "base.out",
		HeadProfile: "head.out",
		Verbosity:   "brief",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmp["passed"] != true {
		t.Fatalf("compare: %#v", cmp)
	}

	badge, err := s.handleBadge(context.Background(), BadgeInput{Label: "cov"})
	if err != nil {
		t.Fatal(err)
	}
	if badge["passed"] != true {
		t.Fatalf("badge: %#v", badge)
	}

	pr, err := s.handlePRComment(context.Background(), PRCommentInput{
		Provider: "github",
		Owner:    "o",
		Repo:     "r",
		PRNumber: 7,
		DryRun:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pr["passed"] != true {
		t.Fatalf("pr-comment: %#v", pr)
	}
}

func TestHandleCompare_MissingBase(t *testing.T) {
	s := New(&mockService{}, DefaultConfig(), "test")
	out, err := s.handleCompare(context.Background(), CompareInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out["error_code"] == nil && out["passed"] != false {
		t.Fatalf("expected missing-arg failure: %#v", out)
	}
}

func TestHandlePRComment_InvalidProvider(t *testing.T) {
	s := New(&mockService{}, DefaultConfig(), "test")
	out, err := s.handlePRComment(context.Background(), PRCommentInput{Provider: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if out["passed"] != false {
		t.Fatalf("%#v", out)
	}
}

func TestApplyFileDeltaBudget(t *testing.T) {
	ds := make([]application.FileDelta, normalRowCap+3)
	for i := range ds {
		ds[i] = application.FileDelta{File: "f.go", Delta: float64(i)}
	}
	got, cursor := applyFileDeltaBudget(ds, VerbosityNormal)
	if len(got) != normalRowCap || cursor == "" {
		t.Fatalf("len=%d cursor=%q", len(got), cursor)
	}
	all, empty := applyFileDeltaBudget(ds[:2], VerbosityVerbose)
	if len(all) != 2 || empty != "" {
		t.Fatalf("verbose: len=%d cursor=%q", len(all), empty)
	}
}

func TestDetectPRContextMCP_Passthrough(t *testing.T) {
	o, r, n := detectPRContextMCP(application.ProviderGitHub, "a", "b", 3)
	if o != "a" || r != "b" || n != 3 {
		t.Fatalf("%s %s %d", o, r, n)
	}
}
