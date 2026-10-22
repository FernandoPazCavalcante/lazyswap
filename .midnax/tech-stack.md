# Tech Stack

## Backend
- **Language:** Go 1.26+ (lazyswap CLI/TUI)
- **Key libraries:** Bubble Tea (TUI framework), OpenZeppelin Contracts (via git submodule), forge-std (Foundry test helpers)
- **Database:** SQLite (modernc/sqlite, cgo-free)
- **Crypto:** AES-256-GCM + PBKDF2 (100k iterations) for key encryption
- **Blockchain:** EVM (Ethereum, BSC, Sepolia) via JSON-RPC; THORchain for cross-chain BTC swaps
- **DEX routing:** Uniswap V2, PancakeSwap V2; OpenOcean aggregator (API route)
- **Token risk:** GoPlus API (fail-closed)

## Frontend
- **Language:** TypeScript
- **Framework:** React 18
- **Bundler:** Vite 6
- **Styling:** Tailwind CSS v4 (@tailwindcss/vite plugin)
- **Component library:** shadcn/ui (Radix UI primitives) + MUI v7
- **Icons:** lucide-react
- **Animation:** motion
- **Routing:** react-router v7
- **Forms:** react-hook-form
- **Charts:** recharts
- **Drag-and-drop:** react-dnd + react-dnd-html5-backend
- **Package manager:** pnpm

## Infrastructure
- **Smart contracts:** Solidity 0.8.24 (pinned)
- **Contract toolchain:** Foundry (forge, cast)
- **Deployment runtime:** Cloudflare Workers (Wrangler v4)
- **Observability:** Cloudflare Workers observability enabled
