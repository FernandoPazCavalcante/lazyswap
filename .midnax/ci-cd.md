# CI/CD

## Frontend (lazyswap-site)
- **CI tool:** GitHub Actions (`.github/workflows/deploy.yml`)
- **Trigger:** every push to `master`, all PRs, manual `workflow_dispatch`
- **Pipeline:** build (pnpm install --frozen-lockfile && pnpm build) → installer smoke-test (bash -n syntax check, binary execution, curl verification) → deploy (Wrangler to Cloudflare Workers, live verification)
- **Concurrency:** serialized per ref; in-progress runs never cancelled
- **Pinned versions:** pnpm 12.15.0, Node 22

## Backend (lazyswap)
- **CI tool:** GitHub Actions (`.github/workflows/ci.yml`, `.github/workflows/nightly.yml`, `.github/workflows/release.yml`)
- **Trigger:** every PR and push to `master` (ci.yml); scheduled nightly (nightly.yml); push to `master` (release.yml)
- **Pipeline (ci.yml):** lint (golangci-lint v2.15.0) → test (coverage gate ≥70% ex-TUI) → e2e (smoke test) → mutation-report (report-only)
- **Pipeline (nightly.yml):** mutation (blocking, ≥50% efficacy) → funded-e2e (real bsc_testnet swap)
- **Release:** semantic-release v24 with exec plugin; cross-compile via `scripts/build-release.sh` (CGO_ENABLED=0; targets linux-x64/arm64, darwin-x64/arm64); version stamped via ldflags
- **Branch strategy:** trunk-based (master); beta prereleases, stable full releases

## Smart Contracts (lazyswap-contracts)
- **CI tool:** GitHub Actions (`.github/workflows/ci.yml`)
- **Trigger:** every PR and push to `master`
- **Pipeline:** format check (forge fmt --check) → build (forge build --sizes) → test (FOUNDRY_PROFILE=ci forge test -vvv, 1024 fuzz runs) → coverage gate (≥70% over src/)
- **Local mirror:** `make gate` (fmt-check → build → test → cover)
