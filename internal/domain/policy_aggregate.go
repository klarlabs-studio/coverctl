package domain

// PolicyAggregate is the aggregate root for coverage policy evaluation.
// It encapsulates the business logic for evaluating coverage against thresholds
// and raises domain events when significant changes occur.
type PolicyAggregate struct {
	id         string
	defaultMin Threshold
	domains    []DomainSpec
	events     *EventCollector
}

// DomainSpec represents a domain specification within a policy.
// It is part of the PolicyAggregate and defines coverage requirements for a specific domain.
type DomainSpec struct {
	Name      DomainName
	Match     []string
	MinValue  Threshold
	WarnValue *Threshold
	Exclude   []string
}

// NewPolicyAggregate creates a new PolicyAggregate from a Policy.
func NewPolicyAggregate(policy Policy) (*PolicyAggregate, error) {
	defaultMin, err := NewThreshold(policy.DefaultMin)
	if err != nil {
		return nil, err
	}

	specs := make([]DomainSpec, 0, len(policy.Domains))
	for _, d := range policy.Domains {
		spec, err := newDomainSpecFromDomain(d, defaultMin)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	return &PolicyAggregate{
		id:         "default",
		defaultMin: defaultMin,
		domains:    specs,
		events:     NewEventCollector(),
	}, nil
}

// newDomainSpecFromDomain converts a Domain to a DomainSpec.
func newDomainSpecFromDomain(d Domain, defaultMin Threshold) (DomainSpec, error) {
	name, err := NewDomainName(d.Name)
	if err != nil {
		return DomainSpec{}, err
	}

	minVal := defaultMin
	if d.Min != nil {
		minVal, err = NewThreshold(*d.Min)
		if err != nil {
			return DomainSpec{}, err
		}
	}

	var warnVal *Threshold
	if d.Warn != nil {
		t, err := NewThreshold(*d.Warn)
		if err != nil {
			return DomainSpec{}, err
		}
		warnVal = &t
	}

	return DomainSpec{
		Name:      name,
		Match:     d.Match,
		MinValue:  minVal,
		WarnValue: warnVal,
		Exclude:   d.Exclude,
	}, nil
}

// EvaluationResult represents the result of evaluating coverage against a policy.
type EvaluationResult struct {
	DomainResults []DomainEvaluationResult
	Passed        bool
	Warnings      []string
}

// DomainEvaluationResult represents the evaluation result for a single domain.
type DomainEvaluationResult struct {
	Name      DomainName
	Stat      CoverageStat
	Percent   Percentage
	Required  Threshold
	Status    Status
	Shortfall float64
}

// OverallPercent calculates the overall coverage percentage.
func (r EvaluationResult) OverallPercent() Percentage {
	var totalCovered, totalStatements int
	for _, d := range r.DomainResults {
		totalCovered += d.Stat.Covered
		totalStatements += d.Stat.Total
	}
	return PercentageFromRatio(totalCovered, totalStatements)
}

// FailingCount returns the number of failing domains.
func (r EvaluationResult) FailingCount() int {
	count := 0
	for _, d := range r.DomainResults {
		if d.Status == StatusFail {
			count++
		}
	}
	return count
}

// PassingCount returns the number of passing domains.
func (r EvaluationResult) PassingCount() int {
	count := 0
	for _, d := range r.DomainResults {
		if d.Status == StatusPass {
			count++
		}
	}
	return count
}

// Evaluate evaluates coverage data against this policy's thresholds.
// It records domain events for the evaluation and any threshold violations.
func (p *PolicyAggregate) Evaluate(coverage map[string]CoverageStat) EvaluationResult {
	results := make([]DomainEvaluationResult, 0, len(p.domains))
	passed := true

	for _, spec := range p.domains {
		stat := coverage[spec.Name.String()]
		// Compare the raw percentage against thresholds; the rounded Percentage
		// is retained only for display (see determineStatus).
		rawPercent := stat.Percent()
		percent := NewPercentage(rawPercent)
		required := spec.MinValue

		status := p.determineStatus(rawPercent, spec)
		if status == StatusFail {
			passed = false
			// Record threshold violation event
			p.events.Record(NewThresholdViolatedEvent(
				spec.Name.String(),
				percent.Value(),
				required.Value(),
			))
		}

		shortfall := required.Shortfall(percent.Value())

		results = append(results, DomainEvaluationResult{
			Name:      spec.Name,
			Stat:      stat,
			Percent:   percent,
			Required:  required,
			Status:    status,
			Shortfall: shortfall,
		})
	}

	result := EvaluationResult{
		DomainResults: results,
		Passed:        passed,
	}

	// Record evaluation event
	p.events.Record(NewCoverageEvaluatedEvent(
		p.id,
		result.OverallPercent().Value(),
		passed,
		len(results),
		result.FailingCount(),
	))

	return result
}

// determineStatus determines the coverage status for a domain from the raw
// (unrounded) percentage, so a value just below a threshold cannot round up
// and pass.
func (p *PolicyAggregate) determineStatus(rawPercent float64, spec DomainSpec) Status {
	if !spec.MinValue.IsMet(rawPercent) {
		return StatusFail
	}
	if spec.WarnValue != nil && !spec.WarnValue.IsMet(rawPercent) {
		return StatusWarn
	}
	return StatusPass
}

// Events returns all domain events that were recorded.
func (p *PolicyAggregate) Events() []DomainEvent {
	return p.events.Events()
}

// ClearEvents clears all recorded events.
func (p *PolicyAggregate) ClearEvents() {
	p.events.Clear()
}

// DefaultMin returns the default minimum threshold.
func (p *PolicyAggregate) DefaultMin() Threshold {
	return p.defaultMin
}

// DomainSpecs returns the domain specifications.
func (p *PolicyAggregate) DomainSpecs() []DomainSpec {
	return p.domains
}

// ToResult converts an EvaluationResult to the legacy Result type.
// This provides backward compatibility with existing code.
func (r EvaluationResult) ToResult() Result {
	domains := make([]DomainResult, 0, len(r.DomainResults))
	for _, d := range r.DomainResults {
		domains = append(domains, DomainResult{
			Domain:   d.Name.String(),
			Covered:  d.Stat.Covered,
			Total:    d.Stat.Total,
			Percent:  d.Percent.Value(),
			Required: d.Required.Value(),
			Status:   d.Status,
		})
	}
	return Result{
		Domains:  domains,
		Passed:   r.Passed,
		Warnings: r.Warnings,
	}
}

// EvaluateWithAggregate is a helper that creates a PolicyAggregate and evaluates coverage.
// It provides a simple API for one-off evaluations.
func EvaluateWithAggregate(policy Policy, coverage map[string]CoverageStat) (Result, []DomainEvent, error) {
	agg, err := NewPolicyAggregate(policy)
	if err != nil {
		return Result{}, nil, err
	}

	evalResult := agg.Evaluate(coverage)
	events := agg.Events()

	return evalResult.ToResult(), events, nil
}
