# Tech Stack

## Backend
- **Language:** Go 1.26+ (lazyswap core); Solidity 0.8.24 (contracts)
- **Frameworks & libraries:**
  - Go: Bubble Tea (TUI), OpenZeppelin Contracts (via git submodule), forge-std (tests)
  - Solidity: OpenZeppelin Contracts (ERC-721, Ownable, ReentrancyGuard)
- **Key packages:** AES-256-GCM + PBKDF2 encryption, SQLite (modernc/sqlite, cgo-free), GoPlus token risk API, THORchain cross-chain routing

## Frontend
- **Language:** TypeScript
- **Framework:** React 18
- **Bundler:** Vite 6
- **Styling:** Tailwind CSS v4 (`@tailwindcss/vite` plugin)
- **Component library:** shadcn/ui (Radix UI primitives) + MUI v7
- **Key libraries:** react-router v7, react-hook-form, recharts, react-dnd, lucide-react, motion
- **Package manager:** pnpm (enforced; no npm/yarn)

## Infrastructure
- **Deployment runtime:** Cloudflare Workers (Wrangler v4)
- **Blockchain:** EVM chains (Ethereum, BSC, Polygon, Arbitrum, Base, Sepolia); THORchain (BTC)
- **Smart contract toolchain:** Foundry (forge, cast)
