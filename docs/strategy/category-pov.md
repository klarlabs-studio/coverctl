# coverctl Category Point-of-View

A category-design POV (Lochhead, *Play Bigger*). It defines the world we want
buyers to see before they evaluate any specific product. It seeds README hero
copy, content calendar topics, and conference pitch language. If a piece of
positioning copy contradicts this document, this document wins.

## 1. The world today

AI coding agents — Claude Code, Cursor, Cline, Copilot — now write a large
share of code in real codebases. They edit faster than humans can review and
ship code into branches, PRs, and merges with minimal in-loop quality
feedback.

Coverage policy is still designed for the human cycle: write code, push,
wait for CI, read the dashboard, fix what broke. That cycle takes minutes to
hours. Inside an agent edit loop it takes seconds. The result is a structural
mismatch:

- Agents ship blind to coverage policy while editing.
- CI sees the regression long after the agent's context is gone.
- Polyglot codebases compound the problem: `go test -cover`, `pytest --cov`,
  `nyc`, `cargo tarpaulin`, JaCoCo, and a dozen others each have their own
  invocation, output format, and threshold conventions. There is no uniform
  governance interface.
- Dashboards (Codecov, Coveralls, SonarQube) report after merge. They never
  enter the agent's edit loop.

The status quo for AI-assisted polyglot teams is the **red-CI agent loop**:
agent edits → human runs tests → CI fails minutes later → human pastes the
error back to the agent → agent guesses → repeat. Trust in agent output
erodes. PR rework expands. Coverage drifts down because nobody is enforcing
it where decisions are being made.

## 2. The world we are describing

Coverage governance moves into the loop where decisions are made.

- Agents *call* coverage as a tool, in the same edit cycle they call file
  reads or test runs. Coverage is no longer a dashboard humans check after
  the fact; it is a signal agents respond to before commit.
- One uniform governance interface spans every language the team uses. The
  policy file looks the same whether the repo is Go, Python, TypeScript,
  Java, Rust, or any combination. Domain-aware thresholds, not project-wide
  averages.
- Quality signals are agent-callable through MCP — the emerging standard
  substrate for tool integration in AI coding clients.
- Local-first execution. Agents and developers get the same fast feedback
  on the same machine, without round-tripping a SaaS dashboard.
- Security at the input boundary: untrusted MCP arguments are sanitized
  before they reach language toolchains, and untrusted output (filenames,
  test names, profile contents) is canonicalized before flowing back into
  agent context.

In this world, the question is not "what is our coverage percentage?" — it
is "did the agent ship a regression we missed?" That is a different
question, with a different answer, and it requires a different category of
tool to answer.

## 3. Why now (three forces)

1. **AI coding adoption inflection.** Claude Code, Cursor, and Cline moved
   from experimental in 2024 to default-on in 2025–2026 for a meaningful
   share of polyglot teams. The volume of agent-authored code in production
   is past the point where post-hoc CI dashboards can catch what slips
   through.
2. **MCP as the standard substrate.** MCP went from Anthropic-only in late
   2024 to multi-vendor governance with OpenAI, Google, Microsoft, and AWS
   in 2025–2026. Tool surfaces that target MCP today reach every major
   AI coding client tomorrow. Tools that don't are stuck in proprietary
   integrations.
3. **Polyglot pain compounds.** Per-language coverage tooling has not
   converged in a decade and will not. Teams using AI agents across
   multiple languages simultaneously experience the per-language tool
   sprawl as a single cognitive cost. They are ready to standardize the
   governance layer above the language tools.

## 4. The category

**Agent-loop coverage governance.**

The product class that runs coverage where the agent works, when the agent
works, on the agent's call. The governance interface is uniform across
languages. The integration substrate is MCP. The execution model is
local-first. The metric of success is *regressions caught before commit,
in the agent's loop* — not dashboard impressions or post-merge alerts.

This category does not yet exist. Coverage tools today are dashboards.
CI tools today are pipelines. Pre-commit tools today are language-specific
hooks. None of them sit where agents actually edit. The category is open.

## 5. Who benefits most

The early customer profile (ECP, first 20 lighthouses):

- Engineering teams of 5–80 developers
- Polyglot codebase: at least two languages in active development
- Active adoption (last 90 days) of Claude Code, Cursor, or Cline
- At least one CI policy gate already in place (any kind: lint, test, type)
- Felt CI-red-surprise pain in the last sprint — coverage drop that
  reached PR review or merge

Compliance-sensitive paths (auth, payments, data pipelines, healthcare, fintech)
are an **accelerator, not a gate**. They accelerate sales velocity and
willingness-to-pay once the wedge is established; they do not disqualify
ECP candidates.

The expansion ICP (after first 20 customers):

- Platform / DevEx teams in larger organizations standardizing coverage
  policy across many repos. They become the procurement champion. The
  threat-model artifact (`docs/security/mcp-threat-model.md`) is the
  enabling material.

## 6. What this POV is not

This POV deliberately excludes:

- "A better Codecov." Coverage dashboards are a different category solving
  a different problem (post-hoc reporting). Competing on Codecov's terms
  is the wrong battle.
- "Multi-language coverage." Multi-language is table stakes for the
  category, not the wedge.
- "AI-powered coverage analysis." We are not generating tests with AI.
  We are giving AI agents a deterministic governance signal they can act
  on.

## 7. Implication for messaging

The single hero question every page should answer in the first paragraph:

> Are coding agents in your repo shipping coverage regressions you only
> see in CI?

If the visitor's mental answer is "yes" or "I don't know," they are in the
ECP and the page should drive them toward the agent-mode quick-start. If
the answer is "no, we don't use AI agents," they are not the primary ECP
and the page should hand them off to terminal-mode quick-start without
friction (still a real audience, just not the wedge).

## 8. Drives

- README hero copy
- Astro/Starlight landing page hero (`docs/src/content/docs/index.mdx`)
- Plugin marketplace listing
- Conference talk pitches
- First five content articles (each tied to one section of this POV)
- Future positioning A/B tests (Codecov-frame vs agent-loop-frame)

## 9. Revision policy

This document is revised when one of three signals fires:

1. The wedge metric (Weekly Protected Agent Loops) trends materially.
2. A new MCP-native competitor enters the agent-loop coverage category.
3. The ECP definition is invalidated by repeated dogfood data showing the
   ECP is too narrow or too broad.

Otherwise it stays stable. Category narratives compound when consistent.
