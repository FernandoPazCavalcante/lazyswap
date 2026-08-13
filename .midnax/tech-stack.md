# Tech Stack

## Backend
- **Go 1.26+** — lazyswap CLI/TUI; cross-compile targets: linux-x64/arm64, darwin-x64/arm64
- **Solidity 0.8.24** — lazyswap-contracts; Foundry toolchain (forge, cast)

## Frontend
- **TypeScript** — lazyswap-site React SPA
- **React 18** + **Vite 6** — bundler with HMR
- **Tailwind CSS v4** (@tailwindcss/vite plugin)
- **shadcn/ui** (Radix UI primitives) + **MUI v7**
- **react-router v7**, **react-hook-form**, **recharts**, **react-dnd**, **lucide-react**, **motion**
- **pnpm** — package manager (required; no npm/yarn)

## Infrastructure
- **Cloudflare Workers** (Wrangler v4) — lazyswap-site deployment runtime
- **Foundry** (forge-std, OpenZeppelin Contracts via git submodules) — smart contract testing and deployment
