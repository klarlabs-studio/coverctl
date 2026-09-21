# coverctl — System Intent

Canonical product and architecture intent. If another document
contradicts this one, this one wins. Category narrative lives in
`docs/strategy/category-pov.md`. Monetization lives in
`docs/strategy/monetization-decision.md`. This file is the
inward-facing contract for what coverctl is allowed to become.

---

## Purpose

coverctl is a coverage governance layer for software agents and CI systems.

Its purpose is not to replace language-specific coverage tools.

Tools such as `go test`, pytest, Jest, JaCoCo, llvm-cov, coverage.py, and
their equivalents remain responsible for executing tests and producing
coverage data.

coverctl sits above those tools and provides a consistent answer to a
different set of questions:

* Does this change satisfy the project’s coverage policy?
* Which changed code is insufficiently tested?
* Why did the coverage policy fail?
* What should be tested next?
* Is existing coverage debt increasing or decreasing?
* Can an autonomous agent verify its work without bypassing project policy?

The core product is the governance loop:

```
change
  ↓
check
  ↓
explain
  ↓
suggest / debt
  ↓
edit tests
  ↓
check again
```

Everything in coverctl should make this loop safer, faster, more
deterministic, or more useful.

---

## Product Intent

coverctl is primarily an agent-native coverage governor.

Traditional CLI and CI workflows remain important interfaces, but they
are not the source of differentiation.

The differentiated capability is enabling software agents to autonomously
reason about and satisfy coverage requirements through a small,
deterministic interface.

The primary agent capabilities are:

* `check`
* `suggest`
* `debt`

These capabilities should allow an agent to:

1. determine whether its change satisfies coverage policy;
2. understand precisely why a policy failed;
3. identify the highest-value missing tests;
4. distinguish new coverage regressions from existing debt;
5. verify that its remediation fixed the problem.

New product functionality should strengthen this loop before expanding
the general-purpose CLI surface.

---

## Architectural Intent

coverctl follows an inward dependency model:

```
interfaces
    ↓
application
    ↓
domain
```

Infrastructure implements ports defined by the inner layers.

The domain model must remain independent of:

* CLI frameworks;
* MCP;
* operating systems;
* coverage tools;
* programming languages;
* build systems;
* package managers;
* CI providers.

Language-specific behavior belongs at the infrastructure boundary.

Adding another language or coverage tool should not require changes to
the coverage policy engine.

---

## Core Domain

The durable core of coverctl consists of:

* Coverage Model
* Policy Engine
* Diff Model
* Debt Model
* Remediation Model

These concepts should remain language-agnostic.

Coverage profiles from different ecosystems are normalized into the same
internal representation before policy evaluation occurs.

```
Go coverage ──────┐
Cobertura ────────┤
JaCoCo ───────────┤
LCOV ─────────────┼──▶ Coverage Model ──▶ Policy Engine
LLVM coverage ────┤
other formats ────┘
```

Parsers translate.

Policies decide.

Runners execute.

These responsibilities should not leak into one another.

---

## Runner Intent

Runner adapters exist to integrate existing test and coverage ecosystems.

They are not the product’s domain model.

A runner is responsible for:

```
typed execution request
        ↓
tool-specific invocation
        ↓
coverage artifact
        ↓
normalized coverage model
```

Runner-specific behavior should remain isolated behind ports.

The number of supported languages must not increase complexity in the
policy engine.

Where ecosystems have multiple tools, package managers, or build
systems, those differences belong inside runner implementations or
runner-specific infrastructure.

Long term, runner extensibility may become external to the core binary
if doing so reduces compatibility burden without weakening security or
reproducibility.

---

## Agent Security Model

Agent input is untrusted.

Repository contents are untrusted.

Coverage profile contents are untrusted.

Coverage filenames and labels are untrusted.

Tool output returned to an agent is also untrusted.

Therefore MCP is a bidirectional trust boundary:

```
LLM
 │
 │ untrusted request
 ▼
┌────────────────────┐
│ capability boundary│
└─────────┬──────────┘
          │
          ▼
      coverctl
          │
          │ untrusted repository-derived data
          ▼
┌────────────────────┐
│ output boundary    │
└─────────┬──────────┘
          │
          ▼
         LLM
```

Both directions must be defended.

---

## Capability-Based Execution

Agent mode should not expose arbitrary subprocess argument construction.

The preferred interface is a set of typed capabilities such as:

* packages
* test pattern
* tags
* timeout
* race detection
* short mode
* coverage scope (CI/human only)

Each runner translates those capabilities into tool-specific arguments.

Coverage scope is policy-derived in agent mode. An agent must not
narrow measurement (pytest `--cov`, coverage.py `--source`, c8/nyc
`--include`, jest `--collectCoverageFrom`) because shrinking the
instrumented set can hide untested policy domains. Repository policy
domains define what is measured. Go `coverpkg` stays derived from
those domains rather than from a caller-supplied path list.

For example:

```
Agent request
{
  packages: ["./internal/..."],
  race: true,
  timeout: "2m"
}
        ↓
Go runner
go test -race -timeout 2m ./internal/...
```

The agent expresses intent, not shell or runner syntax.

This prevents coverctl from having to maintain an ever-growing
cross-language denylist of dangerous arguments.

Arbitrary runner arguments, where supported for human workflows, should
not automatically become available through agent-facing interfaces.

The desired invariant is:

> An agent can request supported testing capabilities but cannot express
> arbitrary code execution through coverctl.

