# CI/CD

## Frontend (lazyswap-site)
- **CI tool:** GitHub Actions
- **Pipeline:** `.github/workflows/deploy.yml` — triggered on every push to `master`, all PRs, and manual dispatch
- **Jobs:** `build` (pnpm install + build; asserts dist artifacts), `installer` (bash syntax-check + smoke-test), `deploy` (Wrangler; skipped on PRs; live verification curl)
- **Branch strategy:** trunk-based (master); concurrency serialized per ref
- **Secrets required:** `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`
- **Pinned versions:** pnpm 12.15.0, Node 22

## Backend (lazyswap)
- **CI tool:** GitHub Actions
- **Pipelines:** 
  - `.github/workflows/ci.yml` — runs on every PR and push to `master`: lint (golangci-lint v2.15.0), test (coverage ≥70% ex-TUI), e2e (smoke), mutation-report (report-only)
  - `.github/workflows/nightly.yml` — scheduled + manual: mutation (blocking, ≥50% efficacy), funded-e2e (real swap on bsc_testnet)
  - `.github/workflows/release.yml` — on push to `master`: semantic-release v24 with exec plugin; cross-compile via `scripts/build-release.sh` (CGO_ENABLED=0; targets linux-x64/arm64, darwin-x64/arm64)
- **Branch strategy:** trunk-based (master); conventional commits required for semantic-release
- **Pinned versions:** golangci-lint v2.15.0, gremlins v0.5.0, Go 1.26+

## Smart Contracts (lazyswap-contracts)
- **CI tool:** GitHub Actions
- **Pipeline:** `.github/workflows/ci.yml` — runs on every PR and push to `master`: format check (forge fmt --check), build (forge build --sizes), test (FOUNDRY_PROFILE=ci; 1024 fuzz runs), coverage gate (≥70% src/)
- **Branch strategy:** trunk-based (master)
- **Local gate:** `make gate` mirrors CI exactly (fmt-check → build → test → cover)

