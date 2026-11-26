# Tech Stack

## Backend
- **Language:** Go 1.26+ (lazyswap core); Solidity ^0.8.24 (contracts)
- **Go runtime:** stdlib + key libraries: Bubble Tea (TUI), go-ethereum (wallet/crypto), modernc/sqlite (cgo-free), semantic-release (release automation)
- **Solidity toolchain:** Foundry (forge, cast); OpenZeppelin Contracts (git submodule); forge-std (git submodule)

## Frontend
- **Language:** TypeScript
- **UI framework:** React 18
- **Bundler:** Vite 6
- **Styling:** Tailwind CSS v4 (@tailwindcss/vite plugin)
- **Component library:** shadcn/ui (Radix UI primitives) + MUI v7
- **Key libraries:** react-router v7, react-hook-form, recharts, react-dnd, lucide-react, motion
- **Package manager:** pnpm (pinned 12.15.0 in CI)

## Infrastructure
- **Deployment runtime:** Cloudflare Workers (Wrangler v4)
- **Container/orchestration:** None (serverless only)

