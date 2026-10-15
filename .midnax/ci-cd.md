# CI/CD

## Backend (Go)
- **GitHub Actions** — three workflows: `ci.yml` (PR/master), `nightly.yml` (scheduled), `release.yml` (master)
- **CI pipeline** — lint (golangci-lint), test (coverage ≥70% ex-TUI), e2e smoke (build tag e2e), mutation report (gremlins, report-only)
- **Nightly** — mutation testing (blocking, ≥50% efficacy), funded e2e on bsc_testnet (when secret set)
- **Release** — semantic-release v24; cross-compile to linux-x64/arm64, darwin-x64/arm64 (CGO_ENABLED=0); publish tarballs + SHA256 checksums
- **Branch strategy** — master → beta prereleases; stable → full releases; conventional commits required

## Frontend (TypeScript/React)
- **Deployment** — Wrangler v4 to Cloudflare Workers; build command in wrangler.jsonc runs automatically on Cloudflare side
- **Local dev** — pnpm dev (Vite HMR); pnpm build → dist/; pnpm preview; pnpm wrangler deploy

## Smart Contracts (Solidity)
- **GitHub Actions** — `ci.yml` runs on PR/master: format check (forge fmt --check), build (forge build --sizes), test (FOUNDRY_PROFILE=ci, 1024 fuzz runs), coverage gate (≥70% src/)
- **Local gates** — make gate (fmt-check → build → test → cover); make lint, make test, make cover, make e2e
- **Fuzz runs** — 512 locally, 1024 in CI
