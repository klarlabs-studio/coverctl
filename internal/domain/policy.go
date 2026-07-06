package domain

import "math"

// CoverageStat summarizes covered vs total statements.
type CoverageStat struct {
	Covered int
	Total   int
}

// Percent returns the coverage percentage as a raw float64.
func (c CoverageStat) Percent() float64 {
	if c.Total == 0 {
		return 0
	}
	return (float64(c.Covered) / float64(c.Total)) * 100
}

// PercentRounded returns the coverage percentage rounded to one decimal place.
func (c CoverageStat) PercentRounded() float64 {
	return Round1(c.Percent())
}

// Uncovered returns the number of uncovered statements.
func (c CoverageStat) Uncovered() int {
	return c.Total - c.Covered
}

// IsEmpty returns true if there are no statements to cover.
func (c CoverageStat) IsEmpty() bool {
	return c.Total == 0
}

// Domain defines a named coverage scope and its policy.
type Domain struct {
	Name    string
	Match   []string
	Min     *float64
	Warn    *float64 // Optional warn threshold (must be >= Min)
	Exclude []string // Optional patterns to exclude from this domain
}

// MinThreshold returns the minimum coverage threshold for this domain,
// falling back to the provided default if not explicitly set.
func (d Domain) MinThreshold(defaultMin float64) float64 {
	if d.Min != nil {
		return *d.Min
	}
	return defaultMin
}

// HasWarnThreshold returns true if a warning threshold is configured.
func (d Domain) HasWarnThreshold() bool {
	return d.Warn != nil
}

// Policy defines default and domain-specific coverage requirements.
type Policy struct {
	DefaultMin float64
	Domains    []Domain
}

type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusWarn Status = "WARN"
)

type DomainResult struct {
	Domain   string   `json:"domain"`
	Covered  int      `json:"covered"`
	Total    int      `json:"total"`
	Percent  float64  `json:"percent"`
	Required float64  `json:"required"`
	Status   Status   `json:"status"`
	Delta    *float64 `json:"delta,omitempty"` // Change from previous run
}

// IsPassing returns true if this domain meets its coverage requirement.
func (d DomainResult) IsPassing() bool {
	return d.Status == StatusPass
}

// IsFailing returns true if this domain fails its coverage requirement.
func (d DomainResult) IsFailing() bool {
	return d.Status == StatusFail
}

// IsWarning returns true if this domain is above min but below warn threshold.
func (d DomainResult) IsWarning() bool {
	return d.Status == StatusWarn
}

// Shortfall returns how many percentage points below the requirement this domain is.
// Returns 0 if the domain is passing.
func (d DomainResult) Shortfall() float64 {
	if d.Percent >= d.Required {
		return 0
	}
	return Round1(d.Required - d.Percent)
}

// Stat returns the coverage statistics for this domain result.
func (d DomainResult) Stat() CoverageStat {
	return CoverageStat{Covered: d.Covered, Total: d.Total}
}

type FileRule struct {
	Match []string
	Min   float64
}

type FileResult struct {
	File     string  `json:"file"`
	Covered  int     `json:"covered"`
	Total    int     `json:"total"`
	Percent  float64 `json:"percent"`
	Required float64 `json:"required"`
	Status   Status  `json:"status"`
}

// IsPassing returns true if this file meets its coverage requirement.
func (f FileResult) IsPassing() bool {
	return f.Status == StatusPass
}

// IsFailing returns true if this file fails its coverage requirement.
func (f FileResult) IsFailing() bool {
	return f.Status == StatusFail
}

// Shortfall returns how many percentage points below the requirement this file is.
// Returns 0 if the file is passing.
func (f FileResult) Shortfall() float64 {
	if f.Percent >= f.Required {
		return 0
	}
	return Round1(f.Required - f.Percent)
}

// Stat returns the coverage statistics for this file result.
func (f FileResult) Stat() CoverageStat {
	return CoverageStat{Covered: f.Covered, Total: f.Total}
}

type Result struct {
	Domains  []DomainResult `json:"domains"`
	Files    []FileResult   `json:"files,omitempty"`
	Passed   bool           `json:"passed"`
	Warnings []string       `json:"warnings,omitempty"`
}

// OverallPercent calculates the overall coverage percentage across all domains.
func (r Result) OverallPercent() float64 {
	var totalCovered, totalStatements int
	for _, d := range r.Domains {
		totalCovered += d.Covered
		totalStatements += d.Total
	}
	if totalStatements == 0 {
		return 0
	}
	return Round1(float64(totalCovered) / float64(totalStatements) * 100)
}

