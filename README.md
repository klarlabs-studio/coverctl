# coverctl

**Agent-loop coverage governance — coverage your AI coding agent calls before commit, not a dashboard you read after CI.**

[Get started ↓](#get-started) · [What it looks like ↓](#what-it-looks-like-in-the-agent-loop) · [MCP tools ↓](#mcp-tools) · [CLI reference ↓](#cli-reference) · [Why this exists ↓](#why-this-exists) · [Community ↓](#community)

![MCP](https://img.shields.io/badge/MCP-server-blueviolet) ![Releases](https://img.shields.io/github/v/release/klarlabs-studio/coverctl?label=release)

> *"Our AI agents ship code fast, but they're blind to coverage policy while editing. We only see breakage in CI, after context is gone, and the same agent then guesses its way to a fix."*

Works best on standard Go/Python/JavaScript/Java/Rust projects with conventional layouts. Mock-heavy code or exotic monorepos may need an explicit `domains:` block in `.coverctl.yaml`.

## Get started

```bash
brew install klarlabs-studio/tap/coverctl
```

Wire into Claude Code (`~/.config/claude-code/mcp.json`):

```json
{
  "mcpServers": {
    "coverctl": {
      "command": "coverctl",
      "args": ["mcp", "serve"]
    }
  }
}
```

Ask the agent: *"Run coverctl check and tell me which domains regressed."*

For Cursor / Cline / Claude Desktop / Aider / Continue / OpenCode and other MCP clients, see [docs/src/content/docs/mcp.mdx](docs/src/content/docs/mcp.mdx). All MCP-capable clients work; `coverctl mcp serve` runs in agent mode by default (3 tools: `check`, `suggest`, `debt`). Use `--mode=ci` for the full nine-tool surface.

Validate the install end-to-end:

```bash
coverctl mcp doctor   # PASS/FAIL per step with remediation
```

## What it looks like in the agent loop

*Rendered for clarity — your client may format tool-calls differently.*

```text
You:    Add the email-validation feature to the api domain.

Claude: I'll add EmailValidator + tests in internal/api/email.

  [edits: internal/api/email.go, internal/api/email_test.go]

  Tool call → coverctl check
  Result   → passed: false; failing: api (78.2%, required 80%)
             uncovered: internal/api/email.go:42-58 (error paths)

  Tool call → coverctl suggest api
  Result   → uncovered files: email.go (16 lines), retry.go (4 lines)

  Two error branches in EmailValidator.Validate are uncovered.
  I'll add cases for empty-input and malformed-domain so api meets
  the 80% threshold before we commit.
```

The regression that used to surface in CI 8 minutes later is fixed in the same agent turn.

**The agent does well:** reads `check` output verbatim, calls `suggest` to find uncovered files, calls `debt` to rank smallest fixes, re-runs `check` to confirm.

**Watch for:** agents lowering thresholds in `.coverctl.yaml` to "fix" a failure (it's not a fix). Agents claiming coverage rose without a new `check` call (hallucination). Agents ignoring rejection `error_code` and retrying with the same input.

coverctl returns deterministic structured signals; the agent's *fix* still needs a human reading. The full contract is in [docs/src/content/docs/mcp.mdx](docs/src/content/docs/mcp.mdx).

## Why this exists

AI coding agents write code blind to coverage. They edit, you commit, the regression surfaces in CI minutes or hours later — too late to course-correct in the same session. Existing coverage tools (Codecov, Coveralls, native `go test -cover`) target humans reading dashboards or PR comments, not agents reasoning inline.

coverctl is built for the agent loop:

- **Catches regressions before commit.** Coverage feedback in the agent's edit turn — not minutes after CI fails. The wedge metric is regressions caught pre-commit.
- **Agent-callable via MCP.** Speaks MCP — the multi-vendor agent-tool standard now governed by Anthropic, OpenAI, Google, Microsoft, AWS. Works with every MCP-capable client today; works with whatever ships next without modification.
- **Polyglot governance, one config.** One `.coverctl.yaml` enforces per-domain thresholds across 15 languages. Agents touch any language; coverage tooling must too.
- **Local-first.** Your source never leaves the machine. Agent calls coverctl over stdio, not over a SaaS API. No account, no upload, no third-party dependency in the agent's reach.
- **Hardened MCP surface.** Input + output sanitization defends against prompt-injection through test-runner flags and hostile filenames in coverage profiles (Lethal Trifecta).

The CLI and MCP server are Apache-2.0 licensed and stay free. A hosted layer for cross-repo coverage history is on the roadmap (see [docs/strategy/monetization-decision.md](docs/strategy/monetization-decision.md)) — additive, not a paywall.

## MCP tools

Agent mode advertises three tools (`check`, `suggest`, `debt`) for reliable agent tool selection. CI mode (`--mode=ci`) adds the rest.

| Tool | Mode | Purpose |
| --- | --- | --- |
| `check` | agent + ci | Run tests with coverage and enforce policy. Returns per-domain pass/fail, files, warnings. |
| `suggest` | agent + ci | Recommend thresholds (`current` / `aggressive` / `conservative`). |
| `debt` | agent + ci | Coverage gap per domain — where to spend effort, ranked. |
| `init` | ci | Auto-detect project structure and create `.coverctl.yaml` with domain policies. |
| `report` | ci | Analyze an existing coverage profile without running tests. |
| `record` | ci | Record current coverage to history for trend tracking. |
| `compare` | ci | Diff two coverage profiles. Returns delta, improved/regressed files, domain changes. |
| `badge` | ci | Generate SVG coverage badge. |
| `pr-comment` | ci | Post coverage report to GitHub / GitLab / Bitbucket PR. |

### MCP resources (read-only context)

| URI | Content |
| --- | --- |
| `coverctl://debt` | Coverage debt as JSON. |
| `coverctl://trend` | Trend over recorded history. |
| `coverctl://suggest` | Threshold suggestions. |
| `coverctl://config` | Detected project config. |

## Security boundaries

coverctl treats MCP traffic as untrusted in both directions, per the Lethal Trifecta threat model.

- **Input boundary.** Test-runner flags that allow arbitrary code loading (`--rootdir`, `--cov-config`, `-D`, `-I`, `--require`, `--init-script`, `--node-options`, ...) are rejected when they come from MCP. Rejection responses use a stable schema with `error_code` and agent-actionable `remediation`. CLI invocations from a human terminal are not sanitized; the human is the trust boundary there.
- **Output boundary.** User-controlled strings flowing *back* to the agent (filenames in coverage profiles, test names, profile-derived paths, PR description content in `pr-comment`) are canonicalized before return. Prevents return-trip prompt injection through a hostile PR or attacker-named test file.

coverctl is local-first. The default install transmits nothing — no telemetry, no analytics, no source data. An opt-in `--mcp-telemetry` flag emits structured tool-call events to stderr for users who want to instrument their own pipelines (format documented in [docs/design/mcp-metrics-spec.md](docs/design/mcp-metrics-spec.md)). Adversarial evals (50+ scenarios under [internal/eval/](internal/eval/)) gate every release on rejection-schema integrity and prompt-injection resistance.

Full threat model + residual risk: [docs/security/mcp-threat-model.md](docs/security/mcp-threat-model.md).

## CLI reference

The CLI is the substrate behind the MCP server; humans can use it directly.

| Command | Purpose |
| --- | --- |
| `init` / `i` | Interactive wizard, auto-detects language and domains. `--no-interactive` for CI. |
| `check` / `c` | Run coverage and enforce policy. `-o json` for machine output, `--fail-under N`, `--ratchet`, `--from-profile`. |
| `run` / `r` | Produce coverage artifacts without policy evaluation. |
| `watch` / `w` | Re-run coverage on file change during development. |
| `report` | Evaluate an existing profile. `-o html`, `--uncovered`, `--diff <ref>`, `--merge <profile>`. |
| `detect` | Auto-detect domains and write config. `--dry-run` to preview. |
| `badge` | SVG coverage badge. `--style flat-square`. |
| `compare` | Diff two profiles. |
| `debt` | Coverage debt report. |
| `trend` | Coverage trend from recorded history. |
| `record` | Append current coverage to history. `--commit`, `--branch` for CI. |
| `suggest` | Threshold suggestions. `--write-config` to apply. |
| `pr-comment` | Post coverage to GitHub/GitLab/Bitbucket PR. |
| `ignore` | Show configured excludes and tracked domains. |
| `mcp serve` | Start MCP server (stdio). `--mode=agent\|ci\|auto`. |
| `mcp doctor` | First-run validation: PASS/FAIL per step with remediation. |
| `survey` | Sean Ellis 40% PMF prompt; appends to `~/.coverctl/survey.jsonl`. |

Global flags: `-q/--quiet`, `--no-color`, `--ci` (combines quiet + GitHub Actions annotations).

### Test-execution flags

`check`, `run`, `record` accept toolchain flags forwarded to the underlying test runner:

| Flag | Example |
| --- | --- |
| `--tags` | `--tags integration,e2e` |
| `--race` | (Go race detector) |
| `--short` | Skip long-running tests |
| `-v` | Verbose test output |
| `--run` | `--run TestFoo` |
| `--timeout` | `--timeout 30m` |
| `--test-arg` | Repeatable: `--test-arg=-count=1 --test-arg=-parallel=4` |
| `--language` / `-l` | Override autodetection: `go`, `python`, `nodejs`, `rust`, `java`, ... |

### Terminal flow (without an agent)

If you prefer running coverctl directly:

```bash
coverctl init      # auto-detect language + domains, write .coverctl.yaml
coverctl check     # enforce policy; exit 1 on violation
coverctl suggest --strategy current
coverctl record
```

If a step fails: `coverctl detect --dry-run` previews `init` output; `coverctl check -o json` surfaces structured failure detail; `coverctl record --commit "$(git rev-parse HEAD)" --branch "$(git rev-parse --abbrev-ref HEAD)"` provides metadata explicitly in CI.

## Configuration

`.coverctl.yaml` (schema: [`schemas/coverctl.schema.json`](schemas/coverctl.schema.json)):

```yaml
version: 1
policy:
  default:
    min: 75
  domains:
    - name: auth
      match: ["./internal/auth/..."]
      min: 90       # critical path — stricter
    - name: api
      match: ["./internal/api/..."]
      min: 80
    - name: utils
      match: ["./internal/utils/..."]
      # falls back to default min: 75
exclude:
  - internal/generated/*
```

Per-domain enforcement is the point: overall coverage hides regressions in critical paths. coverctl evaluates each domain against its own minimum and fails the build if any domain falls below.

### Advanced

```yaml
files:
  - match: ["internal/core/*.go"]
    min: 90                          # per-file overrides
diff:
  enabled: true
  base: origin/main                  # only enforce on changed files
integration:
  enabled: true                      # Go 1.20+ GOCOVERDIR integration tests
  packages: ["./internal/integration/..."]
  cover_dir: ".cover/integration"
  profile: ".cover/integration.out"
merge:
  profiles: [".cover/unit.out", ".cover/integration.out"]
annotations:
  enabled: true                      # // coverctl:ignore, // coverctl:domain=NAME
```

Multi-package monorepo? Use `extends:` for inherited policies. Starting point: copy `templates/coverctl.yaml`.

## Supported languages

| Language | Format | Detection markers |
| --- | --- | --- |
| Go | Native cover profile | `go.mod`, `go.sum` |
| Python | Cobertura, LCOV | `pyproject.toml`, `setup.py`, `requirements.txt` |
| TypeScript / JavaScript | LCOV | `tsconfig.json`, `package.json` |
| Java | JaCoCo, Cobertura | `pom.xml`, `build.gradle` |
| Rust | LCOV (cargo-llvm-cov) | `Cargo.toml` |
| C# / .NET | Cobertura (coverlet) | `*.csproj`, `*.sln` |
| C / C++ | LCOV (gcov/lcov) | `CMakeLists.txt`, `meson.build` |
| PHP | Cobertura (PHPUnit) | `composer.json`, `phpunit.xml` |
| Ruby | LCOV (SimpleCov) | `Gemfile`, `Rakefile` |
| Swift | LCOV (llvm-cov) | `Package.swift` |
| Dart | LCOV (dart test) | `pubspec.yaml` |
| Scala | Cobertura (scoverage) | `build.sbt` |
| Elixir | LCOV (mix test) | `mix.exs` |
| Shell | Cobertura (kcov) | `*.bats` |

## GitHub Action

```yaml
jobs:
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: "1.25"
      - uses: ./.github/actions/coverctl
        with:
          command: check
          config: .coverctl.yaml
          output: text
```

## For platform & devex teams

Standardizing coverage policy across many polyglot repos? coverctl ships with the artifacts your security and procurement reviews will ask for:

- [MCP threat model](docs/security/mcp-threat-model.md) — input + output boundary controls, prompt-injection defense, residual risks.
- [Rejection schema reference](docs/src/content/docs/mcp.mdx) — stable `error_code` + `remediation` contract for agent recovery.
- [GTM funnel metrics spec](docs/design/gtm-metrics-spec.md) — what's instrumented and how telemetry stays opt-in.

Considering coverctl org-wide? Open a GitHub issue with label `platform-evaluation` — happy to walk through architecture and trust boundaries.

## Community

- **Claude Code Plugin Marketplace** — install `coverctl` directly via `/plugin install`.
- **MCP Registry** — listed at [registry.modelcontextprotocol.io](https://registry.modelcontextprotocol.io).
- **GitHub Discussions** — [go.klarlabs.de/coverctl/discussions](https://github.com/klarlabs-studio/coverctl/discussions).
- **Sponsor coverctl development** — [github.com/sponsors/felixgeelhaar](https://github.com/sponsors/felixgeelhaar).

Used coverctl for a few weeks? Run `coverctl survey` to share PMF feedback (local-only by default; nothing transmitted unless you opt in).

Built by Felix Geelhaar with contributions from the polyglot AI-coding community.

## Contributing

- TDD: tests before behavior changes.
- Coverage ≥80% (`go test ./... -cover`).
- Conventional Commits (`feat:`, `fix:`, `chore:`, ...) for Relicta version-bump logic.
- `main` is protected; merge via PR after CI green (`.github/workflows/go.yml` + `.github/workflows/eval.yml`).
- Run `gofmt -w` and `golangci-lint v2` before pushing.

Architecture details for contributors: [ARCHITECTURE.md](ARCHITECTURE.md).

## Releases

Managed by [Relicta](https://github.com/felixgeelhaar/relicta). Do not push `v*` tags manually.

## Security

See [SECURITY.md](SECURITY.md) for disclosure policy. MCP-input sanitization (`internal/mcp/sanitize.go`) and output canonicalization (`internal/mcp/sanitize_output.go`) are the primary defenses against prompt-injection-driven argument and content attacks; report bypasses privately.
