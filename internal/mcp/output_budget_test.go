package mcp

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/coverctl/internal/application"
	"github.com/felixgeelhaar/coverctl/internal/domain"
)

func makeDomains(n int, failingAt ...int) []domain.DomainResult {
	out := make([]domain.DomainResult, n)
	failing := map[int]bool{}
	for _, i := range failingAt {
		failing[i] = true
	}
	for i := 0; i < n; i++ {
		status := domain.StatusPass
		if failing[i] {
			status = domain.StatusFail
		}
		out[i] = domain.DomainResult{
			Domain: "d" + itoa(i),
			Status: status,
		}
	}
	return out
}

func TestApplyDomainBudget_VerboseUntouched(t *testing.T) {
	in := makeDomains(50, 0, 1, 2)
	out, cursor := applyDomainBudget(in, VerbosityVerbose)
	if len(out) != 50 {
		t.Errorf("verbose should not truncate, got len %d", len(out))
	}
	if cursor != "" {
		t.Errorf("verbose should not return cursor, got %q", cursor)
	}
}

func TestApplyDomainBudget_BriefKeepsOnlyFailing(t *testing.T) {
	in := makeDomains(20, 0, 5, 10)
	out, cursor := applyDomainBudget(in, VerbosityBrief)
	if len(out) != 3 {
		t.Errorf("brief should keep 3 failing, got %d", len(out))
	}
	for _, r := range out {
		if r.Status != domain.StatusFail {
			t.Errorf("brief returned non-failing row: %+v", r)
		}
	}
	if cursor != "" {
		t.Errorf("no cursor expected when failing fits brief cap, got %q", cursor)
	}
}

func TestApplyDomainBudget_BriefCapsAtBriefRowCap(t *testing.T) {
	failing := []int{0, 1, 2, 3, 4, 5, 6}
	in := makeDomains(20, failing...)
	out, cursor := applyDomainBudget(in, VerbosityBrief)
	if len(out) != briefRowCap {
		t.Errorf("brief should cap at %d, got %d", briefRowCap, len(out))
	}
	if !strings.HasPrefix(cursor, "next/") {
		t.Errorf("expected pagination cursor, got %q", cursor)
	}
}

func TestApplyDomainBudget_NormalKeepsAllFailingAndTrimsPassing(t *testing.T) {
	failing := []int{0, 1, 2, 3, 4}
	in := makeDomains(40, failing...)
	out, cursor := applyDomainBudget(in, VerbosityNormal)
	if len(out) != normalRowCap {
		t.Errorf("normal should cap at %d, got %d", normalRowCap, len(out))
	}
	failingCount := 0
	for _, r := range out {
		if r.Status == domain.StatusFail {
			failingCount++
		}
	}
	if failingCount != len(failing) {
		t.Errorf("normal should preserve all %d failing rows, got %d",
			len(failing), failingCount)
	}
	if cursor == "" {
		t.Error("expected pagination cursor when input exceeds normal cap")
	}
}

func TestApplyDomainBudget_NormalUndercapNoCursor(t *testing.T) {
	in := makeDomains(5, 0)
	out, cursor := applyDomainBudget(in, VerbosityNormal)
	if len(out) != 5 {
		t.Errorf("normal under cap should return all rows, got %d", len(out))
	}
	if cursor != "" {
		t.Errorf("no cursor expected under cap, got %q", cursor)
	}
}

func TestResolveVerbosity_DefaultsToNormal(t *testing.T) {
	cases := []struct {
		in   string
		want Verbosity
	}{
		{"", VerbosityNormal},
		{"normal", VerbosityNormal},
		{"brief", VerbosityBrief},
		{"verbose", VerbosityVerbose},
		{"unrecognized", VerbosityNormal},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := resolveVerbosity(c.in); got != c.want {
				t.Errorf("resolveVerbosity(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

func TestApplyDebtItemBudget(t *testing.T) {
	in := make([]application.DebtItem, 30)
	for i := range in {
		in[i] = application.DebtItem{Name: "f" + itoa(i)}
	}
	out, cursor := applyDebtItemBudget(in, VerbosityBrief)
	if len(out) != briefRowCap {
		t.Errorf("brief debt cap %d, got %d", briefRowCap, len(out))
	}
	if cursor == "" {
		t.Error("expected cursor when truncating debt items")
	}

	// verbose: untouched
	out2, cursor2 := applyDebtItemBudget(in, VerbosityVerbose)
	if len(out2) != 30 || cursor2 != "" {
		t.Errorf("verbose should not truncate; got len=%d cursor=%q", len(out2), cursor2)
	}
}

func TestCursorFor(t *testing.T) {
	if got := cursorFor(5, 5); got != "" {
		t.Errorf("cursor when taken==total should be empty, got %q", got)
	}
	if got := cursorFor(5, 20); got != "next/5/of/20" {
		t.Errorf("unexpected cursor format: %q", got)
	}
}
