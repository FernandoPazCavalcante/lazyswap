# CI/CD Pipelines

## GitHub Actions Workflows

Phoenix (the largest repo) has extensive CI/CD:

### Python Testing & Publishing
- **`python-CI.yml`** — Full matrix testing across Python versions/platforms on every push/PR
- **`python-all-platforms.yml`** — Extended test suite for edge cases
- **`publish.yaml`** — Publish Python packages to PyPI and conda-forge; triggered on release

### TypeScript Testing & Publishing
- **`typescript-CI.yml`** — Lint, test, build on PR/push
- **`typescript-packages-CI.yml`** — Multi-package testing (monorepo)
- **`typescript-packages-publish.yml`** — Publish @arizeai/* npm packages
- **`typescript-packages-publish-experimental.yml`** — Experimental releases

### Docker & Release
- **`docker-build-release.yml`** — Multi-stage build: test → publish to Docker Hub (`arizephoenix/phoenix`) on release
- **`docker-build-nightly.yml`** — Nightly builds for `latest` tag
- **`docker-build-experimental.yml`** — Experimental branch builds

### Kubernetes & Deployment
- **`helm-ci.yml`** — Helm chart linting via Chart Testing (ct)
- **`helm-release.yaml`** — Release Helm charts to OCI registry

### Documentation & Release Automation
- **`gh_pages.yml`** — Deploy docs site (ReadTheDocs integration for `arize-phoenix` package)
- **`release.yml`** — Automated release notes generation
- **`claude-release-notes.yml`** — Claude-powered release note generation
- **`sync-lockfile-release-pr.yml`** — Keep dependency locks in sync across PRs

### Testing Infrastructure
- **`playwright.yaml`** — Browser E2E tests (6000+ lines, comprehensive)
- **`package-version-check.yml`** — Validate package versioning consistency
- **`openapi-schema.yaml`** — Auto-generate OpenAPI spec

### LazySwap (Secondary Repos)
- **No GitHub Actions found** in `.github/workflows/` — implies local testing only or ad-hoc CI

## Branch Strategy

- **Main branches**: `main` (phoenix, lazyswap-site, cloudflare-metrics, resume) or `master` (azure-infra, lazyswap, dev-agents-setup)
- **Trunk-based**: Pull requests merge to main; CI runs on every push
- **Release triggers**: Tags trigger Docker builds and package publishing

## Deployment Patterns

| Repo | Deployment Method | Trigger |
|---|---|---|
| **phoenix** | Docker Hub + PyPI + npm | Release tag (automated via release-please) |
| **lazyswap-site** | Vite build (SPA) | Manual or PR to main |
| **lazyswap** | Bun CLI/binary + npm | Local testing only (no CI/CD found) |
| **azure-infra** | Terraform Cloud | Manual `terraform apply` per environment |
| **flagsmith-charts** | OCI registry (Helm) | Release tag |

## Release Strategy

- **release-please**: Automated semantic versioning and changelog generation (phoenix, flagsmith-charts)
- **.release-please-manifest.json**: Version tracking per package
- **Semantic versioning**: Major.Minor.Patch throughout

## Artifact Registry

- **Docker Hub**: `arizephoenix/phoenix` images
- **PyPI**: `arize-phoenix` package and subpackages
- **npm**: `@arizeai/*` scoped packages
- **conda-forge**: `arize-phoenix`
- **Helm OCI**: flagsmith-charts pushed to registry

