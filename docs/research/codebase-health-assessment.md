# coverctl Codebase Health Assessment

Evidence-based review of what is strong, what is weak, and what to improve
next. Dated 2026-08-15. Scope: product positioning, architecture, MCP
security, polyglot depth, dogfood coverage, docs drift, and GTM readiness.

This is a research artifact, not a roadmap commitment. Ranked actions at
the end are ordered by leverage for the agent-loop wedge.

---

## Executive verdict

coverctl has a clear wedge (**agent-loop coverage governance**), real
MCP security thinking, enforced DDD boundaries, and unusually mature
release plumbing for a CLI of this size. The main risks are **honesty
gaps** (docs claim more eval corpus and coverage discipline than the
repo practices), **hollow self-gates** (`.coverctl.yaml` under-protects
the MCP/CLI surface), and **god-file debt** (`cli.go` is 3 lines under
its ceiling). Strategy docs are ahead of execution on community proof
and usability validation.

---

## What is good

### Product and positioning

- README and landing copy are re-anchored on the agent loop, not Codecov
  dashboards. Hero → install → MCP wire-up → agent transcript is a
  coherent first viewport.
- Strategy stack is unusually complete for an OSS tool:
  `docs/strategy/category-pov.md`, `monetization-decision.md`,
  `docs/gtm/icp-brief.md`, design metrics specs, threat model.
- Open-core monetization decision is recorded (CLI/MCP free; hosted
  history additive) — reduces bait-and-switch risk later.

### Architecture

- Strict onion DDD with **import fitness tests** in
  `internal/architecture/architecture_test.go` (domain cannot import
  outer layers; application cannot import infra/cli/mcp).
- File-size ceilings acknowledge god-file debt instead of pretending it
  does not exist (`cli.go` ≤1400, `service.go` ≤1600, `server.go` ≤1000).
- Shared application service is the right seam for CLI + MCP.
- Mode-aware MCP surface exists: agent mode advertises 3 tools
  (`check`, `suggest`, `debt`); CI mode expands to the full set.
- `coverctl mcp doctor` closes the opaque-setup failure pattern from
  issues #8 / #19.

### MCP security

- Documented Lethal Trifecta threat model
  (`docs/security/mcp-threat-model.md`).
- Input sanitization (`internal/mcp/sanitize.go`) rejects dangerous
  runner flags, shell metacharacters, path-scope escapes; stable
  `error_code` + remediation schema.
- Output canonicalization exists (`sanitize_output.go`) for paths and
  free-form strings on primary tools.
- Rate limit on `pr-comment`; opt-in telemetry; `writeConfig` gated to
  CI mode.
- Eval harness skeleton (`internal/eval/`) with RuleJudge + optional
  LLM judge, wired in `.github/workflows/eval.yml`.

### Engineering maturity

- Self-dogfood coverage workflow builds from PR source
  (`.github/workflows/coverage.yml`) — correct design for a coverage
  tool.
- Release path: GoReleaser, cosign keyless signing, SBOM, Homebrew tap,
  MCP Registry publish.
- Pinned GitHub Action SHAs; golangci; nox security remediation.
- Almost no TODO/FIXME litter in production Go.

### Issue hygiene

- Historical user pain (Rust LCOV, module-root detection, MCP EOF,
  stale history, profile flags) is closed. No open issues at assessment
  time — either strong triage or low inbound volume.

---

## What is bad / weak

### 1. Docs overclaim vs code reality

| Claim | Reality |
| --- | --- |
| README: “50+ scenarios under `internal/eval/`” | **11** JSON scenario files |
| AGENTS.md: “>80% for each domain” | `cli` min **25**, `application` min **18**; **mcp / runners / parsers / eval ungated** |
| ARCHITECTURE.md: “13 stable RejectionCode constants” | **16** (8 input + 8 op) |
| AGENTS.md: `relicta.config.yaml` | Actual file: **`.relicta.yaml`** |
| “runners (15 languages)” | 15 language IDs; **14** runners; TypeScript aliases to Node |

Honesty gaps matter: procurement and agent users calibrate trust from
these numbers. Overclaiming the eval corpus undermines the security
story the README leans on.

### 2. Dogfood coverage does not protect the wedge

`.coverctl.yaml` intentionally lowers floors on the packages that
implement the product surface. `mcp` is not a domain at all. CI will
pass while MCP/CLI regressions accumulate. AGENTS.md still tells
contributors the opposite.

### 3. God files at the edge of their ceilings

| File | LOC | Ceiling | Headroom |
| --- | --- | --- | --- |
| `internal/cli/cli.go` | 1397 | 1400 | **3** |
| `internal/application/service.go` | 1527 | 1600 | 73 |
| `internal/mcp/server.go` | 925 | 1000 | 75 |

Any non-trivial CLI change can trip the architecture test. Extraction
to `cmd_*.go` / handlers is incomplete; aggregation helpers still live
on `Service`.

### 4. Output sanitize incomplete on agent-facing paths

Applied on `check` / `report` / `debt` (and partially compare). Gaps
observed in review:

- `suggest` returns suggestions / errors without full output scrubbing
- `compare` error path returns `err.Error()` raw in at least one branch
- MCP **resource** handlers marshal JSON that can carry unsanitized
  path/name fields into agent context
- `pr-comment` path returns comment body content that is a known
  residual risk in the threat model

