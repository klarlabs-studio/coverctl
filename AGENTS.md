# Repository Guidelines

## Project Structure & Module Organization
- `cmd/coverctl/` hosts the CLI entry point and wires infrastructure to application services.
- `internal/domain/`, `internal/application/`, `internal/infrastructure/` follow DDD: domain models contain rules, application orchestrates use cases, infrastructure provides adapters (config, Go tooling, reporting, coverprofile parsing).
- `schemas/` defines the `.coverctl.yaml` schema; relocate docs and fixtures near the packages they describe.
- Keep generated folders (proto, mocks) tracked via the `exclude` list and review them with `coverctl ignore` before running policies.

## Build, Test, and Development Commands
- `go test ./...` runs the full suite.
- `go test -covermode=atomic -coverpkg=./cmd/... -coverpkg=./internal/... -coverprofile=.cover/coverage.out ./...` mirrors the `coverctl check` instrumentation.
- `go run ./cmd/coverctl check` enforces each domain's minimum from `.coverctl.yaml` (default 80%; some packages set explicit floors — see that file); repeat it after changing code or tests.
- `go run ./cmd/coverctl init` launches the Bubble Tea wizard before writing `.coverctl.yaml`; pass `--no-interactive` for automation.
- `go run ./cmd/coverctl ignore` lists the `exclude` patterns contributors already documented.
- `relicta release --yes` (GitHub Action) builds the CLI artifact and publishes releases; see `.relicta.yaml` for details.

## Coding Style & Naming Conventions
- Format with `gofmt`, keep imports grouped, and prefer short, lowercase package names that mirror directories.
- Name domain concepts clearly (`PolicyEvaluator`, `CoverageReport`); use table-driven tests for branching logic.
- Keep CLI flags in `kebab-case` (e.g., `--output json`) and document them in README plus `.coverctl.yaml`.
- Inject infrastructure implementations via constructors so DDD boundaries stay intact.

## Testing Guidelines
- Follow TDD: extend or add tests before production code changes. Consult `docs/tdd.md` for examples.
- Use Go’s `testing` package; keep fixtures under `testdata/` and name files `*_test.go`.
- Meet each domain's `min` in `.coverctl.yaml` (default 80% when unset). Run `go run ./cmd/coverctl check` until every slice passes.

## Commit & Pull Request Guidelines
- Use Conventional Commits (`feat:`, `fix:`, `test:`) so release automation can categorize changes.
- PRs need a description, linked issue or PRD reference, tests run (especially `coverctl check`), and any relevant CLI output.
- Keep `main` protected; merge through PRs only once CI and policy checks succeed.

## Release & Automation Notes
- Relicta v2.6.1 drives releases; install via the GitHub action and keep `.relicta.yaml` aligned with CLI artifacts.
- Store the token as `RELICTA_TOKEN` with repo and workflow scopes; avoid `GITHUB_` prefixes.
- Relicta pushes tags only when they do not exist yet; do not tag manually before a release run.
