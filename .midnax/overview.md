# FernandoPazCavalcante's Org — overview

## LazySwap — Org Identity

**What they build**

LazySwap is a decentralized swap platform spanning terminal and web interfaces. The core product is a Go-based terminal wallet and CLI with Vim-style TUI for executing on-chain swaps across EVM chains (Ethereum, BSC, Polygon, Arbitrum, Base) and cross-chain BTC swaps via THORchain. Supporting infrastructure includes Solidity smart contracts (FeeVault, LazySwapPass ERC-721 pass system) and a React + Vite landing page and installer hosted on Cloudflare Workers. All wallet state is encrypted locally (AES-256-GCM + PBKDF2) in SQLite; no backend server.
