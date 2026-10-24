# Performance

Larder is a local desktop tool. It should stay quiet when it is not doing work.

## Budgets

| Resource | Budget |
| --- | --- |
| Idle CPU | below 1% |
| Main process RSS | below 160 MB |
| WebView + helper processes | below 250 MB total |
| App data | below 50 MB with default retention |
| Idle network requests | none |
| Background timers | none |

## Rules

- Do not add background polling. Use events or explicit refresh.
- Load icons and avatars only when they enter or approach the viewport.
- Render long lists in batches instead of mounting every row.
- Coalesce overlapping full refreshes; refresh the active view on focus.
- Snapshot retention is bounded. Backups are explicit user actions.
- Catalog and registry work is started by user actions, not by app startup.

## Local check

Build/start the app, then run:

```sh
./scripts/perf.sh
```

This prints the main process RSS/CPU and the size of the Larder data directory.
