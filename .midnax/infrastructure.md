# Infrastructure

**Cloudflare Workers** — primary deployment platform for lazyswap-site (Wrangler v4 runtime with nodejs_compat flag).

**EVM RPC endpoints** — lazyswap connects at runtime to public RPC endpoints for Ethereum, BSC, Sepolia, BSC Testnet (configured in `internal/chain/config.go` CHAINS map).

**THORchain API** — cross-chain BTC swap routing.

**Smart contract deployment** — Foundry broadcast scripts with Etherscan verification support (BSC, Ethereum, Polygon, Arbitrum, Base).

**Local data storage** — SQLite (modernc/sqlite, cgo-free) for wallet CRUD; filesystem at `~/.lazyswap/` (wallets.db, lazyswap.log).
