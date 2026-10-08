# Observability

## Backend (Go)
- **Logging:** File-based logger (`internal/applog`) → `~/.lazyswap/lazyswap.log`; never stdout/stderr (preserves TUI and MCP JSON-RPC stream)
- **Test isolation:** `LAZYSWAP_TEST=1` routes log to `/dev/null`

## Frontend (React/Vite)
- **Observability:** Cloudflare Workers observability enabled (`observability.enabled: true` in `wrangler.jsonc`)

## Smart Contracts (Solidity)
- **Testing:** forge-std test helpers; fuzz testing with bounded types to avoid overflow
- **Coverage:** enforced gate ≥70% over `src/` only (test mocks and deploy scripts excluded)
