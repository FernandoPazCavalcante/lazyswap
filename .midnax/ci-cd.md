# CI/CD

**Backend**
- lazyswap: GitHub Actions (`release.yml`) on push to `master`; cross-compiles Go (CGO_ENABLED=0) for linux/darwin × amd64/arm64 → `dist/`; semantic-release@24 for versioning. Branch strategy: `master` → beta; `stable` → full releases. Conventional Commits enforced.
- lazyswap-contracts: No CI/CD pipeline configured.

**Frontend**
- lazyswap-site: No GitHub Actions workflows; manual Wrangler deployment (`pnpm install --frozen-lockfile && pnpm build` → Cloudflare Pages/Workers).
- marketingskills: GitHub Actions on `main` branch. `validate-skill.yml` (on SKILL.md changes) runs Flash-Brew-Digital/validate-skill@v1. `sync-skills.yml` (on skills/** changes) runs Node.js sync script, auto-commits via stefanzweifel/git-auto-commit-action.
- resume: GitHub Actions (`generate-pdf-release.yml`) on `v*` tags; Playwright renders HTML → PDF, creates GitHub Release with artifacts (90-day retention).

**Infrastructure**
- flagsmith-charts: GitHub Actions on `main`/PRs. `lint-test.yaml` (PR): chart-testing (ct lint/install) with kind cluster. `release-please.yml` (push to main): googleapis/release-please-action. `release.yaml` (tag `flagsmith-*.*.*`): builds chart tarball, uploads to GitHub Release, updates gh-pages Helm repo index. Conventional Commit PR titles enforced. Pre-commit hooks: check-yaml, prettier.
- home-server: No CI/CD; manual `make` commands only.
- rinha-de-backend-2024-q1: GitHub Actions (`repo-lockdown.yml`) auto-closes PRs/issues after deadline (2024-03-11). No build/deploy pipeline.
