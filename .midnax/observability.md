# Observability

## Backend (Go)
- **Logging:** File-based logger (internal/applog) → ~/.lazyswap/lazyswap.log; never stdout/stderr (preserves TUI and MCP JSON-RPC stream)
- **Test isolation:** LAZYSWAP_TEST=1 routes log to /dev/null

## Frontend (React/Vite)
- **Cloudflare Workers:** Observability enabled (observability.enabled: true in wrangler.jsonc)

## Smart Contracts (Solidity)
- **Coverage:** Line coverage ≥70% over src/ (test mocks and deploy scripts excluded); enforced in CI via scripts/coverage-gate.sh
