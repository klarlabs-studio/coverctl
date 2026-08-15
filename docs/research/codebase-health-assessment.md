# coverctl Codebase Health Assessment

Evidence-based review of what is strong, what is weak, and what to improve
next. Originally dated 2026-08-15; **updated after the health-improvements
implementation pass** on the same day.

This is a research artifact, not a roadmap commitment.

---

## Executive verdict

coverctl has a clear wedge (**agent-loop coverage governance**), real
MCP security thinking, enforced DDD boundaries, and unusually mature
release plumbing for a CLI of this size.

**After the improvements pass:** dogfood gates now cover `mcp`, `cli`,
`application`, `eval`, `runners`, `parsers`, and related infra;
output sanitization covers suggest/resources/pr-comment; eval corpus
is 70+ scenarios; god files sit well under lowered ceilings; docs
honesty gaps (scenario count, rejection codes, Relicta filename,
coverage policy wording) are closed; runner `Detect()` reads
`application.Languages`.

Remaining gaps are mostly GTM validation (usability study not run),
docs-site IA, and deeper polyglot CI smoke — see Still open in
`docs/backlog.md`.

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
  `internal/architecture/architecture_test.go`.
- File-size ceilings enforced; after extraction: `cli.go` ~467/490,
  `service.go` ~1294/1400, `server.go` ~914/1000.
- Shared application service is the right seam for CLI + MCP.
- Mode-aware MCP surface: agent mode advertises 3 tools; CI expands.
- `coverctl mcp doctor` closes opaque-setup failures (#8 / #19).
- Runner detection unified on `application.Languages` via
  `detectByLanguageMarkers`.

### MCP security

- Documented Lethal Trifecta threat model.
- Input sanitization with stable `error_code` + remediation (16 codes).
- Output canonicalization on check/report/compare/debt/**suggest**/
  **pr-comment**/resources.
- Rate limit on `pr-comment`; opt-in telemetry; `writeConfig` CI-gated.
- Eval harness: **70+** embedded scenarios; RuleJudge in CI.

### Engineering maturity

- Self-dogfood coverage workflow builds from PR source.
- Domains for mcp/cli/application/eval/runners/parsers (+ infra)
  with floors set from measured coverage.
- Release path: GoReleaser, cosign, SBOM, Homebrew, MCP Registry.
- Pinned Action SHAs; golangci; nox security remediation.

---

## What improved in the health pass

| Gap | Fix |
| --- | --- |
| Hollow dogfood (cli 25%, application 18%, mcp ungated) | Raised floors; added mcp/eval/runners/parsers/pathutil/history/paths/resolver/badge |
| Docs overclaim (50+ evals, 13 codes, relicta.config.yaml) | README 70+; ARCHITECTURE 16 codes; AGENTS `.relicta.yaml` + honest coverage wording |
| Output sanitize incomplete | suggest, resources, compare errors, pr-comment body |
| God files at ceiling | Extracted CLI helpers + aggregate.go; lowered ceilings |
| Thin eval corpus (11) | Grown to 73 adversarial/schema/judge scenarios |
| Detect marker drift | Shared `detectByLanguageMarkers` |
| Hollow registry fitness test | Asserts each Language const name appears as `Code:` entry |
| Stale backlog | Shipped vs still-open status header |

---

## What remains weak / open

1. **Usability study not run** — protocol exists; no findings.
2. **Docs Astro site IA** — comparison pages, blog, pricing, sidebar
   journey (see backlog Still open).
3. **Polyglot CI smoke** — mocked buildArgs exist; real toolchains
   not in required CI matrix.
4. **Happy-path eval scenarios** — corpus is strong on adversarial
   input; thinner on successful agent-loop flows.
5. **GTM instrumentation** — specs exist; live funnel not productized.
6. **application package coverage** — floored at 30% (measured ~35%);
   still the softest core domain.

---

## Suggested next engineering slice

Run the 5-user usability protocol, then grow happy-path eval scenarios
and raise the `application` domain floor as tests land.

---

## Evidence sources

- Tree inspection + `go test -cover` per package
- `.coverctl.yaml`, workflows, AGENTS/ARCHITECTURE/README
- `internal/eval/scenarios/` count (73)
- GitHub issues (closed at original review time)
