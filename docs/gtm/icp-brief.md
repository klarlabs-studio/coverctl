# coverctl ICP GTM Brief

This brief defines the customer profile, competitive frame, motion stack,
and proof metrics for coverctl. It pairs with `docs/strategy/category-pov.md`
(category narrative) and `docs/strategy/monetization-decision.md` (revenue
path). Where this document conflicts with the category POV, the POV wins.

## Wedge category

**Agent-loop coverage governance** — coverage that runs where the agent
works, when the agent works, on the agent's call. coverctl is positioned
inside this new category, not inside the dashboard-coverage category
where Codecov, Coveralls, and SonarQube already lead.

## Early Customer Profile (first 20 lighthouses — ECP)

Tight gate. Drop everything that is not in the wedge.

- Engineering teams of **5–80 developers**
- **Polyglot** codebase: at least two languages in active development
- **Active adoption (last 90 days)** of Claude Code, Cursor, or Cline
- At least one **CI policy gate** already in place (lint, test, type — any)
- **Felt CI-red-surprise pain** in the last sprint (a coverage drop or
  related quality regression that reached PR review or merge)

Compliance-sensitive paths (auth, payments, data pipelines) are **not**
an ECP gate. They are an expansion accelerator covered by the ICP below.

## Ideal Customer Profile (scaling segment — ICP)

Once the first 20 lighthouses validate the wedge, the ICP widens to
include compliance and platform-team buyers.

- All ECP traits, plus
- **At least one compliance-sensitive path** in the codebase (auth,
  payments, data pipelines, healthcare, fintech) — accelerates sales
  velocity and willingness-to-pay
- **Platform / DevEx team** ready to standardize coverage policy across
  many repos — becomes the procurement champion (Jordan persona in PRD)

## Problem (customer words)

> "Our AI agents ship code fast, but they're blind to coverage policy
> while editing. We only see breakage in CI, after context is gone, and
> the same agent then guesses its way to a fix."

## Why now

- AI-assisted coding adoption inflection (Claude Code, Cursor, Cline)
  has shifted the bottleneck from generation to **quality governance**.
- Polyglot codebases compound per-language tool sprawl into a single
  cognitive cost teams are ready to standardize.
- **MCP** is becoming the integration substrate for agent quality
  signals. Tool surfaces that target MCP today reach every major AI
  coding client tomorrow.

## Competitive alternatives (Dunford framing)

Listed in order of how often the ECP actually evaluates them.

1. **The red-CI agent loop (status quo).** Agent edits → human runs
   tests → CI fails minutes later → human pastes the error to the agent
   → agent guesses → repeat. This is the alternative coverctl
   *displaces*. It costs cycle time, erodes agent trust, and leaks
   regressions into PR review.
2. **Manual human PR review for coverage drops.** Catches some
   regressions but only after merge intent is already formed; high
   reviewer load, easy to miss subtle drops in non-obvious paths.
3. **Codecov / Coveralls / SonarQube dashboards.** Strong post-merge
   visibility, weak in-loop agent feedback. Different category solving
   a different problem.
4. **Hand-wired pre-commit hooks plus language-native commands**
   (`go test -cover`, `pytest-cov`, `nyc`, `cargo tarpaulin`).
   Fragmented per-language policy, no uniform domain-level enforcement,
   no agent-callable surface.

The first frame — the red-CI agent loop — is where the wedge lives.
Positioning copy that opens against Codecov/Coveralls cedes the category
fight before it begins.

## Differentiated capabilities

- **MCP-native** tool surface for agent-callable coverage checks
- **Domain-aware policy** (`.coverctl.yaml`) with per-domain thresholds
- **Multi-language runners and parsers** under one uniform interface
- **Security boundary** for MCP input and output (sanitization at the
  input boundary, canonicalization at the output boundary — see
  `docs/security/mcp-threat-model.md`)

## Differentiated value

- **Catch regressions before commit while the agent still has context.**
  The agent reads the failure, fixes the failure, and never leaves the
  edit loop.
- **Standardize policy across languages without switching tools.** One
  config, one governance interface, one mental model.
- **Reduce red-CI surprise loops and PR rework.** Cycle time
  compresses; coverage stops drifting.

## Positioning statement

For polyglot AI-coding teams that need coverage policy confidence in the
agent edit loop, coverctl is the agent-loop coverage governance tool
that gives agents an in-loop, domain-aware signal before commit, unlike
dashboard coverage products that report after CI has already failed.

## Motion stack

Two motions, run together. The third (enterprise) is deferred until
adoption proves out.

### Primary — Community-led

- Claude Code plugin marketplace listing
- MCP Registry presence
- GitHub stars, OSS contributions, public Discord/Slack presence in AI
  coding communities
- Cheap, compounds, exact channel where the ECP lives

### Secondary — Content-led

- "Agent-loop coverage" tutorials, failure-mode playbooks, regression
  case studies (drives the category POV into shared language)
- Conference pitches and developer-facing talks
- Defines POV; supports community-led discovery

### Deferred — Enterprise-led

Building enterprise sales motion (SOC 2, DPA templates, procurement
responses, security questionnaires) requires 6–12 months of investment
before first deal. Premature for current adoption stage. Keep the
threat-model artifact (`docs/security/mcp-threat-model.md`) ready for
inbound; do not market into enterprise outbound. Trigger condition is
defined in `docs/strategy/monetization-decision.md` Stage 4.

## First proof metrics (GTM funnel)

Distinct from product/usage metrics. These measure *whether the GTM is
working*, not whether the tool runs.

- **Activation rate** — `init` users reaching first passing `check`
- **30-day usage retention** — repos still calling `check` weekly 30
  days after install
- **Sean Ellis 40%** — % of users very-disappointed if coverctl went
  away (PMF signal)
- **Plugin marketplace install velocity** — installs/week trend
- **Unprompted advocate count** — mentions in Claude Code Discord, X,
  forums without prompting
- **Enterprise inbound** — procurement document requests (leading
  indicator for Stage 4 trigger)

The product/usage metrics (pre-commit catch rate, MCP tool-call success
rate, time-to-fix) live in the PRD success-metrics section, not here.

## Open assumptions to validate

- That ECP teams will adopt the agent-loop coverage category framing
  rather than evaluate coverctl as a Codecov alternative. Cheap test:
  A/B landing-page hero copy (Codecov-frame vs agent-loop-frame), 2
  weeks, measure activation by source.
- That community-led + content-led motions are sufficient to reach
  first 100 active repos without paid acquisition. Validation: 90-day
  install velocity trend after POV doc and category content land.
- That compliance-sensitive paths really do accelerate sales velocity
  in expansion (rather than just selecting for slow procurement). First
  five paying customers will tell us.
