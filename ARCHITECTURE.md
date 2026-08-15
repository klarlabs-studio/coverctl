# coverctl architecture

Internal layout. Read this if you are contributing or auditing.
End users do not need it.

## Layers

Strict DDD with dependencies pointing inward:

- `internal/domain` — coverage stats, policy evaluation, value objects.
  Knows nothing of CLI, MCP, or infrastructure.
- `internal/application` — orchestration: check, run, report, detect,
  record, compare, debt, suggest. Owns the use-case-shaped service
  interface used by both CLI and MCP entrypoints.
- `internal/infrastructure` — runners for 15 language IDs (14 runner
  implementations; TypeScript aliases to the Node runner), parsers
  (Go coverprofile, LCOV, Cobertura, JaCoCo), config loader, history
  store, PR clients (GitHub, GitLab, Bitbucket).
- `internal/cli` — CLI parsing, output formatters, golden-path output
  with shortfall delta and inline next-action footer.
- `internal/mcp` — MCP server, input sanitization, output
  canonicalization, tool/resource handlers, mode-aware tool exposure
  (agent vs CI), structured rejection schema with `error_code` and
  `remediation`.
- `internal/eval` — agent-loop eval harness: scenario corpus,
  RuleJudge, optional HTTPLLMJudge, embed.FS-backed scenarios.
- `internal/architecture` — ceiling tests preventing handler-file
  growth past their documented limits.

## Wedge artifacts

The wedge is **agent-loop coverage governance**. Source-of-truth
documents:

- `docs/strategy/category-pov.md` — category narrative.
- `docs/strategy/monetization-decision.md` — open-core path,
  stage gates.
- `docs/gtm/icp-brief.md` — ICP / motion stack.
- `docs/design/language-agnostic-prd.md` — Initiative Hypothesis,
  North Star (Weekly Protected Agent Loops), input metrics.
- `docs/design/mcp-metrics-spec.md` — tool-call success metrics,
  rejection rate, time-to-success, regression catch rate.
- `docs/design/gtm-metrics-spec.md` — activation funnel metrics
  (activation rate, 30-day retention, mentions, install velocity,
  enterprise inbound), opt-in trace donation pipeline design.
- `docs/security/mcp-threat-model.md` — Lethal Trifecta framing,
  input + output boundary controls, residual risk.

## Boundaries

- **MCP input boundary:** `internal/mcp/sanitize.go`. Stable rejection
  schema in `rejectionResponse` and `errorResponse`. 16 stable
  `RejectionCode` constants (8 input + 8 operational) with
  operator-actionable remediation copy in `remediationFor` and
  inline at error sites.
- **MCP output boundary:** `internal/mcp/sanitize_output.go`. File
  paths canonicalized to `[A-Za-z0-9._/-]`; free-form strings have
  control characters stripped, backticks rewritten, length capped.
- **Module-root resolution:** `internal/infrastructure/gotool/module.go`
  emits typed `ModuleRootError` with cwd and searched paths;
  `internal/mcp/runtime_errors.go` recognizes the error and emits
  schema-conformant rejection with `OpCodeModuleRootMissing` +
  `ModuleRootRemediation`. Same hint surfaces from CLI via
  `remediationHintForError` in `internal/cli/cli.go`.

## Mode-aware exposure

`coverctl mcp serve --mode=agent|ci|auto`:

- agent (default): advertises 3 tools — `check`, `suggest`, `debt`.
  Pruned for reliable agent tool selection inside the edit loop.
- ci: advertises full 9-tool surface for non-agent callers.
- auto: env-var heuristic (`GITHUB_ACTIONS`, `GITLAB_CI`, `BUILDKITE`,
  `CIRCLECI`, `JENKINS_URL`, `TF_BUILD`, `CI`).

## Eval harness

`internal/eval/`:

- `Scenario` JSON files under `scenarios/` (embed.FS).
- `RuleJudge` deterministic substring assertions.
- `HTTPLLMJudge` opt-in via `COVERCTL_EVAL_LLM_JUDGE=1` +
  `ANTHROPIC_API_KEY`. Calls Anthropic Messages API directly;
  no SDK dependency.
- `Server.Dispatch` is the in-process seam used by the harness so
  scenarios run in the same Go test binary.
- CI gate: `.github/workflows/eval.yml`. Rule-only on push/PR;
  LLM-judge only on manual `workflow_dispatch` with secret.

## First-run validation

`coverctl mcp doctor`: 6-step diagnostic (binary on PATH, working-
directory markers, config resolvable, MCP server constructs, tool
dispatch smoke verifying rejection schema, mode auto-detect). Each
step prints PASS/FAIL with remediation. Returns 0 only when every
step passes. Closes the opaque-setup-failure pattern surfaced in
issues #8, #19.
