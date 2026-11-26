# Observability

## Frontend (lazyswap-site)
- **Observability enabled:** Cloudflare Workers observability (`observability.enabled: true` in wrangler.jsonc)

## Backend (lazyswap)
- **Logging:** File-based logger (`internal/applog`) → `~/.lazyswap/lazyswap.log`; never writes to stdout/stderr outside CLI (preserves TUI and MCP JSON-RPC stream)
- **Test isolation:** `LAZYSWAP_TEST=1` routes log to `/dev/null`

## Smart Contracts
- No observability tooling configured

