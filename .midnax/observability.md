# Observability

## Frontend
- **Cloudflare Workers** — observability enabled (observability.enabled: true in wrangler.jsonc)

## Backend (Go)
- **File logging** — internal/applog writes to ~/.lazyswap/lazyswap.log; never stdout (preserves TUI and MCP JSON-RPC stream)
- **Test isolation** — LAZYSWAP_TEST=1 routes log to /dev/null

## Smart Contracts
- No explicit observability tooling documented; test coverage enforced (≥70% src/)
