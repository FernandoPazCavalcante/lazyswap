# Technology Stack

## Languages & Runtimes

| Language | Repos | Details |
|---|---|---|
| **Python** | phoenix | Core observability platform; packages via uv & pip; conda support |
| **TypeScript** | phoenix, lazyswap, lazyswap-site | ESM modules, strict mode, pnpm/bun package managers |
| **JavaScript (Node)** | cloudflare-metrics | Node.js for scripting; Cloudflare GraphQL API client |
| **Go** | inventory-tui | Go 1.25+; tview library for TUI |
| **Terraform** | azure-infra | HCL for Azure IaC; Terraform Cloud backend |
| **YAML/Helm** | flagsmith-charts | Helm chart templates for K8s deployment |

## Frameworks & Libraries

### Backend / Core
- **OpenTelemetry** (otel) — standard for tracing, used across phoenix package ecosystem
- **FastAPI** or similar web frameworks in phoenix (implied from REST API, OpenAPI schema)
- **SQLite** — lazyswap uses for wallet storage
- **ethers.js v6** — lazyswap EVM blockchain interactions

### Frontend
- **React 18** — lazyswap-site with TypeScript
- **Tailwind CSS 4** — utility-first styling (lazyswap-site)
- **Radix UI** — accessible component library (50+ @radix-ui/* packages in lazyswap-site)
- **OpenTUI** — terminal UI framework (lazyswap)
- **tview** — Go TUI library (inventory-tui)

### DevOps & CLI
- **Vite** — bundler for web projects
- **Wrangler** — Cloudflare Workers CLI (lazyswap-site)
- **Bun** — runtime for lazyswap; faster alternative to Node.js
- **Helm** — K8s package manager (flagsmith-charts)
- **Docker** — containerisation (phoenix has multi-stage Dockerfile)

### Testing & Linting
- **Playwright** — E2E testing (phoenix)
- **pytest** — Python testing (phoenix)
- **Bun test** — testing framework (lazyswap uses; coverage ≥80% required)
- **ESLint**, **Prettier** — code quality (multiple repos)
- **Oxlint**, **Oxfmt** — high-performance linters (phoenix)
- **pre-commit** — Git hooks (phoenix, flagsmith-charts)

## Package Managers

| Tool | Repos |
|---|---|
| **uv** | phoenix (Python) |
| **pip / conda** | phoenix (alt Python) |
| **pnpm** | lazyswap-site, phoenix (workspaces) |
| **npm** | cloudflare-metrics |
| **bun** | lazyswap (primary) |
| **go mod** | inventory-tui |
| **terraform** | azure-infra (no package manager; HCL modules) |

## Notable Patterns

- **Monorepo structure**: phoenix uses `packages/` for multiple related Python/TS packages
- **Workspace management**: pnpm workspaces for multi-package TypeScript projects
- **Encryption**: AES-256-GCM + PBKDF2 in lazyswap for wallet security
- **OpenInference**: phoenix promotes OpenInference standard for LLM observability; rich integration ecosystem

