# Observability

## Frontend (lazyswap-site)
- **Monitoring:** Cloudflare Workers observability enabled (observability.enabled: true in wrangler.jsonc)

## Backend (lazyswap)
- **Logging:** File-based logger (internal/applog) → ~/.lazyswap/lazyswap.log; never writes to stdout/stderr outside CLI (preserves Bubble Tea TUI and MCP JSON-RPC stream)
- **Test isolation:** LAZYSWAP_TEST=1 routes log to /dev/null
- **No centralized monitoring/tracing** — local file logging only

## Smart Contracts (lazyswap-contracts)
- **No observability tooling documented** — Foundry test output and CI logs only

