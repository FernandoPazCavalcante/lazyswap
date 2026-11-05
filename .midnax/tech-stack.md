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
- **Key libraries:** react-router v7, react-hook-form, recharts, react-dnd, lucide-react, motion
- **Package manager:** pnpm (pinned 12.15.0 in CI)

## Infrastructure
- **Smart contracts:** Solidity ^0.8.24 (pinned 0.8.24)
- **Contract toolchain:** Foundry (forge, cast)
- **Contract dependencies:** forge-std, OpenZeppelin Contracts (git submodules)
- **Deployment runtime:** Cloudflare Workers (Wrangler v4)
