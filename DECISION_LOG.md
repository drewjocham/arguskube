# Decision Log

> **This file is intentionally a stub.** It used to contain decisions
> copy-pasted from a different project (payments-api / order-processor /
> nginx-ingress rate limits — none of which exist in this repo). KubeWatcher
> reads `DECISION_LOG.md` at runtime via `KUBEWATCHER_DECISION_LOG` and
> surfaces relevant entries to Argus during AI-assisted diagnostics.
> Feeding it the wrong project's history made the AI cite phantom services.
>
> Add real entries here as load-bearing infra/config changes happen.

## Format

Use ATX headings dated `YYYY-MM-DD` followed by free-form prose.
Argus parses headings to scope retrieval, so keep the date format
consistent.

```
# 2026-05-15
Reduced kubewatcher-agent CPU request from 200m → 100m after week of
flat usage. Owner: @sre-team · ticket: KW-42
```

## Entries

<!-- Add new entries below in reverse-chronological order. -->
