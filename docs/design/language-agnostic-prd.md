# Product Requirements Document: Agent-Loop Coverage Governance

## Initiative Hypothesis

> We believe that giving polyglot AI-coding teams MCP-native, domain-aware
> coverage feedback **in the agent edit loop**
> Will result in a measurable pre-commit regression catch rate ≥80%
> For teams using Claude Code, Cursor, or Cline on repos with at least
> one CI policy gate today
> We will know this is true when ≥40% of weekly-active repos block at
> least one regression per 7 days via MCP tool calls
> We will know this is false when median agent session shows zero
> coverctl tool calls despite eligible edits
> Our riskiest assumption is that agents will *autonomously* invoke
> `check` without prompting, often enough on the right edits, to catch
> regressions.

The category framing for this work is **agent-loop coverage governance**
(see `docs/strategy/category-pov.md`). The wedge is in-loop coverage
feedback before commit. Multi-language support is table stakes; the
differentiated capability is being callable from the agent edit loop
through MCP, with a uniform governance interface across languages.

---

## Problem

The status quo for AI-assisted polyglot teams is the **red-CI agent loop**:
agent edits code → human runs tests locally or in CI → CI fails minutes
later → human pastes the error back to the agent → agent guesses → repeat.
Coverage policy lives in dashboards and post-merge reports, never inside
the loop where the agent is actually deciding what to write.

This produces three concrete pains:

1. **Agents ship coverage regressions blind.** The agent has no signal
   that an edit dropped a domain below threshold until CI surfaces it,
   long after context is lost.
2. **Polyglot tool sprawl.** Per-language coverage tools (`go test
   -cover`, `pytest --cov`, `nyc`, `cargo tarpaulin`, JaCoCo) have no
   uniform invocation, output format, or threshold convention. Every
   language is a separate governance integration.
3. **PR rework expands.** Red-CI surprises lead to repeated PR cycles,
   and coverage drifts down over time because nobody is enforcing it
   where decisions are being made.

---

## Shipped capability (foundation)

The product reached language-agnostic coverage as a foundation for the
wedge above:

- **15 languages**: Go, Python, TypeScript/JavaScript, Java, Rust, C#,
  C/C++, PHP, Ruby, Swift, Dart, Scala, Elixir, Shell
- **Multi-format parsing**: LCOV, Cobertura XML, JaCoCo XML, Go native
  profiles
- **MCP-native**: stdio MCP server with 8 tools and 4 resources for AI
  agent integration
- **Domain-aware policy**: per-domain coverage thresholds via
  `.coverctl.yaml`
- **Backward compatible**: existing Go workflows unchanged

Multi-language is treated here as **a delivered prerequisite**, not the
product story. The story is what comes next.

---

## Success metrics

### North Star

**Weekly Protected Agent Loops** = unique repos where `coverctl check`
ran in agent or MCP context within the last 7 days **and** blocked at
least one regression. Captures real usage, agent context, and value
delivered. Cannot be moved by shipping more features alone.

### Input metrics (the four levers)

| Metric | Definition | Why it matters |
|---|---|---|
| **Time-to-first-protected-commit** | Median minutes from `coverctl init` completion to first agent-initiated `check` that returns `passed=true`. | Activation. If this is high, the wedge cannot fire. |
| **MCP tool-call success rate** | (successful calls / total calls) × 100 across `check`, `suggest`, `debt`. Target >85%. | Quality of the agent-callable surface. Defined in `docs/design/mcp-metrics-spec.md`. |
| **Pre-commit hook adoption** | % of repos with `coverctl check` wired into a pre-commit hook (Husky, lefthook, native git hook). | Depth of integration; lock-in proxy. Without the hook, the wedge depends entirely on agent autonomy. |
| **Regressions caught per agent session** | Eval-harness measured: scripted Claude Code session over N synthetic regression scenarios. Numerator denominator both controlled. | Direct measurement of the wedge value claim. |

### Why the prior metrics were retired

The prior success table tracked "Languages Supported (5+)", "Plugin
Installs (1,000+)", "Go User Retention (100%)", "Setup Time (<2 min)",
"GitHub Stars (+500)". Those are vanity or proxy metrics — installs and
stars do not measure value delivered, language count is a shipped
prerequisite, and Go retention is a binary protective constraint rather
than a forward-looking signal. The four input metrics above replace
them.

---

## Personas

The product targets **one primary user** and **one secondary buyer**.
Earlier persona drafts included four archetypes; that breadth diluted
focus. Two pruned personas align every PRD decision against the wedge.

### Primary user — Taylor (AI-coding polyglot developer)

- Uses Claude Code, Cursor, or Cline daily across at least two
  languages
- Has felt CI-red-surprise pain in the last sprint
- Wants the agent to catch coverage regressions before commit, not
  after CI
