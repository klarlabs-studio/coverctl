# Changelog

All notable changes to `coverctl` will be documented here. Relicta manages this file automatically.

## [1.18.0] - 2026-07-06

Security & correctness hardening from a full deep review. The headline theme is
**closing fail-open paths** — cases where the coverage gate reported PASS when it
should have FAILED.

### Fixed
- **Gate fail-open: MCP `check` ignored `failUnder`/`ratchet`.** The overall-floor
  and no-regression gates were enforced only on the CLI path, so an agent calling
  the `check` tool with `ratchet: true` or `failUnder: N` got `passed: true` even
  on a regression. Enforcement is now shared between the CLI and the MCP surface. (#103)
- **Gate fail-open: negative statement counts inflated coverage.** A crafted/corrupt
  coverage profile with a negative statement count underflowed a domain's denominator
  (coverage > 100%), passing the gate. The parser now rejects negative counts. (#103)
- **Gate fail-open: diff-coverage skipped specially-named files.** `git diff` under
  the default `core.quotePath` C-quoted non-ASCII filenames, which dropped them from
  the changed-file set. Now uses `git diff -z` with verbatim, NUL-framed paths. (#104)
- **Gate fail-open: broken `**` glob left domains unenforced.** A pattern like
  `src/**/*.py` matched zero directories, silently unenforcing that domain. Rewrote
  `**` matching (doublestar semantics) and now fail closed on a zero-match domain. (#106)
- Threshold comparison used the display-rounded percentage; 79.95% rounded up and
  passed an 80% gate. The raw percentage is now compared; rounding is display-only. (#103)
- `--ratchet` silently passed when the coverage history could not be read; it now
  fails closed on a baseline-load error. (#103)
- Bitbucket PR comments could duplicate because only the first page of existing
  comments was scanned; pagination is now followed. (#107)
- Windows path matching: exclude and directory attribution now normalize separators,
  fixing fail-open mis-attribution on `\`-separated paths. (#109)

### Security
- Path containment enforced on the annotation scanner and config `extends` resolution,
  preventing reads of files outside the module root via a crafted coverage report or
  `.coverctl.yaml` (the legitimate monorepo `extends`/upward search still works). (#105)
- MCP server: panic-recovery middleware (one bad tool call no longer kills the stdio
  session); `suggest{writeConfig:true}` is read-only in agent mode (config writes
  require `--mode=ci`) so an agent cannot silently relax the gate; config-write targets
  are scope-validated; `check` has a default runtime cap. (#108)
- VCS clients: reject cross-host redirects (GitLab `PRIVATE-TOKEN` can no longer leak
  to an attacker host); path-escape `owner`/`repo` segments; bound response bodies. (#107)
- Parsers/runners: input-size caps on all coverage parsers, JaCoCo integer-overflow
  guard, timeouts on tool-detection subprocesses, and bounded child-process output. (#109)

## [1.13.0] - 2026-03-01

### Added
- **15-Language Coverage Support**: Expand from 5 to 15 supported languages
  - Add C#/.NET (Cobertura via coverlet)
  - Add C/C++ (LCOV via gcov/lcov)
  - Add PHP (Cobertura via PHPUnit)
  - Add Ruby (LCOV via SimpleCov)
  - Add Swift (LCOV via llvm-cov)
  - Add Dart (LCOV via dart test)
  - Add Scala (Cobertura via scoverage)
  - Add Elixir (LCOV via mix test)
  - Add Shell (Cobertura via kcov)
- **Language-Agnostic Parser Registry**: Multi-format parser registry for automatic format detection
- **Security Baseline**: Nox security scanner baseline with 570 triaged entries

### Changed
- **Security Scanning**: Replace VerdictSec with nox for security scanning in CI and docs

### Fixed
- Resolve SEC-085 vulnerability: remove URL-embedded credentials in Homebrew tap script
- Use credential store for Homebrew tap clone in CI
- Resolve gofmt formatting issues in cli and annotations
- Handle Go workspace in ModulePath resolution

## [1.12.0] - 2026-02-01

## [1.11.0] - 2026-01-09

### Added
- **DDD Domain Layer**: Extract domain value objects (`Threshold`, `DomainName`, `FilePath`, `Percentage`)
- **Domain Aggregates**: Add `PolicyAggregate` and `CoverageAggregator` for business logic encapsulation
- **Domain Services**: Add `TrendService` and domain events for event-driven architecture
- **Application Handlers**: Extract `Check`, `Report`, `Watch`, `History`, `Analytics`, `PRComment` handlers

### Changed
- **Documentation**: Update for multi-language support (Go, Python, JavaScript, Rust, Java)
- **Test Coverage**: Improve coverage across domain (85.5%), autodetect (86.3%), gotool (81.1%)

### Security
- Fix command injection vulnerability in nodejs runner
- Fix path traversal vulnerability in history store
- Improve input validation in API clients (GitHub, GitLab, Bitbucket)

## [1.10.0] - 2026-01-07

### Added
- **Multi-Language Support**: Add coverage runners for Python, JavaScript, Rust, and Java
- **Auto-Detection**: Automatically detect project language from config files
- **Multi-Provider PR Comments**: Support for GitHub, GitLab, and Bitbucket PR annotations

## [1.7.1] - 2025-12-25

### Changed
- Lower Go version requirement from 1.25.5 to 1.24.0
- Update mcp-go to v1.1.0

## [1.7.0] - 2025-12-25

### Changed
- **MCP Server**: Migrate from `modelcontextprotocol/go-sdk` to `mcp-go` library
  - Fluent builder API for cleaner tool/resource registration
  - Simplified handler signatures (removed unused parameters)
  - Automatic EOF/graceful shutdown handling
  - Go version updated to 1.25.5

## [1.6.0] - 2025-12-25

### Added
- **MCP Server**: Add Model Context Protocol server via `coverctl mcp serve` for AI agent integration
  - Tools: `check`, `report`, `record` for programmatic coverage operations
  - Resources: `coverctl://debt`, `coverctl://trend`, `coverctl://suggest`, `coverctl://config`
  - STDIO transport for Claude Desktop and other MCP-compatible clients

### Fixed
- Correct jsonschema tag format for MCP SDK compatibility

## [1.5.0] - 2025-12-24

### Added
- **Brief output format**: `--output brief` for single-line LLM/agent-optimized output

## [1.4.0] - 2025-12-23

### Added
- **HTML coverage reports**: Generate styled HTML reports with `-o html` flag
- **Severity levels**: Add `warn` threshold for domains (WARN status between min and warn)
- **Badge generation**: `coverctl badge` command generates SVG coverage badges
- **Coverage trend tracking**: `coverctl trend` and `coverctl record` for historical analysis
- **Threshold suggestions**: `coverctl suggest` recommends optimal thresholds
- **Coverage delta**: `--show-delta` flag displays coverage changes from history
- **Domain-specific excludes**: Per-domain `exclude` patterns for fine-grained control
- **Watch mode**: `coverctl watch` for continuous coverage on file changes
- **Coverage debt report**: `coverctl debt` shows coverage shortfall and remediation effort
- **Integration coverage**: Support for Go 1.20+ binary coverage with `GOCOVERDIR`
- **Profile merging**: Combine multiple coverage profiles for unified analysis
- **Diff-based checks**: Enforce coverage only on changed files with `diff.enabled`
- **File-level rules**: Per-file minimum thresholds with `files` config
- **Annotations**: `// coverctl:ignore` and `// coverctl:domain=NAME` pragmas

## [0.1.0] - 2024-01-01
- initial scaffolding of the CLI, DDD layers, report tooling, and documentation
- strict DDD/TDD guidance with coverage enforcement and Relicta release configuration
