# CI/CD

## Backend (Go)
- **CI tool:** GitHub Actions (`.github/workflows/ci.yml`, `nightly.yml`, `release.yml`)
- **Pipeline:** lint (golangci-lint) → test (coverage ≥70% ex-TUI) → e2e (CLI + TUI smoke) → mutation-report (gremlins, report-only on PR; blocking ≥50% on nightly)
- **Branch strategy:** trunk-based (master); beta/stable branches for releases
- **Release:** semantic-release v24 on push to master; cross-compile to linux-x64/arm64, darwin-x64/arm64 via `scripts/build-release.sh` (CGO_ENABLED=0)

## Frontend (React/Vite)
- **CI tool:** GitHub Actions (inferred from repo structure; no explicit workflow provided)
- **Build:** `pnpm build` → Vite production bundle
- **Deployment:** Wrangler v4 to Cloudflare Workers; build command defined in `wrangler.jsonc`

## Smart Contracts (Solidity)
- **CI tool:** GitHub Actions (`.github/workflows/ci.yml`)
- **Pipeline:** format check (forge fmt --check) → build (forge build --sizes) → test (FOUNDRY_PROFILE=ci, 1024 fuzz runs) → coverage gate (≥70% src/)
- **Local gate:** `make gate` (fmt-check → build → test → cover)
- **Fuzz runs:** 512 locally, 1024 in CI
