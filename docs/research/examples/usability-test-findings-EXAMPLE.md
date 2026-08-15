# Polyglot Claude Code Usability Test — Findings (EXAMPLE)

> **FAKE DATA for scaffolding only.** This is not a real study.
> Copy [`usability-test-findings-template.md`](../usability-test-findings-template.md)
> to `usability-test-findings-YYYY-MM-DD.md` after real sessions.

Run cohort: example only. Protocol: [`usability-test-protocol.md`](../usability-test-protocol.md).

## Cohort summary

| ID | Stack | Role | Company size | Recording? |
| --- | --- | --- | --- | --- |
| P1 | Python+TS | IC | 50 | yes |
| P2 | Python+TS | staff | 200 | no |
| P3 | Go+Rust | IC | 20 | yes |
| P4 | Go+Rust | platform | 500 | yes |
| P5 | Java | IC | 80 | no |

## Goal recap

1. Agent-mode discovery — **example:** 3/5 found MCP via docs within 5 minutes
2. Failure-mode recovery — **example:** init without config was confusing for 2/5
3. Time-to-first-fix — **example:** median 12 minutes (missed <10m target)

## Aggregate rubric

| Goal | Median | Notes |
| --- | --- | --- |
| Discovery | 4 | README wedge helped |
| Recovery | 3 | want richer remediation in CLI |
| Time-to-fix | 3 | suggest step underused |

## What we change next (example)

- Surface `coverctl mcp doctor` earlier in quick-start-agent
- Add DocsNext from init failures to configuration domains

## Status

Not a real findings report — delete or ignore when real sessions land.
