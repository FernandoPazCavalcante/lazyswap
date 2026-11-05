# FernandoPazCavalcante's Org — overview

## LazySwap — Org Identity

**What they build**

LazySwap is a self-custody terminal wallet and decentralized exchange aggregator for on-chain swaps across EVM chains and Bitcoin via THORchain. The core product is a Go-based CLI/TUI that enables users to quote and execute swaps locally, paired with Solidity smart contracts (fee vault, ERC-721 pass) and a React marketing site deployed to Cloudflare Workers. All private key material is encrypted locally (AES-256-GCM + PBKDF2) and never transmitted; swaps route through Uniswap V2, PancakeSwap, and cross-chain liquidity.
