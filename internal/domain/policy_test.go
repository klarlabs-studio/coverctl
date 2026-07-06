package domain

import "testing"

func TestEvaluatePolicy(t *testing.T) {
	min := 85.0
	policy := Policy{
		DefaultMin: 80,
		Domains: []Domain{
			{Name: "core", Min: &min},
			{Name: "api"},
		},
	}
	coverage := map[string]CoverageStat{
		"core": {Covered: 16, Total: 20},
		"api":  {Covered: 8, Total: 10},
	}

	result := Evaluate(policy, coverage)
	if result.Passed {
		t.Fatalf("expected policy to fail")
	}
	if got := result.Domains[0].Status; got != StatusFail {
		t.Fatalf("expected core to fail, got %s", got)
	}
	if got := result.Domains[1].Status; got != StatusPass {
		t.Fatalf("expected api to pass, got %s", got)
	}
}

// TestEvaluateRoundingDoesNotPassSubThreshold ensures the threshold comparison
// uses the raw percentage, not the display-rounded one: 79.95% rounds to 80.0
// for display but must still FAIL an 80% gate.
func TestEvaluateRoundingDoesNotPassSubThreshold(t *testing.T) {
	policy := Policy{
		DefaultMin: 80,
		Domains:    []Domain{{Name: "core"}},
	}
	// 7995/10000 = 79.95% (raw) which Round1 rounds up to 80.0.
	coverage := map[string]CoverageStat{"core": {Covered: 7995, Total: 10000}}

	result := Evaluate(policy, coverage)
	if result.Passed {
		t.Fatalf("expected 79.95%% to fail an 80%% gate, but it passed")
	}
	if got := result.Domains[0].Status; got != StatusFail {
		t.Fatalf("expected core to FAIL, got %s", got)
	}
	// Display value is still the rounded 80.0.
	if got := result.Domains[0].Percent; got != 80.0 {
		t.Fatalf("expected displayed percent 80.0, got %v", got)
	}

	// Exactly 80.0 must still pass.
	pass := Evaluate(policy, map[string]CoverageStat{"core": {Covered: 8000, Total: 10000}})
	if !pass.Passed {
		t.Fatalf("expected exactly 80.0%% to pass an 80%% gate")
	}
}

// TestPolicyAggregateRoundingDoesNotPassSubThreshold mirrors the above for the
// aggregate evaluation path.
func TestPolicyAggregateRoundingDoesNotPassSubThreshold(t *testing.T) {
	policy := Policy{DefaultMin: 80, Domains: []Domain{{Name: "core"}}}
	agg, err := NewPolicyAggregate(policy)
	if err != nil {
		t.Fatalf("new aggregate: %v", err)
	}
	res := agg.Evaluate(map[string]CoverageStat{"core": {Covered: 7995, Total: 10000}})
	if res.Passed {
		t.Fatalf("expected aggregate 79.95%% to fail an 80%% gate")
	}
	if res.DomainResults[0].Status != StatusFail {
		t.Fatalf("expected core to FAIL, got %s", res.DomainResults[0].Status)
	}
}

func TestCoveragePercent(t *testing.T) {
	stat := CoverageStat{Covered: 1, Total: 3}
	if got := stat.Percent(); got < 33.3 || got > 33.4 {
		t.Fatalf("expected ~33.3, got %f", got)
	}
	zero := CoverageStat{}
	if got := zero.Percent(); got != 0 {
		t.Fatalf("expected 0, got %f", got)
	}
}

func TestEvaluateWarnThreshold(t *testing.T) {
	min := 80.0
	warn := 90.0
	policy := Policy{
		DefaultMin: 75,
		Domains: []Domain{
			{Name: "core", Min: &min, Warn: &warn},
		},
	}
	// Coverage is above min but below warn
	coverage := map[string]CoverageStat{
		"core": {Covered: 85, Total: 100},
	}

	result := Evaluate(policy, coverage)
	if !result.Passed {
		t.Fatal("expected policy to pass (above min)")
	}
	if result.Domains[0].Status != StatusWarn {
		t.Fatalf("expected warn status, got %s", result.Domains[0].Status)
	}
}

