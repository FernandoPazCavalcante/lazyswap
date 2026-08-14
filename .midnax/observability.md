# Observability

## Backend
- **File-based logging** (lazyswap) — `internal/applog` writes to `~/.lazyswap/lazyswap.log`; never stdout (preserves Bubble Tea TUI)
- **LAZYSWAP_TEST=1** routes log to `/dev/null` in tests

## Frontend
- **Cloudflare Workers observability** — enabled (`observability.enabled: true` in `wrangler.jsonc`)
