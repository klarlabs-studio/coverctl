# coverctl Monetization Decision

A strategy decision document. It evaluates monetization paths, picks one,
and records the reasoning so future contributors can challenge the choice
on its merits rather than re-deriving it from scratch.

This is a **decision artifact, not an implementation plan**. Implementation
work for the chosen path lives downstream of this document in separate
roady features.

## Why now

Without a chosen monetization path, GTM investment compounds into community
without a path to sustainable funding. Two failure modes:

1. **Hobby drift.** Pure-OSS-without-sponsorship-strategy means maintainer
   burnout displaces feature velocity. Common pattern in single-maintainer
   developer tools after the first ~18 months.
2. **Premature enterprise.** Building enterprise SKU, SOC 2, procurement
   docs before community traction wastes capital on a buyer that has not
   shown up yet. Enterprise GTM motion costs roughly 6–12 months of
   maintainer time before first revenue.

Decision must precede major community-led GTM investment (per `gtm-expert`
review).

## Options evaluated

### Option A — Pure OSS + Sponsorship

**Mechanism.** Apache/MIT license. GitHub Sponsors, OpenCollective, or
similar. Optional feature donations from companies that adopt heavily.

**Pros.**
- No friction to adoption — community-led GTM compounds freely.
- No commercial-OSS confusion in marketplace listings.
- Aligned with developer-tool norms in the AI/agent ecosystem (most MCP
  servers are OSS today).

**Cons.**
- Sponsorship rarely funds full-time development. Typical OSS dev tool
  sponsorship: $500–$5K/month, far short of one FTE.
- No leverage to influence enterprise feature roadmap based on revenue
  signal.
- No defensible revenue line if a competitor enters the category.

**Verdict.** Floor option. Always available. Not sufficient as the only
path.

### Option B — Open Core (paid hosted history + team dashboards)

**Mechanism.** CLI and MCP server remain Apache/MIT. Paid hosted layer
(SaaS) provides:
- Coverage history persistence beyond local `.coverctl/history/`
- Team dashboards aggregating multiple repos
- Cloud MCP relay for organizations that prefer not to run local stdio MCP
  in CI runners
- Cross-repo trend analysis and benchmarking

Pricing target: per-repo or per-seat, ~$10–$30/repo/month.

**Pros.**
- Recurring revenue scales with adoption.
- Hosted layer is a natural extension, not a feature gate on the core wedge.
- ICP brief's "Platform/DevEx" secondary persona is the buyer.
- Aligned with category POV — local-first stays free; cross-repo
  governance is paid.

**Cons.**
- Building and operating SaaS is a real cost: hosting, on-call, billing,
  support. Roughly 30–50% of maintainer time once live.
- Open-core boundary requires constant defense — what stays free, what
  is paid. Misjudgments here erode community trust.
- 12–18 month payback to first meaningful MRR.

**Verdict.** Strongest revenue ceiling. Requires sustained operational
investment. Right answer if maintainer commits to going beyond OSS.

### Option C — Paid SLA Support

**Mechanism.** OSS remains free. Paid tier: priority issue response,
guaranteed bug-fix SLA, private Slack/email support, custom runner
integration support.

Pricing target: $500–$5K/month per organization.

**Pros.**
- Low operational overhead — no infrastructure to run.
- Direct buyer signal — companies paying for SLA are exactly the ICP
  expansion segment (Platform/DevEx with compliance pressure).
- Can layer on top of any other option.

**Cons.**
- Revenue scales with maintainer hours, not with adoption.
- Lonely position — most developer tools that try this struggle to find
  buyers willing to pay for support on a free CLI.
- Sales motion is high-touch; not aligned with community-led GTM.

**Verdict.** Layer, not foundation.

### Option D — Enterprise Security Gate (audit log, SSO, compliance export)

**Mechanism.** OSS remains free. Closed-source enterprise build adds:
- Audit logging of all MCP tool calls with tamper-evident storage
- SSO/SAML for hosted dashboard
- Compliance exports (SOC 2 audit format, evidence bundles)
- Private MCP server image / on-prem deployment

Pricing target: $20K–$80K/year per organization.