func TestEvaluateWarnThresholdPassAboveWarn(t *testing.T) {
	min := 80.0
	warn := 85.0
	policy := Policy{
		DefaultMin: 75,
		Domains: []Domain{
			{Name: "core", Min: &min, Warn: &warn},
		},
	}
	// Coverage is above both min and warn
	coverage := map[string]CoverageStat{
		"core": {Covered: 90, Total: 100},
	}

	result := Evaluate(policy, coverage)
	if !result.Passed {
		t.Fatal("expected policy to pass")
	}
	if result.Domains[0].Status != StatusPass {
		t.Fatalf("expected pass status, got %s", result.Domains[0].Status)
	}
}

func TestEvaluateWarnThresholdFailBelowMin(t *testing.T) {
	min := 80.0
	warn := 90.0
	policy := Policy{
		DefaultMin: 75,
		Domains: []Domain{
			{Name: "core", Min: &min, Warn: &warn},
		},
	}
	// Coverage is below min
	coverage := map[string]CoverageStat{
		"core": {Covered: 70, Total: 100},
	}

	result := Evaluate(policy, coverage)
	if result.Passed {
		t.Fatal("expected policy to fail (below min)")
	}
	if result.Domains[0].Status != StatusFail {
		t.Fatalf("expected fail status, got %s", result.Domains[0].Status)
	}
}

func TestEvaluateNoWarnThreshold(t *testing.T) {
	min := 80.0
	policy := Policy{
		DefaultMin: 75,
		Domains: []Domain{
			{Name: "core", Min: &min}, // No warn set
		},
	}
	// Coverage is above min - should just pass without warn
	coverage := map[string]CoverageStat{
		"core": {Covered: 85, Total: 100},
	}

	result := Evaluate(policy, coverage)
	if !result.Passed {
		t.Fatal("expected policy to pass")
	}
	if result.Domains[0].Status != StatusPass {
		t.Fatalf("expected pass status, got %s", result.Domains[0].Status)
	}
}

func TestCoverageStatBehavior(t *testing.T) {
	t.Run("PercentRounded", func(t *testing.T) {
		stat := CoverageStat{Covered: 1, Total: 3}
		got := stat.PercentRounded()
		if got != 33.3 {
			t.Errorf("PercentRounded() = %f, want 33.3", got)
		}
	})

	t.Run("Uncovered", func(t *testing.T) {
		stat := CoverageStat{Covered: 7, Total: 10}
		if got := stat.Uncovered(); got != 3 {
			t.Errorf("Uncovered() = %d, want 3", got)
		}
	})

	t.Run("IsEmpty", func(t *testing.T) {
		empty := CoverageStat{}
		if !empty.IsEmpty() {
			t.Error("IsEmpty() = false, want true")
		}
		nonEmpty := CoverageStat{Covered: 1, Total: 1}
		if nonEmpty.IsEmpty() {
			t.Error("IsEmpty() = true, want false")
		}
	})
}

func TestDomainBehavior(t *testing.T) {
	t.Run("MinThreshold with explicit min", func(t *testing.T) {
		min := 85.0
		d := Domain{Name: "core", Min: &min}
		if got := d.MinThreshold(70); got != 85 {
			t.Errorf("MinThreshold() = %f, want 85", got)
		}
	})

	t.Run("MinThreshold without explicit min", func(t *testing.T) {
		d := Domain{Name: "api"}
		if got := d.MinThreshold(70); got != 70 {
			t.Errorf("MinThreshold() = %f, want 70 (default)", got)
		}
	})

	t.Run("HasWarnThreshold", func(t *testing.T) {
		warn := 90.0
		withWarn := Domain{Name: "core", Warn: &warn}
		if !withWarn.HasWarnThreshold() {
			t.Error("HasWarnThreshold() = false, want true")
		}
		withoutWarn := Domain{Name: "api"}
		if withoutWarn.HasWarnThreshold() {
			t.Error("HasWarnThreshold() = true, want false")
		}
	})
}