// PassingDomainCount returns the number of domains that are passing.
func (r Result) PassingDomainCount() int {
	count := 0
	for _, d := range r.Domains {
		if d.IsPassing() {
			count++
		}
	}
	return count
}

// FailingDomainCount returns the number of domains that are failing.
func (r Result) FailingDomainCount() int {
	count := 0
	for _, d := range r.Domains {
		if d.IsFailing() {
			count++
		}
	}
	return count
}

// WarningDomainCount returns the number of domains with warnings.
func (r Result) WarningDomainCount() int {
	count := 0
	for _, d := range r.Domains {
		if d.IsWarning() {
			count++
		}
	}
	return count
}

// TotalStatements returns the total number of statements across all domains.
func (r Result) TotalStatements() int {
	total := 0
	for _, d := range r.Domains {
		total += d.Total
	}
	return total
}

// TotalCovered returns the total number of covered statements across all domains.
func (r Result) TotalCovered() int {
	total := 0
	for _, d := range r.Domains {
		total += d.Covered
	}
	return total
}

// HasWarnings returns true if there are any warnings.
func (r Result) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// ApplyDeltas calculates and sets delta values for each domain based on history.
// This modifies the Result in place, computing the change from the latest history entry.
func (r *Result) ApplyDeltas(history History) {
	if len(history.Entries) == 0 {
		return
	}
	latest := history.LatestEntry()
	if latest == nil {
		return
	}

	for i := range r.Domains {
		domainName := r.Domains[i].Domain
		if prevEntry, ok := latest.Domains[domainName]; ok {
			delta := Round1(r.Domains[i].Percent - prevEntry.Percent)
			r.Domains[i].Delta = &delta
		}
	}
}

// WithDeltas returns a copy of the Result with deltas applied from history.
// This is a pure function that doesn't modify the original Result.
func (r Result) WithDeltas(history History) Result {
	result := r
	result.Domains = make([]DomainResult, len(r.Domains))
	copy(result.Domains, r.Domains)
	result.ApplyDeltas(history)
	return result
}

// DomainByName returns the domain result with the given name, or nil if not found.
func (r Result) DomainByName(name string) *DomainResult {
	for i := range r.Domains {
		if r.Domains[i].Domain == name {
			return &r.Domains[i]
		}
	}
	return nil
}

// FailingDomains returns all domains that are failing.
func (r Result) FailingDomains() []DomainResult {
	var failing []DomainResult
	for _, d := range r.Domains {
		if d.IsFailing() {
			failing = append(failing, d)
		}
	}
	return failing
}

// WarningDomains returns all domains with warnings.
func (r Result) WarningDomains() []DomainResult {
	var warnings []DomainResult
	for _, d := range r.Domains {
		if d.IsWarning() {
			warnings = append(warnings, d)
		}
	}
	return warnings
}

// PassingDomains returns all domains that are passing.
func (r Result) PassingDomains() []DomainResult {
	var passing []DomainResult
	for _, d := range r.Domains {
		if d.IsPassing() {
			passing = append(passing, d)
		}
	}
	return passing
}

// Summary returns a brief summary of the result.
func (r Result) Summary() string {
	if r.Passed {
		return "All coverage thresholds met"
	}
	return "Coverage thresholds not met"
}

func Evaluate(policy Policy, coverage map[string]CoverageStat) Result {
	results := make([]DomainResult, 0, len(policy.Domains))
	passed := true

	for _, d := range policy.Domains {
		stat := coverage[d.Name]
		required := policy.DefaultMin
		if d.Min != nil {
			required = *d.Min
		}
		// Compare the raw percentage against the threshold so that a value just
		// under the bound (e.g. 79.95%) cannot round up to 80.0 and pass an 80%
		// gate. The rounded value is used only for display/reporting.
		rawPercent := stat.Percent()
		percent := Round1(rawPercent)
		status := StatusPass
		if rawPercent < required {
			status = StatusFail
			passed = false
		} else if d.Warn != nil && rawPercent < *d.Warn {
			// Above min but below warn threshold
			status = StatusWarn
		}
		results = append(results, DomainResult{
			Domain:   d.Name,
			Covered:  stat.Covered,
			Total:    stat.Total,
			Percent:  percent,
			Required: required,
			Status:   status,
		})
	}

	return Result{Domains: results, Passed: passed}
}

// Round1 rounds a float64 to one decimal place.
// This is the standard rounding function used for coverage percentages.
func Round1(v float64) float64 {
	return math.Round(v*10) / 10
}
