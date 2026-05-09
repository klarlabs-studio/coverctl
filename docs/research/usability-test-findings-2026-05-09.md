# Polyglot Claude Code Usability Test — Findings (2026-05-09)

## Methodology caveat — read first

This report is **not** the live 5-user cohort described in
`usability-test-protocol.md`. Live recruitment did not run before this
iteration's review window closed. To still produce decision-grade
signal we used two proxy sources:

1. **Closed-issue mining (real signal, past-tense).** All closed issues
   on `felixgeelhaar/coverctl` were read, categorized by friction tag,
   and cross-checked against the merge log to identify which patterns
   were already addressed and which still pose risk to new users
   running today's release.
2. **Synthetic persona walkthrough (low signal, present-tense).** Five
   personas matching the protocol's cohort archetypes were walked
   through the current docs and CLI by the maintainer, scoring each
   against the protocol's observation rubric. Synthetic walkthroughs
   capture obvious gaps; they do not replicate real-user surprise,
   trust calibration, or behavioral drift.

**Trust hierarchy in this document:** closed-issue findings are
primary evidence (real users hit them, real fixes followed). Synthetic
walkthrough findings are *hypotheses awaiting live validation*; they
are flagged with `[synthetic]` so a future reader can weight them
appropriately. The live cohort still needs to run before iteration
N+1 ships.

## Sources

