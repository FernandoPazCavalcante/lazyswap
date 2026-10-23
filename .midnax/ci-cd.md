# CI/CD

## Backend (Go)
- **CI tool:** GitHub Actions
- **Workflows:**
  - **ci.yml** (every PR + push to master): lint (golangci-lint v2.15.0) → test (coverage ≥70% ex-TUI) → e2e smoke (build tag e2e) → mutation report (report-only)
  - **nightly.yml** (scheduled + manual): mutation test (blocking, ≥50% efficacy) → funded e2e on bsc_testnet (when secret set)
  - **release.yml** (push to master): cross-compile release tarballs (CGO_ENABLED=0; linux-x64/arm64, darwin-x64/arm64) → semantic-release (Conventional Commits)
- **Branch strategy:** master (beta prereleases) → stable (full releases)
- **Local gate:** `make gate` (lint → test → cover)

## Frontend (React/Vite)
- **Build tool:** Vite 6
- **Package manager:** pnpm (no npm/yarn)
- **Deployment:** Cloudflare Workers (Wrangler v4); build command in wrangler.jsonc runs automatically on Cloudflare side

## Smart Contracts (Solidity)
- **CI tool:** GitHub Actions
- **Workflow:** ci.yml (every PR + push to master): format check (forge fmt --check) → build (forge build --sizes) → test (FOUNDRY_PROFILE=ci, 1024 fuzz runs) → coverage gate (src/ ≥70%)
- **Local gate:** `make gate` (fmt-check → build → test → cover)
- **Fuzz runs:** 512 locally, 1024 in CI
