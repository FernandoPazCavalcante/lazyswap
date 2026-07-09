# Observability Stack

## Monitoring & Tracing

Phoenix itself is an observability platform, but uses minimal external observability:

### Internal (Phoenix Project)
- **OpenTelemetry (OTEL)** — First-class support; `arize-phoenix-otel` package for Python and `@arizeai/phoenix-otel` for TS
- **OpenInference** — Standardised semantic conventions for LLM observability (own project by Arize)
- **Tracing formats**: OTLP (OpenTelemetry Protocol) for span export
- **No external APM**: Phoenix provides its own observability UI; no dependency on Datadog, New Relic, or similar

### Logging
- **File-based logging** — lazyswap writes all events to `lazyswap.log` (app-specific, no centralised logging service)
- **Python logging** — standard lib; likely used in phoenix
- **Cloud logging** — azure-infra includes `azurerm_log_analytics_workspace` for Azure Container App and MySQL diagnostics

### Metrics & Analytics
- **Cloudflare Analytics** — cloudflare-metrics repo queries Cloudflare GraphQL API for:
  - Monthly request counts, pageviews, unique visitors
  - HTTP success rates and threat blocking
  - Response times (Edge TTFB, origin latency)
  - No real-time dashboard; script outputs to console and JSON

### Infrastructure Observability (Azure)
- **Azure Log Analytics Workspace** — Diagnostic settings for:
  - Container App logs and metrics
  - MySQL slow query logs and connection metrics
- **No Grafana, Datadog, or CloudWatch** — Azure native only

## Summary

**No production-grade external APM/observability integrations.** The organisation:
- Builds observability tools (phoenix) for others
- Uses file-based and cloud-native logging for internal systems
- Leverages Cloudflare's native analytics API
- Does not use Grafana, Datadog, New Relic, Sentry, CloudWatch, or similar managed platforms

