# Infrastructure

**Cloud provider:** Cloudflare (Workers for frontend deployment)

**Blockchain networks:** EVM chains (Ethereum, BSC, BSC Testnet, Sepolia) via public JSON-RPC endpoints; THORchain for cross-chain BTC swaps

**Container/orchestration:** None — pure local binary (lazyswap) and serverless (Cloudflare Workers)

**Key managed services:**
- Cloudflare Workers (frontend hosting + routing)
- Public EVM RPC endpoints (chain-specific, configured in internal/chain/config.go)
- THORchain API (cross-chain swap routing)
- GoPlus API (token risk assessment)
- OpenOcean aggregator (API swap route; backend not deployed as of v0.1.0)
- Etherscan / BSCScan / PolygonScan / Arbiscan / BaseScan (block explorer APIs, contract verification)

**IaC:** None documented; Foundry scripts (Deploy.s.sol, DeployPass.s.sol) handle smart contract deployment
