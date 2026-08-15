# Monthly funnel review

Period: YYYY-MM  
Reviewer:  
Data sources: survey JSONL / telemetry JSONL / GitHub insights / manual logs

## Snapshot

| Metric | Value | Notes |
| --- | --- | --- |
| Installs (approx) | | brew / GitHub releases |
| `init_completed` events | | telemetry |
| `first_passing_check` events | | telemetry |
| Activation rate | | first_passing / init |
| 30d retention proxy | | repeat fingerprint |
| MCP tool top-3 by retained-value score | | `analyze-mcp-telemetry.py` |
| Advocate mentions | | `mention-log.md` |
| Enterprise inbound | | `enterprise-inbound-log.md` |
| PMF survey n / % very disappointed | | `coverctl survey` |

## Decisions

- Keep / kill / iterate on:
- Revisit conditions from gtm-metrics-spec:

## Next month experiments
