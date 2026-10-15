# Tech Stack

## Backend
- **Go 1.26+** — terminal wallet and CLI (lazyswap); SQLite DAO (modernc/sqlite, cgo-free); AES-256-GCM + PBKDF2 encryption
- **Solidity 0.8.24** — smart contracts (FeeVault, LazySwapPass); Foundry (forge, cast)

## Frontend
- **TypeScript** — React 18 + Vite 6 SPA (lazyswap-site)
- **Styling** — Tailwind CSS v4 (@tailwindcss/vite plugin)
- **UI** — shadcn/ui (Radix primitives) + MUI v7; lucide-react icons; motion animations; react-router v7; react-hook-form; recharts; react-dnd
- **Package manager** — pnpm (enforced; no npm/yarn)

## Infrastructure
- **Cloudflare Workers** — Wrangler v4; observability enabled
- **EVM chains** — Ethereum, BSC, Polygon, Arbitrum, Base (via public RPC endpoints)
- **Cross-chain** — THORchain API for BTC swaps
- **Smart contract libraries** — OpenZeppelin Contracts (git submodule); forge-std (git submodule)
