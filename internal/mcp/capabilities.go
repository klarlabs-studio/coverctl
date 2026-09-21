package mcp

import "fmt"

// Agent-mode execution is capability-based: the caller expresses intent
// (packages, tags, race, timeout, short, run) and each runner maps those
// fields to tool-specific argv. Arbitrary testArgs are a human/CI escape
// hatch and must not leak onto the agent surface — otherwise coverctl
// becomes a generic command-execution MCP server and has to maintain an
// ever-growing cross-language denylist of dangerous flags.
//
// See docs/strategy/system-intent.md ("Capability-Based Execution",
// "Policy Authority").

// agentInvocation is the subset of MCP check/suggest/debt input that
// agent mode must constrain. CI mode leaves these fields to the existing
// sanitizers and human/automation workflows.
type agentInvocation struct {
	TestArgs    []string
	ConfigPath  string
	Domains     []string
	FromProfile bool
	Incremental bool
}

// enforceAgentCapabilities rejects agent-mode inputs that would express
// arbitrary execution, evaluate a weaker or partial policy, or skip the
// test run that check exists to perform. CI mode is unchanged: sanitized
// testArgs, alternate configPath, domain filters, fromProfile, and
// incremental remain available for human/automation workflows.
func enforceAgentCapabilities(mode Mode, inv agentInvocation, serverConfigPath string) error {
	if mode == ModeCI {
		return nil
	}
	if len(inv.TestArgs) > 0 {
		return &SanitizationError{
			Field:  "testArgs",
			Value:  fmt.Sprintf("%q", inv.TestArgs),
			Reason: "arbitrary runner arguments are not an agent capability; use typed fields (packages, tags, race, short, run, timeout)",
			Code:   CodeArbitraryArgs,
		}
	}
	if inv.ConfigPath != "" && inv.ConfigPath != serverConfigPath {
		return &SanitizationError{
			Field:  "configPath",
			Value:  inv.ConfigPath,
			Reason: "agent mode cannot select an alternate policy file; repository .coverctl.yaml is authoritative",
			Code:   CodePolicyOverride,
		}
	}
	if len(inv.Domains) > 0 {
		return &SanitizationError{
			Field:  "domains",
			Value:  fmt.Sprintf("%q", inv.Domains),
			Reason: "agent mode cannot restrict policy evaluation to a subset of domains; repository policy is authoritative",
			Code:   CodePartialPolicy,
		}
	}
	if inv.FromProfile {
		return &SanitizationError{
			Field:  "fromProfile",
			Value:  "true",
			Reason: "agent mode cannot skip the test run; check must execute tests so verification cannot be satisfied by a planted profile",
			Code:   CodeSkipVerification,
		}
	}
	if inv.Incremental {
		return &SanitizationError{
			Field:  "incremental",
			Value:  "true",
			Reason: "agent mode cannot run incremental check; an empty diff auto-passes without evaluating repository policy",
			Code:   CodeIncremental,
		}
	}
	return nil
}
