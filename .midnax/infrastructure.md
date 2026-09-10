# Infrastructure

**Cloud provider** — Cloudflare (Workers for frontend SPA)

**Blockchain** — EVM chains (Ethereum, BSC, Polygon, Arbitrum, Base) via public RPC endpoints; THORchain API for cross-chain BTC swaps

**Container/orchestration** — None; lazyswap is a pure local binary (no server, no backend container)

**IaC** — Foundry (forge) for smart contract deployment; Wrangler v4 for Cloudflare Workers

**Managed services** — Cloudflare Workers (frontend hosting); public EVM RPC endpoints; THORchain API; GoPlus token risk API (cached, fail-closed); OpenOcean swap routing (API mode, mainnets only)

**Data** — SQLite (modernc/sqlite, cgo-free) for wallet DAO; ~/.lazyswap/ data directory (wallets.db, lazyswap.log)
