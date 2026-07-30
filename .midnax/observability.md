# Observability

**Backend**
- lazyswap: File-based logger only (`internal/applog` → `~/.lazyswap/lazyswap.log`). No external monitoring/tracing.
- lazyswap-contracts: None configured.

**Frontend**
- lazyswap-site: Cloudflare Workers built-in observability enabled (`"observability": { "enabled": true }` in `wrangler.jsonc`).
- marketingskills, resume: None configured.

**Infrastructure**
- flagsmith-charts: Prometheus ServiceMonitor support configurable in `values.yaml` (`serviceMonitor.enabled`). No external monitoring tooling configured in the repo itself.
- home-server: None configured. Watchtower handles automatic container image updates.
- rinha-de-backend-2024-q1: None configured.
