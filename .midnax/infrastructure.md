# Infrastructure

**Cloud provider:** Cloudflare (Workers for frontend deployment)

**Blockchain networks:** Ethereum, BSC (mainnet + testnet), Sepolia, Polygon, Arbitrum, Base (via Foundry RPC endpoints in foundry.toml)

**Container/orchestration:** None — lazyswap is a pure local binary; lazyswap-site is serverless (Cloudflare Workers)

**Key managed services:**
- Cloudflare Workers (frontend SPA + install-script routing)
- Cloudflare Workers observability (enabled in wrangler.jsonc)
- EVM RPC endpoints (public, configured per chain in internal/chain/config.go; override via LAZYSWAP_RPC_URL)
- THORchain API (for cross-chain BTC swaps)
- GoPlus API (token risk check, fail-closed, cached)
- Price-monitor backend API (https://api.lazyswap.app; bearer-key auth; override via LAZYSWAP_API_URL)

**IaC:** None explicitly documented; Foundry scripts (Deploy.s.sol, DeployPass.s.sol) handle smart-contract deployment via broadcast.

**Data persistence:** SQLite (modernc/sqlite, cgo-free) for wallet DAO; local filesystem (~/.lazyswap/) for config, logs, keystore exports.

