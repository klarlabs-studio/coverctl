package application

import (
	"errors"
	"testing"

	"go.klarlabs.de/coverctl/internal/domain"
)

// fakeHistoryStore is a minimal HistoryStore for exercising the ratchet gate.
type fakeHistoryStore struct {
	hist    domain.History
	loadErr error
}

func (f *fakeHistoryStore) Load() (domain.History, error)    { return f.hist, f.loadErr }
func (f *fakeHistoryStore) Save(domain.History) error        { return nil }
func (f *fakeHistoryStore) Append(domain.HistoryEntry) error { return nil }

// resultAt builds a Result whose OverallPercent is the given percentage.
func resultAt(percent float64) domain.Result {
	return domain.Result{
		Passed: true,
		Domains: []domain.DomainResult{
			{Domain: "core", Covered: int(percent * 10), Total: 1000},
		},
	}
}

func ptr(f float64) *float64 { return &f }

func TestEnforceExtraGates_FailUnder(t *testing.T) {
	svc := &Service{}
	// 60% overall, floor of 90 → violation.
	err := svc.EnforceExtraGates(resultAt(60), CheckOptions{FailUnder: ptr(90)})
	if err == nil {
		t.Fatal("expected fail-under violation, got nil")
	}
	// 95% overall, floor of 90 → ok.
	if err := svc.EnforceExtraGates(resultAt(95), CheckOptions{FailUnder: ptr(90)}); err != nil {
		t.Fatalf("expected no violation at 95%% vs 90 floor, got %v", err)
	}
}

func TestEnforceExtraGates_RatchetRegression(t *testing.T) {
	svc := &Service{}
	store := &fakeHistoryStore{hist: domain.History{Entries: []domain.HistoryEntry{{Overall: 95}}}}
	// Current 60% < previous 95% → regression.
	err := svc.EnforceExtraGates(resultAt(60), CheckOptions{Ratchet: true, HistoryStore: store})
	if err == nil {
		t.Fatal("expected ratchet regression violation, got nil")
	}
	// Current 95% == previous 95% → ok (no decrease).
	if err := svc.EnforceExtraGates(resultAt(95), CheckOptions{Ratchet: true, HistoryStore: store}); err != nil {
		t.Fatalf("expected no violation when coverage holds, got %v", err)
	}
}

// TestEnforceExtraGates_RatchetLoadErrorFailsClosed ensures a corrupt/unreadable
// baseline does not silently pass the ratchet gate.
func TestEnforceExtraGates_RatchetLoadErrorFailsClosed(t *testing.T) {
	svc := &Service{}
	store := &fakeHistoryStore{loadErr: errors.New("corrupt history file")}
	err := svc.EnforceExtraGates(resultAt(95), CheckOptions{Ratchet: true, HistoryStore: store})
	if err == nil {
		t.Fatal("expected ratchet to fail closed on a history-load error, got nil")
	}
}

// TestEnforceExtraGates_RatchetNoBaselinePasses documents that a first run with
// no recorded history passes (there is nothing to regress from).
func TestEnforceExtraGates_RatchetNoBaselinePasses(t *testing.T) {
	svc := &Service{}
	store := &fakeHistoryStore{hist: domain.History{}}
	if err := svc.EnforceExtraGates(resultAt(50), CheckOptions{Ratchet: true, HistoryStore: store}); err != nil {
		t.Fatalf("expected no violation with an empty baseline, got %v", err)
	}
}
