# Tech Stack

## Backend
- **Language:** Go 1.26+ (lazyswap core)
- **Frameworks/Libraries:** Bubble Tea (TUI), go-ethereum, OpenZeppelin Contracts (via git submodule)
- **Key packages:** internal/swap (quote/execute), internal/wallet (SQLite DAO via modernc/sqlite), internal/dex (Uniswap V2/PancakeSwap), internal/thorchain (cross-chain BTC), internal/crypto (AES-256-GCM + PBKDF2)

## Frontend
- **Language:** TypeScript
- **Framework:** React 18
- **Bundler:** Vite 6
- **Styling:** Tailwind CSS v4 (@tailwindcss/vite plugin)
- **Component library:** shadcn/ui (Radix UI primitives) + MUI v7
- **UI utilities:** lucide-react (icons), motion (animation), react-router v7 (routing), react-hook-form (forms), recharts (charts), react-dnd (drag-and-drop)
- **Package manager:** pnpm 12.15.0
- **Deployment runtime:** Cloudflare Workers (Wrangler v4)

## Infrastructure
- **Smart contracts:** Solidity ^0.8.24 (pinned to 0.8.24)
- **Contract toolchain:** Foundry (forge, cast)
- **Contract test helpers:** forge-std (git submodule), OpenZeppelin Contracts (git submodule)
- **Optimizer:** enabled, 200 runs

