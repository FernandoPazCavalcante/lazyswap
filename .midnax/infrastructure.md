# Infrastructure

**Cloud provider:** Cloudflare (Workers for frontend deployment)

**Blockchain infrastructure:**
- EVM chains: Ethereum, BSC, Polygon, Arbitrum, Base, Sepolia (public RPC endpoints; configurable via `LAZYSWAP_RPC_URL` env)
- Cross-chain: THORchain API for BTC swaps
- Smart contracts: Foundry (forge, cast) for development and deployment; Etherscan-compatible block explorers for verification

**Data storage:**
- Frontend: Cloudflare Workers (static assets via Vite build)
- Backend: SQLite (modernc/sqlite, cgo-free) for wallet CRUD; filesystem (`~/.lazyswap/`) for config and logs
- Contracts: On-chain state (FeeVault treasury, LazySwapPass ERC-721 expiry)

**No containerization or orchestration** — lazyswap is a pure local binary; lazyswap-site is serverless (Workers); contracts are stateless deployment scripts.
