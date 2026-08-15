#!/usr/bin/env python3
"""Rank MCP tools from coverctl --mcp-telemetry JSONL (stderr capture).

Usage:
  coverctl mcp serve --mcp-telemetry 2> telemetry.jsonl
  # ... agent session ...
  python3 scripts/analyze-mcp-telemetry.py telemetry.jsonl

Retained-value score per docs/design/mcp-metrics-spec.md:
  score = success_rate * regression_caught_count
where success_rate = success / (success + policy_fail + error + rejected)
"""

from __future__ import annotations

import json
import sys
from collections import defaultdict
from pathlib import Path


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print("usage: analyze-mcp-telemetry.py <telemetry.jsonl>", file=sys.stderr)
        return 2
    path = Path(argv[1])
    tools: dict[str, dict[str, int]] = defaultdict(lambda: defaultdict(int))
    regressions: dict[str, int] = defaultdict(int)
    activation: dict[str, int] = defaultdict(int)

    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or not line.startswith("{"):
            continue
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        if "event" in ev:
            if ev["event"] == "regression_caught":
                regressions[ev.get("tool", "?")] += 1
            elif ev["event"] == "activation_step":
                activation[ev.get("step", "?")] += 1
            continue
        tool = ev.get("tool")
        if not tool:
            continue
        outcome = ev.get("outcome", "unknown")
        tools[tool][outcome] += 1
        tools[tool]["total"] += 1

    print("Tool-call outcomes")
    print(f"{'tool':<12} {'total':>6} {'success':>8} {'policy':>8} {'error':>6} {'rej':>5} {'succ%':>7} {'regr':>5} {'score':>8}")
    ranked = []
    for tool, counts in tools.items():
        total = counts["total"]
        success = counts.get("success", 0)
        policy = counts.get("policy_fail", 0)
        error = counts.get("error", 0)
        rejected = counts.get("rejected", 0)
        rate = (success / total) if total else 0.0
        regr = regressions.get(tool, 0)
        score = rate * regr
        ranked.append((score, tool, total, success, policy, error, rejected, rate, regr))
    for score, tool, total, success, policy, error, rejected, rate, regr in sorted(ranked, reverse=True):
        print(f"{tool:<12} {total:6d} {success:8d} {policy:8d} {error:6d} {rejected:5d} {rate:7.1%} {regr:5d} {score:8.2f}")

    if activation:
        print("\nActivation steps")
        for step, n in sorted(activation.items()):
            print(f"  {step}: {n}")

    if ranked:
        print("\nTop retained-value tools (score = success_rate × regressions):")
        for score, tool, *_ in sorted(ranked, reverse=True)[:3]:
            print(f"  {tool} ({score:.2f})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
