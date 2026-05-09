
## Product Messaging & Docs Alignment

Align CLI help text, README, and PRD with current reality (15 languages, MCP-native, domain-aware positioning). Remove all Go-only wording from user-facing copy unless contextually Go-specific.

---

## AI/Agent Success Metrics Baseline

Define and instrument MCP tool-call success metrics: success rate, rejection rate, time-to-success, pre-commit regression catch rate. Enable data-driven decisions on which 3 MCP tools drive retained value.

---

## Architecture Drift Guardrails

Add explicit extraction plan and enforcement for acknowledged large files (service.go, cli.go, server.go). New capabilities must land in dedicated handler files per the existing ceiling-test contract.

---

## Golden Path UX for First-Run

Tighten docs and CLI flow around a single golden path: init -> check -> suggest -> record. Include actionable failure-snippet guidance per step. New user goal: install to first fix in <10 minutes.

---

## CI Product Metrics Dogfooding

Extend CI workflow (go.yml) to preserve machine-readable check/report outputs for trend analysis. Current dogfooding only asserts pass/fail; add structured artifact collection.

---

## GTM & Enterprise Readiness Package

Build ICP-focused targeting for polyglot AI-assisted teams with compliance governance needs. Publish security architecture doc (MCP threat model + sanitization boundaries).

---

## Coverage Quality Hotspot Uplift

Add scenario tests for weaker coverage surfaces: internal/mcp, internal/cli, runner edge cases. Focus on failure-path handling and parser/runner boundary conditions.

---

## Wedge Re-anchoring: PRD and ICP

Reframe PRD around agent-loop wedge (in-loop coverage feedback before commit). Prune personas to Taylor primary + Jordan secondary buyer. Replace vanity success metrics (5+ langs shipped, 1000 installs, GitHub stars) with North Star (Weekly Protected Agent Loops) plus input metrics (activation, MCP tool-call success, pre-commit hook adoption, regressions caught per session). Reframe ICP brief competitive alternatives to lead with red-CI-agent-loop status quo (not Codecov). Pull compliance-sensitive paths from ECP gate to expansion accelerator. Replace front-page positioning with single Initiative Hypothesis.

---

## Agent-Mode Onboarding Path

Add parallel Terminal vs AI Agent quick-start path. Agent-mode page shows install, enabling MCP server in Claude Code/Cursor/Cline, first agent-initiated check, agent UI transparency (tool-call visible), approval gate example, structured rejection example, override capability. Cross-link from terminal quick-start. Closes wedge-invisible gap for primary ICP at first contact.

---

## Golden-Path Failure-Mode Snippets

Add per-step caution blocks to quick-start.mdx covering predictable first-run failures: no language markers detected, missing language toolchain (pytest-cov, nyc, cargo-tarpaulin), profile path mismatch, threshold-too-high first FAIL, no tests detected. Each block names the failure, gives the exact recovery command. Closes the original T-5 requirement properly.

---

## Realistic CLI Output with Inline Next-Action Hints

Redesign coverctl check terminal output: realistic mixed PASS/FAIL rows with shortfall delta, summary line, and inline next-action footer (run coverctl suggest DOMAIN / coverctl debt). Update quick-start sample to mirror real output. Adds designed Peak-End moment on first passing check (subtle success line + next-step nudge). Touches internal/cli/check.go print path and docs sample.

---

## MCP Agent-Loop Eval Harness

Build internal/eval/ skeleton: 50-100 synthetic regression scenarios across supported languages (known coverage drop in known domain), scripted headless MCP agent replay, LLM-as-judge for output-interpretation accuracy, tool-selection accuracy and recall metrics, adversarial prompt-injection eval set. Wire into CI as gate. Establishes denominator for North Star regression catch rate that telemetry alone cannot measure.

---

## Mode-Aware MCP Tool Surface

Add coverctl mcp serve --mode=agent|ci flag. Agent mode advertises pruned 3-tool surface (check, suggest, debt) for reliable agent tool selection within context budget. CI mode advertises full 8 tools (adds report, compare, record, badge, pr-comment). Auto-detect mode by MCP client-id where possible. Validates the check/suggest/debt value-driver hypothesis from metrics spec.

---

## MCP Output Boundary Hardening

Canonicalize and escape user-controlled strings in MCP tool outputs (file paths, test names, profile contents, PR descriptions in pr-comment) before return to agent. Closes Lethal Trifecta exposure where untrusted content flows from coverage profiles back into agent context as a new prompt-injection vector. Add 50+ adversarial output-injection tests under internal/mcp/. Update docs/security/mcp-threat-model.md with output-boundary controls section.

---

## Structured Rejection Schema and Output Budgets

Stable JSON schema for all MCP rejection responses with required fields: passed=false, error_code, summary, remediation (agent-actionable next step). Add per-tool output token budget (default 2K), pagination cursors for overflow, verbosity flag (brief|normal|verbose) so agents can request minimal default. Auto-truncate verbose outputs (e.g., report) to top-N failing domains with summary. Reduces context pollution and prevents agent-stuck-on-rejection failures.

---

## Pricing and Monetization Wedge Decision

Two-page strategy decision doc evaluating monetization options for coverctl: open-core (paid hosted coverage history, team dashboards, cloud MCP relay), paid SLA support contracts, enterprise security feature gate (audit logging, SSO, compliance exports), or remain pure OSS with sponsorship. Decide before scaling community-led GTM. Decision artifact only, no implementation in this task. Output: docs/strategy/monetization-decision.md.

---

## Category Point-of-View Doc

Two-page Lochhead Point-of-View document defining the agent-loop coverage category. Sections: world today (red-CI agent loops, polyglot tool sprawl, governance gap), world we are describing (in-loop coverage governance, MCP-callable, polyglot-uniform), why now (3 bullets: Claude Code adoption inflection, MCP standard emerging, polyglot pain), the category name, who benefits most (pruned ECP). Drives README hero, content calendar seed articles, conference pitch language. Output: docs/strategy/category-pov.md.

---

## Activation Funnel and GTM Metrics

Distinct GTM funnel metrics layer separate from tool-execution telemetry: activation rate (init users reaching first passing check), 30-day usage retention (repos calling check weekly), advocate-mention count (unprompted Claude Code/Discord/X mentions), plugin marketplace install velocity, enterprise inbound (procurement document requests), Sean Ellis 40% PMF survey infrastructure. Opt-in trace donation pipeline for real-world data growing eval corpus.

---

## 5-User Polyglot Usability Test

Krug-style observed usability test: recruit 5 polyglot devs actively using Claude Code or Cursor, watch them install coverctl from scratch and reach first fix. Two using Python+TS, two using Go+Rust, one using Java or Shell. Measure: did they discover MCP/agent integration unprompted, did first failed check produce a clear next action, time-to-first-fix, abandonment points. Single-day spend. Validates onboarding fixes from features F2/F3/F4 before broader rollout.

---