- Trusts tooling that returns deterministic, agent-readable signals
- Decision authority for tool adoption inside their team
- **Value coverctl delivers:** in-loop coverage feedback the agent
  actually calls

### Secondary buyer — Jordan (Platform / DevEx team lead)

- Manages 20–200-developer organization with polyglot codebases
- Standardizes coverage policy across many repos
- Procurement champion in larger organizations; cares about audit
  logging, security boundary, compliance evidence
- Activates coverctl org-wide once Taylor proves the wedge
- **Value coverctl delivers:** uniform governance interface across
  languages, with security architecture artifacts already documented
  (`docs/security/mcp-threat-model.md`)

### Compat-and-breadth notes

Earlier personas (Alex the legacy Go developer, Sam the Python/TS
generalist) are **not** primary segments. Existing Go users represent a
backward-compatibility constraint, not a growth segment — covered under
NFR1 below. Generic polyglot developers without an AI-agent workflow are
served by the terminal quick-start path and are a real audience for the
free CLI, but they are not the wedge entry point. Compliance-sensitive
organizations are an **expansion accelerator**, not a gate; they sell
faster and pay more once Taylor and Jordan are established.

---

## Feature Requirements

### Phase 1: Multi-Format Profile Analysis (MVP)

**Priority:** P0 (Must Have)
**Effort:** ~5 days

#### FR1.1: LCOV Format Parser
Support parsing LCOV format (`lcov.info`, `coverage.lcov`):
- Used by: pytest-cov, nyc/c8, Jest, Ruby, PHP, GCC/LLVM
- Parse `SF:`, `DA:`, `LF:`, `LH:` directives
- Map to internal `domain.CoverageStat` structure

#### FR1.2: Cobertura XML Parser
Support parsing Cobertura XML format:
- Used by: Java (Maven/Gradle), Python (coverage.py), .NET, many CI tools
- Parse `<package>`, `<class>`, `<line>` elements
- Handle both DTD versions (coverage.py vs Java)

#### FR1.3: Format Auto-Detection
Automatically detect coverage format from file:
- Sniff file headers (mode: for Go, `<?xml` for XML, `TN:` for LCOV)
- Use file extension as hint (`.out`, `.info`, `.xml`)
- Fall back to explicit `format:` config field

#### FR1.4: Configuration Extension
Extend `.coverctl.yaml` schema:
```yaml
version: 2
language: auto  # or: go, python, typescript, java, rust
profile:
  format: auto  # or: go, lcov, cobertura, jacoco
  path: coverage.out
```

#### FR1.5: Language Auto-Detection
Detect project language from markers:
- Go: `go.mod`
- Python: `pyproject.toml`, `setup.py`, `requirements.txt`
- TypeScript/JS: `package.json`, `tsconfig.json`
- Java: `pom.xml`, `build.gradle`
- Rust: `Cargo.toml`

### Phase 2: Language-Specific Runners (Optional)

**Priority:** P1 (Should Have)
**Effort:** ~10 days

#### FR2.1: Runner Interface
Define abstract runner interface:
```go
type CoverageRunner interface {
    Run(ctx context.Context, opts RunOptions) (profilePath string, err error)
    Name() string
    Detect() bool
}
```

#### FR2.2: Python Runner
Execute `pytest --cov` with appropriate flags:
- Auto-detect pytest-cov or coverage.py
- Generate LCOV or XML output
- Pass through test patterns and markers

#### FR2.3: Node.js Runner
Execute coverage tools:
- Support nyc, c8, Jest --coverage
- Auto-detect from package.json scripts
- Generate LCOV output

#### FR2.4: Rust Runner
Execute `cargo tarpaulin` or `cargo llvm-cov`:
- Generate LCOV output
- Handle workspace configurations

#### FR2.5: Java Runner
Execute Maven/Gradle with JaCoCo:
- `mvn jacoco:report` or `gradle jacocoTestReport`
- Parse JaCoCo XML output

### Phase 3: Claude Code Plugin

**Priority:** P0 (Must Have)
**Effort:** ~3 days

#### FR3.1: Plugin Manifest
Create `.claude-plugin/plugin.json`:
```json
{
  "name": "coverctl",
  "description": "Universal domain-aware coverage enforcement",
  "keywords": ["coverage", "testing", "go", "python", "typescript", "java", "rust"]
}
```

#### FR3.2: Slash Commands
- `/coverctl:check` - Run coverage check
- `/coverctl:report` - Analyze existing profile
- `/coverctl:suggest` - Get threshold recommendations

#### FR3.3: Skills
- `coverage-enforcement` - Auto-invoke during TDD
- `coverage-review` - Activate during PR reviews

#### FR3.4: MCP Integration
Bundle existing MCP server within plugin.

---

## Non-Functional Requirements

### NFR1: Backward Compatibility
- All existing Go workflows MUST continue working unchanged
- Existing `.coverctl.yaml` files (version 1) MUST be supported
- CLI commands and flags MUST remain stable

