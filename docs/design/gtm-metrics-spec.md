# GTM Funnel Metrics Spec

This is the **GTM-side measurement layer**. It is distinct from product
metrics — `docs/design/mcp-metrics-spec.md` covers tool-call success rate,
rejection rate, and pre-commit regression catch rate. Those measure
*whether the tool works*. This document covers the funnel that decides
*whether the GTM is working*: do users discover, activate, retain, and
advocate.

The two layers feed each other but are read by different audiences. Tool
metrics gate engineering. GTM metrics gate motion stack and pricing
decisions.

## Why now

`docs/strategy/category-pov.md` defines the wedge. `docs/gtm/icp-brief.md`
picks the motion stack (community-led + content-led) and lists "first
proof metrics." This document operationalizes those proof metrics into
named instruments with collection methods, target benchmarks, and
revisit cadences.

## North Star (recap)

**Weekly Protected Agent Loops** — unique repos where `coverctl check`
ran in agent or MCP context within the last 7 days *and* blocked at
least one regression. Defined in
[the PRD](./language-agnostic-prd.md#north-star).

NSM is product-rooted. The GTM funnel below explains how new repos
arrive at the NSM and why they stay.

## Funnel definitions

### 1. Activation rate

```
activation_rate = (repos with at least one passing `check`) / (repos that ran `init`)
```

**Collection.** Two events emitted from CLI/MCP, both opt-in:

- `RecordActivationStep("init_completed", repoFingerprint)`
- `RecordActivationStep("first_passing_check", repoFingerprint)`

`repoFingerprint` is a deterministic hash of `git remote get-url origin`
SHA-256-truncated to 12 chars. No URL or repo content is transmitted.

**Target.** ≥ 60% within first 30 days of install.

**Why this matters.** Activation is the cleanest leading indicator. If
fewer than half of users who run `init` ever reach a passing check, the
problem is upstream of all retention work — usually a UX or
documentation gap surfaced by failure-mode snippets and the trust
calibration callout (T17/T18).

### 2. 30-day usage retention

```
retention_30d = (repos calling `check` in week 4) / (repos calling `check` in week 1)
```

**Collection.** Aggregated from existing `RecordToolCall` events on the
opt-in side. No new instrument needed.

**Target.** ≥ 35% (best-in-class developer-tool retention is 40-50%; CLI
adoption skews lower than SaaS).

**Why this matters.** One-shot users are a sign of failed PMF or wrong
ECP. Sustained call rate is the strongest implicit value signal we have.

### 3. Advocate mention count

```
mentions_per_week = unprompted references in Claude Code / Discord / X / forums
```

**Collection.** External, manual at first. Set up a monitoring alert
(Beeper / Mention.com / Slack search) for "coverctl" mentions in
relevant communities. Track in a spreadsheet; review monthly.

**Target.** > 5 unprompted mentions per week by month 6.

**Why this matters.** Word-of-mouth is the cheapest acquisition channel
and a leading indicator for community-led motion health (per
gtm-expert review). If mentions are flat after a launch event, the
content-led motion is not connecting POV to community.

### 4. Plugin marketplace install velocity

```
installs_per_week = new plugin installs / week (Claude Code marketplace + MCP Registry)
```

**Collection.** External. Marketplace analytics where available; GitHub
release download counts as proxy where not. Track 7-day rolling average.

**Target.** Compounding week-over-week growth in months 1-3 post-launch.
Flat after that signals saturation in current ICP and need to expand.

### 5. Sean Ellis 40% PMF survey

```
pmf_signal = (% surveyed users responding "very disappointed" if coverctl went away)
```

**Collection.** One-question opt-in survey. Two delivery options:

1. **CLI prompt** after Nth use of `check` (e.g., 20 calls). Single
   stdout question with `[y]es / [n]o / [s]kip` response. Implementable
   as the lightest possible instrument.
2. **Docs-site banner** for users who land on quick-start while logged
   in to GitHub OAuth.

Aggregated server-side via the trace-donation pipeline. See "Pipeline
shape" below.

**Target.** > 40% (Ellis threshold). Below 40% means PMF is unproven and
GTM scaling is premature.

**Revisit.** Quarterly until threshold is hit, then annually.

### 6. Enterprise inbound

```
enterprise_inbound = (procurement / security / DPA documentation requests per month)
```

**Collection.** External. GitHub issue label `enterprise-inquiry`,
contact-form submissions, Discord/email mentions of "SOC 2", "DPA",
"procurement", "security questionnaire".

**Target.** ≥ 3 per month before triggering Stage 4 of monetization
plan (`docs/strategy/monetization-decision.md`).

**Why this matters.** Pre-trigger, this is a non-actionable signal.
Post-trigger, it becomes the leading indicator for enterprise GTM
investment. Track it now so the trigger is supported by data, not
intuition.

## Pipeline shape (opt-in trace donation)

The product- and GTM-funnel metrics on the opt-in side require a
collection pipeline. The pipeline is deliberately minimal:

```
CLI / MCP server
   │ writes (opt-in only) JSONL events to stderr or local file
   │
   ├─→ User stores locally for own analysis (default)
   │
   └─→ User opts into "donate anonymized telemetry"
        │
        v
   coverctl-telemetry.example.com  (Cloudflare Worker stub for v1)
        │
        v
   append-only object storage (R2 / S3) partitioned by week
        │
        v
   manual aggregation queries (DuckDB local) → monthly review
```

**Privacy guarantees the pipeline must enforce:**

- Repo fingerprints are hashed before send; no remote URL leaves the
  client.
- File paths are never transmitted. Tool names, durations, error codes,
  and rejection reasons are.
- Donation is **opt-in per repo**. Default is no transmission.
- Donation can be revoked; receiver honors revocation by purging
  fingerprint history.
- No analytics provider in the pipeline (no Segment, no Amplitude, no
  Posthog managed instances).

**v1 scope of what we build now**

- `Telemetry` interface extension: `RecordActivationStep(step, fingerprint)`
- Wire `init_completed` and `first_passing_check` events in handlers
- `MetricsTelemetry` writes events to stderr in opt-in mode (existing)

**What v1 does not build (deferred)**

- Trace donation receiver (Cloudflare Worker / S3 sink)
- Sean Ellis CLI prompt / OAuth survey infrastructure
- Aggregation queries / dashboards
- Mention-tracking alerts (manual setup)

These are infrastructure deliverables that fire only after the wedge
metric (NSM) shows steady growth. Build measurement *capacity* now;
build measurement *automation* when adoption justifies the cost.

## Revisit conditions

| Condition | Action |
|---|---|
| Activation rate < 30% for two consecutive months | Re-run usability test (T23/T24); fix highest-friction step. |
| 30-day retention < 20% | Re-evaluate ECP — wedge may not be matching real usage. |
| Sean Ellis PMF < 40% after 6 months | Pause community-led GTM scaling; return to discovery. |
| Mentions flat for 3 months post-launch | Re-evaluate content calendar; may be talking past ECP. |
| Enterprise inbound ≥ 3/month | Trigger Stage 4 evaluation in monetization plan. |
| Any of the above stable above threshold for 6 months | Increase investment in that motion. |
