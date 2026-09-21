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

// enforceAgentCapabilities rejects agent-mode inputs that would express
// arbitrary execution or silently point policy evaluation at a different
// config file. CI mode is unchanged: sanitized testArgs and alternate
// configPath remain available for human/automation workflows.
func enforceAgentCapabilities(mode Mode, testArgs []string, configPath, serverConfigPath string) error {
	if mode == ModeCI {
		return nil
	}
	if len(testArgs) > 0 {
		return &SanitizationError{
			Field:  "testArgs",
			Value:  fmt.Sprintf("%q", testArgs),
			Reason: "arbitrary runner arguments are not an agent capability; use typed fields (packages, tags, race, short, run, timeout)",
			Code:   CodeArbitraryArgs,
		}
	}
	if configPath != "" && configPath != serverConfigPath {
		return &SanitizationError{
			Field:  "configPath",
			Value:  configPath,
			Reason: "agent mode cannot select an alternate policy file; repository .coverctl.yaml is authoritative",
			Code:   CodePolicyOverride,
		}
	}
	return nil
}