### NFR2: Performance
- Profile parsing: < 100ms for 10,000 files
- Format detection: < 10ms
- Language detection: < 50ms

### NFR3: Error Messages
- Clear error when unsupported format detected
- Helpful suggestions for missing dependencies
- Language-specific troubleshooting guidance

### NFR4: Documentation
- README with examples for each language
- Language-specific quick-start guides
- Migration guide for existing users

---

## User Stories

### Epic 1: Profile Analysis (Phase 1)

```
US1.1: As a Python developer, I want to analyze my pytest-cov LCOV output
       so that I can enforce domain-level coverage policies.

US1.2: As a Java developer, I want to analyze my JaCoCo XML report
       so that I can enforce coverage thresholds per package.

US1.3: As a polyglot developer, I want coverctl to auto-detect my coverage format
       so that I don't need to specify it manually.

US1.4: As an existing Go user, I want my workflow to remain unchanged
       so that I don't need to update any scripts or configs.
```

### Epic 2: Test Runners (Phase 2)

```
US2.1: As a Python developer, I want coverctl to run pytest with coverage
       so that I have a single command for enforcement.

US2.2: As a TypeScript developer, I want coverctl to run my coverage tool
       so that I don't need to manage multiple commands.

US2.3: As a CI engineer, I want coverctl to work with any language in my monorepo
       so that I can standardize my pipeline.
```

### Epic 3: Plugin Distribution (Phase 3)

```
US3.1: As a Claude Code user, I want to install coverctl with one command
       so that I can quickly add coverage enforcement.

US3.2: As a developer, I want coverctl to activate automatically during TDD
       so that I get continuous coverage feedback.

US3.3: As a team lead, I want to share coverctl configuration via plugin
       so that my team has consistent coverage policies.
```

---

## Acceptance Criteria

### Phase 1 MVP

- [ ] `coverctl report --profile lcov.info` works for LCOV files
- [ ] `coverctl report --profile coverage.xml` works for Cobertura XML
- [ ] `coverctl init` detects Python/TypeScript/Java/Rust projects
- [ ] Existing Go workflows pass all regression tests
- [ ] Config schema supports `language` and `profile.format` fields
- [ ] Error messages guide users to correct format/configuration

### Phase 2 Runners

- [ ] `coverctl check` runs `pytest --cov` for Python projects
- [ ] `coverctl check` runs `npm test -- --coverage` for Node.js
- [ ] `coverctl check` runs `cargo tarpaulin` for Rust
- [ ] Runner auto-detection works based on project markers
- [ ] All runners produce analyzable coverage profiles

### Phase 3 Plugin

- [ ] Plugin installable via `/plugin install coverctl`
- [ ] Slash commands work for all supported languages
- [ ] Skills activate appropriately based on context
- [ ] Plugin listed in Claude Code marketplace

---

## Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Breaking Go compatibility | High | Low | Extensive regression testing |
| Coverage format variations | Medium | High | Test with real-world samples |
| Runner dependency issues | Medium | Medium | Document prerequisites clearly |
| Plugin rejection | Medium | Low | Follow Anthropic guidelines |
| Scope creep to more languages | Low | High | Strict phase boundaries |

---

## Timeline

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| Phase 1: Profile Analysis | 1 week | LCOV + Cobertura parsers, auto-detection |
| Phase 2: Runners | 2 weeks | Python, Node.js, Rust, Java runners |
| Phase 3: Plugin | 1 week | Claude Code plugin, marketplace submission |

**Total:** ~4 weeks for full implementation

---

## Out of Scope

1. **IDE Plugins:** VS Code, JetBrains extensions (future consideration)
2. **Custom Format Support:** User-defined format parsers
3. **Remote Coverage:** Fetching coverage from CI systems
4. **Coverage Visualization:** Web UI dashboard (use existing HTML report)
5. **Proprietary Formats:** Coveralls, Codecov native formats

---

## Appendix

### Supported Coverage Formats

| Format | File Extensions | Languages |
|--------|-----------------|-----------|
| Go Coverage | `.out` | Go |
| LCOV | `.info`, `.lcov` | Python, JS/TS, Ruby, PHP, C/C++ |
| Cobertura XML | `.xml` | Java, Python, .NET |
| JaCoCo XML | `.xml` | Java, Kotlin |
| LLVM-cov JSON | `.json` | Rust, C/C++ |

### Competitive Analysis

| Tool | Languages | Domain-Aware | AI Integration | Plugin System |
|------|-----------|--------------|----------------|---------------|
| coverctl (shipped) | **15** (Go, Python, TS/JS, Java, Rust, C#/C++, PHP, Ruby, Swift, Dart, Scala, Elixir, Shell) | Yes | MCP-native | Claude Code |
| Codecov | Many | No | No | No |
| Coveralls | Many | No | No | No |
| SonarQube | Many | Partial | No | Yes |