**Pros.**
- Highest per-customer revenue.
- Aligned with compliance accelerator from category POV.
- The threat-model doc (`docs/security/mcp-threat-model.md`) is already
  the enabling artifact.

**Cons.**
- Enterprise sales motion is expensive: SOC 2 audit, DPA templates,
  procurement responses, security questionnaires. Realistic 6–12 month
  build before first deal.
- Requires sales hire or maintainer-as-salesperson — neither cheap.
- Premature for current adoption stage. ICP brief explicitly defers
  enterprise.

**Verdict.** Right destination, wrong stage. Defer until 100+ champion
users in the wedge segment.

## Decision

**Adopt Option A (Sponsorship) immediately. Plan toward Option B (Open
Core) as the primary monetization path. Defer Options C and D.**

### Concretely

| Stage | Trigger | Action |
|---|---|---|
| Now | — | Enable GitHub Sponsors. Document sponsor tiers. Add a "Sponsor coverctl" link to README. Floor option active. |
| Stage 2 | 50+ unique repos with weekly active `check` calls (measured via opt-in trace donation pipeline from feature `activation-funnel-and-gtm-metrics`) | Begin design of hosted history + team dashboards SKU. Pick hosting (Fly.io, Railway, or Cloudflare for MCP relay). Validate willingness-to-pay via 5–10 prospect interviews before building. |
| Stage 3 | First 5 paying open-core customers + reproducible activation funnel | Layer Option C (paid SLA support) on top of open-core. Targets the buyer who already pays and wants more service. |
| Stage 4 | 100+ champion users + at least 3 inbound enterprise procurement requests | Begin Option D (enterprise security gate). Hire or contract sales support before, not after. |

### Reasoning

- Option B is the only path with revenue ceiling tall enough to fund
  sustained category leadership in agent-loop coverage governance. Sponsor-
  ship alone funds maintenance, not category creation.
- Option B aligns with the wedge: the free CLI/MCP server is the loss
  leader that creates dependence on the paid hosted governance layer.
  Same shape as Sentry, Datadog, GitLab, Codecov — pattern-validated.
- Stage gating prevents the failure mode where the maintainer builds a
  hosted SKU before the wedge is proven. Stage 2 trigger is a falsifiable
  adoption signal, not a calendar date.
- Options C and D layer on, they don't replace. Sequencing matters: Option
  D before Option B is enterprise-without-funnel. Option B before Option C
  is paid-product-without-support-motion (acceptable).

### What this decision is not

- Not a roadmap for Stage 2 implementation. That happens when the trigger
  fires and goes through normal roady planning.
- Not a license change. Core remains Apache/MIT. Open-core boundary is
  drawn at the *hosted* layer, not at CLI features.
- Not a commitment to never accept enterprise inbound. If a Stage 4 deal
  walks in early, it gets evaluated case-by-case. The decision rule is
  about where to *invest* maintainer time proactively.

## Risks and counter-arguments

**"What if a competitor monetizes faster?"** Acceptable risk. Category
creation rewards the seller of the narrative more than the first to
revenue. Open core caught up to incumbents in many categories (GitLab,
Sentry, dbt) by being slower to monetize and faster to community-build.

**"What if the maintainer needs revenue before Stage 2 trigger fires?"**
Then Option C (paid SLA support) layers on as a stopgap without
contradicting the long-term path. Document this as the early-revenue
escape hatch.

**"What if MCP loses to a different agent integration standard?"** Then
the integration substrate of the hosted SKU changes; the category and
monetization shape do not. The hosted layer is "uniform governance across
repos with agent-callable interface" — substrate-agnostic.

**"What if the open-core boundary is wrong?"** Most likely failure mode.
Mitigation: Stage 2 begins with prospect interviews, not coding. Pricing
and feature boundary validated against real buyers before build.

## Revisit conditions

- Stage 2 trigger fires (act on plan).
- 12 months elapse without Stage 2 trigger (re-evaluate ECP — wedge may be
  weaker than expected).
- New competitor enters category with materially different monetization
  pattern (re-evaluate Option B mechanics).
- MCP adoption stalls or fragments (re-evaluate substrate assumption).
