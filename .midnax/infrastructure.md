# Infrastructure

**Cloud provider:** Cloudflare (Workers for frontend SPA + install-script routing)

**IaC tool:** None (Wrangler config in `wrangler.jsonc` for Workers; Foundry config in `foundry.toml` for contracts)

**Container/orchestration:** None (serverless only; Go CLI is a local binary with no backend container)

**Key managed services:**
- Cloudflare Workers (frontend deployment, install-script host)
- Public EVM RPC endpoints (configured per chain in `internal/chain/config.go`; lazyswap connects at runtime)
- THORchain API (cross-chain BTC swap routing)
- LI.FI API (BTC quote provider; optional API key via `LAZYSWAP_LIFI_API_KEY`)
- NEAR Intents API (BTC quote provider; optional API key via `LAZYSWAP_NEAR_INTENTS_API_KEY`)
- GoPlus API (pre-swap token risk check; fail-closed)
- GitHub API (self-update mechanism in lazyswap; release asset downloads)
- Blockchain networks: Ethereum, BSC, BSC Testnet, Sepolia (EVM); Bitcoin (cross-chain via LI.FI / NEAR Intents)

