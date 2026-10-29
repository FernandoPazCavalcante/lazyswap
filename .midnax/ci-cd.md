# CI/CD

## Frontend (lazyswap-site)
- **Trigger:** every push to `master`, all PRs, manual `workflow_dispatch`
- **Concurrency:** serialized per ref; in-progress runs never cancelled
- **Pipeline:** build (pnpm install --frozen-lockfile && pnpm build; asserts dist/install.sh, dist/uninstall.sh, dist/index.html) → installer (bash -n syntax check, REPO assertion, smoke-test binary) → deploy (Wrangler; skipped on PRs; curl verification of live installer)
- **Secrets:** CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID
- **Pinned versions:** pnpm 12.15.0, Node 22

## Backend (lazyswap)
- **Trigger:** every PR and push to `master` (ci.yml); nightly schedule + manual (nightly.yml); push to `master` (release.yml)
- **CI pipeline:** lint (golangci-lint v2.15.0) → test (coverage ≥70% ex-TUI) → e2e (smoke test; funded swap self-skips) → mutation-report (report-only)
- **Nightly:** mutation (blocking; ≥50% efficacy) → funded-e2e (real bsc_testnet swap when secret set)
- **Release:** semantic-release 24 with exec plugin; cross-compile via scripts/build-release.sh (CGO_ENABLED=0; targets linux-x64/arm64, darwin-x64/arm64); version stamped via ldflags

## Smart Contracts (lazyswap-contracts)
- **Trigger:** every PR and push to `master`
- **Pipeline:** format check (forge fmt --check) → build (forge build --sizes) → test (FOUNDRY_PROFILE=ci forge test -vvv; 1024 fuzz runs) → coverage gate (src/ ≥70%)
- **Local mirror:** make gate (fmt-check → build → test → cover)
- **Fuzz runs:** 512 locally, 1024 in CI