func TestDomainResultBehavior(t *testing.T) {
	passing := DomainResult{Domain: "core", Percent: 85, Required: 80, Status: StatusPass}
	failing := DomainResult{Domain: "api", Percent: 70, Required: 80, Status: StatusFail}
	warning := DomainResult{Domain: "util", Percent: 85, Required: 80, Status: StatusWarn}

	t.Run("IsPassing", func(t *testing.T) {
		if !passing.IsPassing() {
			t.Error("IsPassing() = false, want true")
		}
		if failing.IsPassing() {
			t.Error("IsPassing() = true, want false")
		}
	})

	t.Run("IsFailing", func(t *testing.T) {
		if !failing.IsFailing() {
			t.Error("IsFailing() = false, want true")
		}
		if passing.IsFailing() {
			t.Error("IsFailing() = true, want false")
		}
	})

	t.Run("IsWarning", func(t *testing.T) {
		if !warning.IsWarning() {
			t.Error("IsWarning() = false, want true")
		}
		if passing.IsWarning() {
			t.Error("IsWarning() = true, want false")
		}
	})

	t.Run("Shortfall", func(t *testing.T) {
		if got := failing.Shortfall(); got != 10 {
			t.Errorf("Shortfall() = %f, want 10", got)
		}
		if got := passing.Shortfall(); got != 0 {
			t.Errorf("Shortfall() = %f, want 0", got)
		}
	})

	t.Run("Stat", func(t *testing.T) {
		dr := DomainResult{Covered: 80, Total: 100}
		stat := dr.Stat()
		if stat.Covered != 80 || stat.Total != 100 {
			t.Errorf("Stat() = %+v, want {Covered:80 Total:100}", stat)
		}
	})
}

func TestResultBehavior(t *testing.T) {
	result := Result{
		Domains: []DomainResult{
			{Domain: "core", Covered: 80, Total: 100, Status: StatusPass},
			{Domain: "api", Covered: 60, Total: 100, Status: StatusFail},
			{Domain: "util", Covered: 85, Total: 100, Status: StatusWarn},
		},
		Passed:   false,
		Warnings: []string{"test warning"},
	}

	t.Run("OverallPercent", func(t *testing.T) {
		// (80+60+85) / (100+100+100) = 225/300 = 75%
		if got := result.OverallPercent(); got != 75 {
			t.Errorf("OverallPercent() = %f, want 75", got)
		}
	})

	t.Run("PassingDomainCount", func(t *testing.T) {
		if got := result.PassingDomainCount(); got != 1 {
			t.Errorf("PassingDomainCount() = %d, want 1", got)
		}
	})

	t.Run("FailingDomainCount", func(t *testing.T) {
		if got := result.FailingDomainCount(); got != 1 {
			t.Errorf("FailingDomainCount() = %d, want 1", got)
		}
	})

	t.Run("WarningDomainCount", func(t *testing.T) {
		if got := result.WarningDomainCount(); got != 1 {
			t.Errorf("WarningDomainCount() = %d, want 1", got)
		}
	})

	t.Run("TotalStatements", func(t *testing.T) {
		if got := result.TotalStatements(); got != 300 {
			t.Errorf("TotalStatements() = %d, want 300", got)
		}
	})

	t.Run("TotalCovered", func(t *testing.T) {
		if got := result.TotalCovered(); got != 225 {
			t.Errorf("TotalCovered() = %d, want 225", got)
		}
	})

	t.Run("HasWarnings", func(t *testing.T) {
		if !result.HasWarnings() {
			t.Error("HasWarnings() = false, want true")
		}
		noWarnings := Result{}
		if noWarnings.HasWarnings() {
			t.Error("HasWarnings() = true, want false")
		}
	})
}

func TestRound1(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{33.333, 33.3},
		{33.35, 33.4},
		{0, 0},
		{100, 100},
		{99.99, 100},
		{75.55, 75.6},
	}
	for _, tt := range tests {
		if got := Round1(tt.input); got != tt.want {
			t.Errorf("Round1(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}