The input boundary is stronger than the output boundary — opposite of
what Lethal Trifecta residual risk needs once profiles/PR text are
attacker-influenced.

### 5. Eval corpus is a skeleton, not a gate worthy of the marketing

11 scenarios cover dangerous flags, metachar, path scope, tags/timeout/run,
and schema fields. Missing: polyglot runner flags, resource URIs,
suggest/writeConfig, hostile-filename output injection end-to-end, mode
advertising. README “50+” and backlog “50–100” set a bar the tree does
not meet.

### 6. Polyglot depth is uneven

- Four profile formats (Go coverprofile, LCOV, Cobertura, JaCoCo) —
  correct, not “15 parsers.”
- Runner `Detect()` re-lists markers instead of reading
  `application.Languages` → drift risk.
- Shell detection is broad (`*.sh` + test dirs) → false positives in
  polyglot repos.
- CI dogfood is Go-only; most runner `Run` tests skip without toolchains.
- Kotlin rides Java extensions without a first-class language ID.

“15 languages” is enum-count marketing. Major stacks work; long-tail
quality is not proven in CI.

### 7. Backlog is stale relative to shipped work

`docs/backlog.md` still lists items that ARCHITECTURE/README treat as
done (mode-aware MCP, eval harness skeleton, mcp doctor, monetization
decision, category POV, output budgets, etc.). Without pruning, the
backlog cannot drive prioritization.

### 8. GTM / validation gap

- Usability protocol exists (`docs/research/usability-test-protocol.md`)
  but no findings report — 5-user study not run.
- Community surfaces (marketplace install counts, Discussions CTA,
  Sponsors, social proof) are documented as needed; inbound evidence is
  thin (closed issues, no open issues).
- North Star (Weekly Protected Agent Loops) is defined in PRD/metrics
  specs; instrumentation/adoption measurement is not yet a product loop.

### 9. Minor fitness-test holes

- Architecture registry completeness test can short-circuit on a single
  matching `Code:` substring (does not prove every language is
  registered).
- Ignored `Getwd` error in glob resolver; dead `_ = strings.HasSuffix`
  check in service aggregation.

---

## What should be improved (ranked by impact)

### P0 — Protect the wedge

1. **Extend and raise `.coverctl.yaml` domains**  
   Add `mcp`, `runners`/`parsers`, `eval`. Lift `cli` and `application`
   floors toward honest bars. Align AGENTS.md with real policy or raise
   policy to match AGENTS.md. Highest leverage: dogfood currently under-
   protects the agent-loop surface.

2. **Docs honesty pass**  
   Fix scenario count (11), rejection-code count (16), Relicta filename,
   coverage-policy wording, and “15 runners” phrasing. Trust is part of
   the security product.

3. **Finish output-boundary coverage**  
   Sanitize `suggest`, resource handlers, compare error strings, and any
   PR-derived strings returned to agents. Add adversarial tests for
   hostile filenames in profiles.

### P1 — Keep the architecture movable

4. **Extract before feature work on CLI**  
   `cli.go` has ~3 LOC headroom. Move remaining logic into `cmd_*.go`;
   pull `AggregateByDomain*` out of `service.go`; then **lower** ceilings.

5. **Grow eval corpus toward the claimed bar**  
   Target 50+ scenarios spanning input rejection, output injection,
   schema integrity, and mode advertising. Keep RuleJudge in CI; LLM
   judge optional.

6. **Single detection source of truth**  
   Runner `Detect()` must consume `application.Languages` markers and
   priorities. Fixes shotgun drift and false-positive shell detection.

### P2 — Prove polyglot and GTM

7. **Runner quality bar**  
   Require mocked `Run`/`buildArgs` tests that do not skip; document
   which languages are production-hardened vs detect-only. Optional CI
   matrix job with Python/Node/Rust toolchains for smoke.

8. **Prune and re-rank `docs/backlog.md`**  
   Mark shipped items done; keep only unfinished work. Surface P0 items
   above.

9. **Run the 5-user usability protocol**  
   Protocol and findings template already exist. Without observed
   sessions, onboarding claims are unvalidated.

10. **Community / platform surfaces**  
    Marketplace + MCP Registry links, Discussions CTA, Sponsors, and a
    thin “For platform teams” inbound path (already sketched in backlog)
    when ready to spend GTM time.

---

## Keep doing

- Enforced DDD + ceiling tests (tighten ceilings after extraction).
- Input + output security framing with stable rejection codes.
- Source-built dogfood coverage workflow.
- Agent-mode pruned tool surface + `mcp doctor`.
- Signed releases, SBOM, pinned actions.
- Strategy docs as decision artifacts before implementation.

---

## Suggested next engineering slice

If only one change lands after this assessment: **raise dogfood gates
for `mcp` + `cli` and correct the README eval-count / AGENTS coverage
claims**. That couples product honesty with regression protection on the
wedge without requiring a large feature.

---

## Evidence sources

- Tree inspection: `internal/{cli,application,mcp,eval,architecture,infrastructure}`
- Config: `.coverctl.yaml`, `.github/workflows/*`, `AGENTS.md`,
  `ARCHITECTURE.md`, `README.md`
- Strategy: `docs/{strategy,gtm,design,security,backlog,research}/*`
- GitHub issues via `gh` (all listed issues closed; none open at review)
- Line counts via `wc -l` on god files; scenario count via directory
  listing (`internal/eval/scenarios/` = 11)
)
