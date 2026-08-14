# CI/CD

## Backend
- **GitHub Actions** (.github/workflows/release.yml) — triggered on push to `master`
- Runs `scripts/build-release.sh` (CGO_ENABLED=0) → cross-compiled tarballs in `dist/`
- **semantic-release v24** — derives versions from Conventional Commits; creates GitHub Release with tarballs + SHA256 checksums
- Branch strategy: `master` → `beta` prereleases; `stable` → full releases

## Frontend
- **Cloudflare Workers build** — defined in `wrangler.jsonc`; runs `pnpm install --frozen-lockfile && pnpm build` automatically on Cloudflare during deployment
- Deploy command: `pnpm wrangler deploy` (requires Wrangler auth)

## Smart Contracts
- No CI pipeline configured — run `forge test` and `forge fmt --check` locally before pushing
