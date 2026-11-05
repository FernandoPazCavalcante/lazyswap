# Observability

## Frontend (lazyswap-site)
- **Cloudflare Workers observability:** enabled (`observability.enabled: true` in `wrangler.jsonc`)

## Backend (lazyswap)
- **Logging:** file-based via `internal/applog` → `~/.lazyswap/lazyswap.log` (never stdout; preserves Bubble Tea TUI and MCP JSON-RPC stream)
- **Test logging:** routed to `/dev/null` when `LAZYSWAP_TEST=1`
- **No centralized monitoring/tracing configured**

## Smart Contracts
- **No observability tooling configured**
