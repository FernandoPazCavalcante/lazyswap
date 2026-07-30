# Infrastructure

**Cloud & Hosting**
- Cloudflare: Workers (lazyswap-site), Pages, DNS-01 TLS, Zero Trust tunnels (home-server)
- GitHub: Actions runners (ubuntu-latest, ubuntu-slim), Releases for artifact distribution

**Container & Orchestration**
- Docker Compose (home-server, rinha-de-backend-2024-q1); Kubernetes target with Helm (flagsmith-charts)
- kind cluster for chart testing (flagsmith-charts)

**Managed Services**
- Bitnami PostgreSQL, InfluxDB2 (flagsmith-charts dependencies)
- Cloudflare Tunnels, Zerotier VPN (home-server)

**Local/Self-Hosted**
- Bare-metal Linux (Arch Linux, home-server)
- Pure local binaries (lazyswap); no cloud infra
- Multi-chain smart contract deployment (BSC, Ethereum, Polygon, Arbitrum, Base; lazyswap-contracts)

**IaC & Configuration**
- Helm charts (flagsmith-charts)
- Docker Compose YAML (home-server, rinha-de-backend-2024-q1)
- Wrangler config (lazyswap-site)
- foundry.toml (lazyswap-contracts)