- 10 closed GitHub issues (#8, #14, #15, #19, #20, #22, #23, #24, #36,
  #37). Merged-PR cross-reference: PR #48 fixed Rust LCOV detection;
  PR #49 added language-aware parser fallback. Both shipped 2026-03-28
  and resolve issues #36/#37.
- Synthetic walkthroughs against current main (`a98b9e0`) docs:
  `quick-start-agent.mdx`, `quick-start.mdx`, `mcp.mdx`, landing
  `index.mdx`.

## Cohort summary

| ID | Stack | Source | Notes |
| --- | --- | --- | --- |
| Real-A | Rust + Cargo | Issues #36, #37 | LCOV format mismatch, fixed by PR #48 |
| Real-B | Go + MCP | Issues #8, #15, #19, #20 | MCP-setup + Go module-root cluster |
| Real-C | Go + CLI | Issues #22, #23, #24 | profile flag, transport, stale history |
| P1 | Python + TS | Synthetic | Claude Code primary user |
| P2 | Python + TS | Synthetic | Cursor user, mid-size repo |
| P3 | Go + Rust | Synthetic | Strong-typing cohort |
| P4 | Go + Rust | Synthetic | Confirmation persona |
| P5 | Java or Shell | Synthetic | Smaller-ecosystem stack |

## Per-source rubric (real issues)

Scored from issue text + merge log. "Recovered" means the user found a
workaround or the project shipped a fix that resolves the friction in
current main.

| Cluster | First-fix time (real) | MCP setup | Failure recovery | Trust calibration | Status today |
| --- | --- | --- | --- | --- | --- |
| Real-A (Rust LCOV) | abandoned until PR #48 | n/a | self-recovered via workaround (raw `cargo llvm-cov --text`) | distrust → "tool doesn't support my language" | **fixed** in current main |
| Real-B (MCP first-run) | abandoned (server EOF on init) | failed | required GitHub issue; no in-product remediation | over-trusted at install, distrust on failure | partially fixed; #8 resolved by mcp-go upgrade chain (PR #34/#44); #19/#20 cwd context still concerning |
| Real-C (CLI profile flow) | self-recovered with manual workaround | n/a | reported as bug rather than UX | calibrated | partially fixed |

### Verbatim signals (closed-issue evidence)

These are **real user words** from real friction events. They are the
strongest signal in this report.

- *Issue #8, MCP first-run abandonment:* "The MCP server (`coverctl
  mcp serve`) exits immediately upon receiving an initialize request,
  preventing integration with Claude Code and other MCP clients."
- *Issue #19, agent context confusion:* "The `check` command should
  run `go test -coverprofile=...` and analyze the results... Returns
  'go test failed: exit status 1'... Returns 'No domains found'."
- *Issue #20, opaque failure-mode:* "The `check` command fails with
  `'module root not found'` even when running in a valid Go module
  directory with `go.mod` present... Provide clearer error message
  explaining what's missing (e.g., 'No go.mod found in /path')."
- *Issue #36, language support boundary:* "All commands return
  `{'error': 'invalid coverage mode line'...}`." (User had to read
  source to understand the parser was Go-format-only.)
- *Issue #37, persistent format gap:* "`coverctl run` delegates to
  `cargo-llvm-cov` which exports using `llvm-cov export -format=lcov`.
  The resulting `.cover/coverage.out` starts with LCOV records...
  instead of Go's coverage header."

## Per-persona rubric (synthetic walkthrough)

| Signal | P1 | P2 | P3 | P4 | P5 |
| --- | --- | --- | --- | --- | --- |
| Agent-mode discovery | did `[synthetic]` | did `[synthetic]` | told `[synthetic]` | did `[synthetic]` | told `[synthetic]` |
| First-fix time est. | ~6 min | ~7 min | ~9 min | ~7 min | ~12 min |
| Failure recovery (init) | self | self | self | self | uncertain — Shell repo edge case |
| Failure recovery (check) | self via footer | self via footer | self via footer | self via footer | uncertain |
| MCP setup | success | success | success | success | success-with-friction |
| Trust calibration | calibrated | calibrated | calibrated | calibrated | under-trusted (smaller-ecosystem caveat fires) |
| First emotional peak | inline next-action footer ("→ coverctl suggest api") | same | shortfall delta column | same | passing first PASS row |
| First friction peak | wizard arrow-key behavior unclear | language auto-detect for mixed Python/TS repo | none flagged | none flagged | parser support uncertainty |

`[synthetic]` reminder: discovery was scored against the index hero
parallel-path selector + agent-mode link as a structural signal; real
users may default to terminal even with the selector present. Validate
in live cohort.

## Headline metrics

Mixed-source aggregate. Treat real-issue-derived numbers as primary,
synthetic estimates as bounds-checking.

| Metric | Result | Target | Source |
| --- | --- | --- | --- |
| MCP first-run success today | likely (#8 fix shipped) | high | issues + merge log |
| First-fix < 10 min on common stacks | est. 4/5 in Python+TS / Go+Rust | ≥ 3/5 | synthetic walkthrough |
| First-fix < 10 min on Rust pre-PR#48 | 0/2 (abandoned) | ≥ 3/5 | issues #36, #37 |
| First-fix < 10 min on Rust post-PR#48 | likely yes | ≥ 3/5 | merge log; live verification needed |
| Recovery without help on opaque error | weak — issue #20 quote captures user explicitly asking for clearer message | ≥ 4/5 | issue #20 |
| Sean Ellis "very disappointed" proxy | n/a — not measurable from issue corpus | ≥ 2/5 | needs live cohort |

## Friction map

Aggregate of real issues + synthetic walkthrough findings.

| Tag | Real-issue count | Synthetic count | Notes |
| --- | --- | --- | --- |
| discovery | 0 | 1 (P3, P5) | Synthetic only; live cohort required |
| install | 0 | 0 | — |
| init | 0 | 1 (P5) | Smaller-ecosystem language detection |
| check | 5 | 0 | Real cluster: profile/parser failures |
| MCP-setup | 2 | 1 (P5) | #8 (server exit), #15 (version) |
| agent-trust | 1 | 1 (P5) | #19 cwd context implies trust loss |
| fix-loop | 1 | 0 | #19 forced manual coverage workflow |

## What we change

Three concrete iterations the data justifies. All three are derived
from real-issue evidence, not synthetic hypothesis.

1. **Surface clearer "module root not found" remediation in `check`.**
   Issue #20 user explicitly asked: "Provide clearer error message
   explaining what's missing (e.g., 'No go.mod found in /path')."
   Current rejection schema (`internal/mcp/sanitize.go`) covers MCP
   input boundary. Add equivalent operational error code on the
   *runtime* boundary: when `check` cannot resolve module root,
   emit a structured response with searched paths + explicit hint to
   pass `--language` or run from repo root.
   **Where:** `internal/application/service.go` and matching MCP
   handler path; new `OpCodeModuleRootMissing` rejection code.

2. **Document Rust LCOV happy path in agent-mode quick-start.** Two of
   the four highest-friction issues (#36, #37) were Rust users hitting
   parser format mismatch. PR #48 fixed it; the agent-mode quick-start
   does not yet show a Rust example, so a Rust user's first instinct
   may still be "this is for Go." Add a tabbed Rust example mirroring
   the Python and TypeScript blocks in `quick-start.mdx`.
   **Where:** `docs/src/content/docs/quick-start.mdx` Tabs (also
   propagate to `quick-start-agent.mdx`).

3. **Add `coverctl mcp doctor` subcommand for first-run validation.**
   Issue #8 (MCP server EOF on initialize) and #19 (cwd context
   failure) both surfaced as opaque failures the user could not
   diagnose. A diagnostic subcommand that runs the same handshake
   `mcp serve` would receive but prints structured success/failure
   output to stderr would cut "abandoned setup" cases. The `coverctl
   mcp serve --help` smoke check in `mcp.mdx` is a weak substitute.
   **Where:** new `internal/cli/cmd_mcp.go` subcommand; new roady
   feature.

## What we keep

Things the data confirms work and should not change in N+1.

- **Rejection schema with `error_code` + `remediation`.** Issue #20
  ("provide clearer error message explaining what's missing") would
  have been cheaper to debug if the rejection schema we shipped this
  iteration had been live for that user. Keep the field set.
- **Mode-aware MCP tool surface.** No real-issue evidence of selection
  drift since issues predate the change, but the change is the right
  shape per the literature on agent tool-selection.
- **Output canonicalization.** No issues evidenced filename injection
  yet; the output boundary is defense-in-depth and stays.

## What we explicitly defer

Real or synthetic signals we choose not to act on this iteration, with
reason.

- **Stale coverage history (issue #22).** Workaround exists; not on
  the wedge. Reopen if Stage 2 monetization (hosted history) raises
  the priority.
- **`--profile` flag ignored (issue #24).** CLI bug; fix on the next
  CLI cleanup pass, not in the wedge re-anchoring epic.
- **Synthetic-only friction (P5 Shell parser uncertainty).** No real
  user has reported this; do not preemptively rework Shell support
  without a real signal.

## Stop-condition triggers

The protocol's early-exit conditions did not fire because the cohort
did not run live. Real-issue count provides parallel evidence: the
distribution is heavy on `check` cluster (5 issues), light on
discovery and install (0 each). Concentrating engineering time on the
`check` cluster — exactly what the "What we change" list does — is
the right allocation.

## Hand-off

- New roady features to create from "What we change":
  1. Module-root failure remediation (operational rejection code +
     handler integration).
  2. Rust example in quick-start tabs.
  3. `coverctl mcp doctor` subcommand.
- Updates to existing docs: none in this report; iterations land via
  the new features above.
- **Live cohort still scheduled.** Synthetic findings flagged
  `[synthetic]` are awaiting validation. Run the protocol when
  recruitment slots open; replace synthetic rows in this report with
  live data; do not delete the closed-issue evidence.
- Next review date: **2026-06-20** (six weeks from this report).
