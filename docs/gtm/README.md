# GTM metrics scaffolding

Specs live in:

- [`docs/design/gtm-metrics-spec.md`](../design/gtm-metrics-spec.md) — activation funnel
- [`docs/design/mcp-metrics-spec.md`](../design/mcp-metrics-spec.md) — tool-call success / retained value

There is **no live dashboard** in-repo (by design for v1). Use:

1. Opt-in MCP telemetry: `coverctl mcp serve --mcp-telemetry` (JSONL on stderr)
2. Local ranking: `python3 scripts/analyze-mcp-telemetry.py telemetry.jsonl`
3. Fixtures + DuckDB SQL under `fixtures/` and `queries/` for dry runs
4. Monthly review template: [`funnel-review-template.md`](./funnel-review-template.md)

Human logs (mentions, enterprise inbound) are markdown tables you fill by hand until donation/aggregation ships.
