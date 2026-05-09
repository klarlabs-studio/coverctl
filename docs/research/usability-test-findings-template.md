# Polyglot Claude Code Usability Test — Findings

> **Status: TEMPLATE.** Copy this file to
> `docs/research/usability-test-findings-2026-MM-DD.md` after running
> sessions; fill placeholders with real observations. Do not edit this
> file in place — it is the canonical template.

Run cohort: 5 polyglot Claude Code / Cursor users. Test protocol:
[`usability-test-protocol.md`](./usability-test-protocol.md). Reporting
audience: maintainer + future contributors.

## Cohort summary

Replace this table after sessions complete. Names anonymized; stack
and role only.

| ID | Stack | Role | Company size | Recording? |
| --- | --- | --- | --- | --- |
| P1 | Python+TS | … | … | yes / no |
| P2 | Python+TS | … | … | … |
| P3 | Go+Rust | … | … | … |
| P4 | Go+Rust | … | … | … |
| P5 | Java or Shell | … | … | … |

## Goal recap

Three onboarding fixes under test:

1. Agent-mode discovery
2. Failure-mode recovery (init, check)
3. Time-to-first-fix < 10 minutes

For each, the report below states **what we observed**, **what it
means**, and **what we change next**.

## Per-participant rubric

Scored live during sessions per the rubric in the protocol. Aggregate
in the next section; this table preserves per-participant signals so
patterns stay visible.

| Signal | P1 | P2 | P3 | P4 | P5 |
| --- | --- | --- | --- | --- | --- |
| Agent-mode discovery | did / told | … | … | … | … |
| First-fix time (min) | … | … | … | … | … |
| Failure recovery (init) | self / help | … | … | … | … |
| Failure recovery (check) | self / help | … | … | … | … |
| MCP setup outcome | success / doc / abandoned | … | … | … | … |
| Trust calibration | cal / over / under | … | … | … | … |
| First emotional peak | quote + moment | … | … | … | … |
| First friction peak | quote + moment | … | … | … | … |

## Headline metrics

Counted after all 5 sessions.

| Metric | Result | Target |
| --- | --- | --- |
| Discovered agent mode unprompted | x / 5 | ≥ 4 / 5 |
| Reached first fix in < 10 min | x / 5 | ≥ 3 / 5 |
| Recovered from first failure without help | x / 5 | ≥ 4 / 5 |
| Likelihood-to-use score (median 1-5) | … | ≥ 4 |
| Sean Ellis-style "very disappointed if I lost it" | x / 5 | ≥ 2 / 5 (small N proxy) |

## Verbatim quotes

One per participant, the strongest. Include the moment and what
triggered it. Replace bullets after sessions.

- *P1, while reading the failure caution block:* "…"
- *P2, after first MCP tool call:* "…"
- *P3, on the trust-calibration callout:* "…"
- *P4, hitting an unsupported repo shape:* "…"
- *P5, at the closing 'what would you keep' question:* "…"

## Friction map

Tag each session's worst moment with one of: discovery, install,
init, check, MCP-setup, agent-trust, fix-loop. Build a frequency
table — clusters point at the highest-leverage iteration target.

| Tag | Count | Notes |
| --- | --- | --- |
| discovery | 0 | … |
| install | 0 | … |
| init | 0 | … |
| check | 0 | … |
| MCP-setup | 0 | … |
| agent-trust | 0 | … |
| fix-loop | 0 | … |

## What we change

Three concrete changes the cohort surfaced. If fewer than three
emerge, do not fabricate — write fewer.

1. **What:** … **Why:** … **Where:** *(file path or PR slug)*
2. **What:** … **Why:** … **Where:** …
3. **What:** … **Why:** … **Where:** …

## What we keep

Things that landed well and should not change in the next iteration.

- …
- …

## What we explicitly defer

Real signals from the cohort that we choose **not** to act on now,
with reason. Useful so reviewers understand the prioritization, not
the omission.

- …
- …

## Stop-condition triggers

Did any of the protocol's stop conditions fire? If yes, the cohort
ran < 5 — record reason here.

- … (none, or describe)

## Hand-off

- Linked roady tasks created from the "What we change" list:
  *(comma-separated list of new task IDs after roady_add_feature)*
- Updates to PRD / ICP brief / mcp-metrics-spec, if any:
  *(file paths)*
- Next review date: *(absolute date, six weeks out)*
