# Infrastructure

**Cloud provider:** Cloudflare (Workers for frontend deployment)

**Container/Orchestration:** None — lazyswap is a local binary (no server, no backend container)

**IaC tool:** None explicitly configured

**Key managed services:**
- Cloudflare Workers (frontend SPA + install-script routing)
- Public EVM RPC endpoints (Ethereum, BSC, Polygon, Arbitrum, Base, Sepolia — configured in `internal/chain/config.go`)
- THORchain API (cross-chain BTC swaps)
- GoPlus API (pre-swap token risk check, fail-closed)
- Price-monitor backend API (`https://api.lazyswap.app` — bearer-key auth, watchlist management)
- Etherscan/BSCScan/PolygonScan/Arbiscan/BaseScan (contract verification via Foundry)