---

## Determinism

Agent-facing operations should be deterministic whenever possible.

Given the same:

* repository state
* configuration
* coverage profile
* diff

coverctl should produce the same:

* policy result
* diagnostics
* debt calculation
* suggestions
* structured output

LLM inference must not be required for core coverage decisions.

Agents may reason about coverctl output.

coverctl itself should provide deterministic evidence.

---

## Structured Output

Machine-facing output is an API.

It must therefore be:

* structured;
* bounded;
* versionable;
* deterministic;
* safe to consume;
* semantically lossless.

Sanitization must not collapse distinct identifiers.

Paths, domain names, filenames, package names, and similar identifiers
should use reversible escaping or structured encoding rather than
destructive replacement.

For example:

`src/über.go`

must not become indistinguishable from another valid filename.

Prompt-injection resistance should come from structural boundaries,
bounded output, escaping, validation, and explicit data semantics — not
from unnecessarily destroying legitimate repository data.

---

## Policy Authority

Agents must not be able to weaken project policy simply by changing
invocation parameters.

Repository policy is authoritative.

An agent may ask:

* Does this change pass?
* Why does it fail?
* What should I test?
* What debt already exists?

It should not be able to silently transform:

```
minimum coverage = 90%
```

into:

```
minimum coverage = 0%
```

through tool arguments, a `domains` filter, or a narrowed
`coverageScope`.

Configuration precedence and mutation rules must preserve this principle.

---

## CI Security

CI configuration is executable supply-chain infrastructure.

Dependencies involved in CI execution should therefore be immutable
wherever practical.

Reusable workflows and third-party actions should be pinned to immutable
revisions rather than mutable branches or tags when they execute with
privileged permissions or secrets.

Secrets should be passed explicitly rather than inherited broadly
whenever practical.

The same standard applies to downloaded binaries:

```
version pinned
+
digest verified
```

The security posture of coverctl’s own development pipeline should
reflect the security guarantees expected from the product.

---

## Cross-Platform Intent

coverctl orchestrates operating-system-specific processes and filesystem
behavior.

Cross-platform behavior is therefore part of correctness even though the
core implementation is Go.

At minimum, CI should exercise representative behavior on:

* Linux
* macOS
* Windows

The full integration matrix does not need to run everywhere.

A reasonable model is:

```
Linux
  full suite
macOS
  CLI + process + path smoke tests
Windows
  CLI + process + path smoke tests
```

Platform-specific behavior should remain isolated behind infrastructure
boundaries.

---

## Testing Philosophy

Tests should protect behavior and architectural invariants.

Important invariants include:

* domain has no infrastructure dependencies
* policy evaluation is deterministic
* language-specific behavior cannot leak into policy
* agent capabilities cannot produce arbitrary execution
* repository-controlled strings cannot become instructions
* sanitization preserves semantic identity
* configuration cannot silently weaken policy
* runner failures produce actionable diagnostics

Architecture tests should primarily guard dependency direction and
forbidden coupling.

File size can be a useful maintenance signal, but it is not itself an
architectural boundary.

---

## Evaluation

Agent effectiveness is a first-class product metric.

coverctl should maintain deterministic evaluation scenarios covering
cases such as:

* agent changes covered code
* agent changes uncovered code
* coverage falls below policy
* existing debt is present
* new debt is introduced
* agent requests remediation
* agent adds appropriate tests
* agent re-runs verification
* malicious repository content is encountered
* malicious coverage metadata is encountered

Important questions include:

* Did the agent invoke `check` at the appropriate point?
* Did it correctly understand the failure?
* Did `suggest` lead it toward useful tests?
* Did it distinguish existing debt from new regression?
* Did it successfully verify its remediation?
* Could repository-controlled content manipulate the agent through
  coverctl output?

Improving these outcomes has higher priority than maximizing the number
of CLI commands.

---

## Product Expansion Rule

Before adding a feature, ask:

> Does this make the autonomous coverage-governance loop materially better?

Strong reasons include:

* the agent knows WHEN to check
* the agent understands WHY it failed
* the agent knows WHAT to test
* the agent cannot bypass policy
* the agent can VERIFY its remediation
* the result becomes more deterministic
* the trust boundary becomes stronger

Supporting another language can also be valuable when demanded by real
users, provided it does not compromise these principles.

Feature count is not a goal.

---

## Non-Goals

coverctl is not intended to become:

* a test framework;
* a replacement for language-native coverage tools;
* a universal build system;
* a package manager;
* a generic command-execution MCP server;
* an LLM-based test generator;
* an AI code-review system;
* a CI platform.

Existing ecosystems should perform those jobs.

coverctl should integrate them into a consistent coverage-governance
model.

---

## Desired End State

The conceptual architecture should remain small:

```
                Agent / CI / Human
                       │
                       ▼
               Typed Capabilities
                       │
                       ▼
              Coverage Application
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
     Policy         Coverage         Diff
     Engine          Model           Model
        ▲              ▲
        │              │
     Config          Parsers
                       ▲
                       │
                  Runner Ports
                       ▲
          ┌────────────┼────────────┐
          ▼            ▼            ▼
         Go          Python        Java ...
```

Everything above the runner boundary should remain language-agnostic.

Everything exposed to an autonomous agent should be:

typed, bounded, deterministic, policy-preserving, and incapable of
expressing arbitrary execution.

That is the architectural and product intent of coverctl.
