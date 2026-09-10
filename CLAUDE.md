## Organization context

This repository is part of the **FernandoPazCavalcante's Org** organization. Shared org-wide context (overview, tech stack, CI/CD, observability, infrastructure, ticket workflow, integrations) lives in [`.midnax/`](./.midnax/) — read `.midnax/overview.md` first, then the domain files.

## What this is

**lazyswap** is a self-custody terminal wallet — a Vim-style **TUI and a non-interactive CLI** — that executes crypto swaps directly on-chain from your machine. No exchange account, no custodian. It supports EVM chains (Ethereum, BSC) via Uniswap V2 / PancakeSwap, and cross-chain BTC swaps via THORchain.

Go rewrite of the original Bun/TypeScript app (reference: `lazyswap-old/`). Module: `github.com/FernandoPazCavalcante/lazyswap`. Requires **Go 1.26+** (go.mod: `go 1.26.3`).

> **Org-wide context** (CI/CD, infra, observability, tech stack, other repos) lives in [`.midnax/`](./.midnax/) — read `.midnax/overview.md` first, then the domain files.

---

## Commands

```bash
# Run
go run .                          # run from source (no binary)
go build -o lazyswap .            # build binary (gitignored)

# Test
go test ./...                     # all tests
go test ./internal/swap/          # single package
go test -run TestQuote ./...      # filter by name
go test -cover ./...              # with coverage

# Release build (cross-compile → dist/)
bash scripts/build-release.sh     # CGO_ENABLED=0; targets: linux-x64/arm64, darwin-x64/arm64

# Quality gates (see REVIEW.md for thresholds; CI enforces on every PR)
make gate                    # lint + coverage >= 70% (ex-TUI) — the PR-gate mirror
make lint                    # golangci-lint (cyclomatic <= 15, funcs <= 80 lines, files <= 500)
make test                    # LAZYSWAP_TEST=1 go test -race ./...
make cover                   # coverage gate (scripts/coverage-gate.sh)
make e2e                     # CLI quote + TUI smoke (build tag e2e); funded swap self-skips
make mutate                  # gremlins mutation run (nightly enforces >= 50%)
```

**Install from source:**
```bash
go install github.com/FernandoPazCavalcante/lazyswap@latest
```

**CLI usage examples:**
```bash
lazyswap                          # launch TUI (no args)
lazyswap swap 0.50 BNB USDT       # swap BNB → USDT
lazyswap swap 5 BNB USDT --yes    # skip confirmation
lazyswap swap 5 BNB USDT --api    # route via API (OpenOcean best rate; needs a Pass)
lazyswap config set-swap-mode api # make the API route the default
lazyswap wallets                  # list wallet addresses
lazyswap config show              # print chain/slippage/default-wallet/swap-mode
lazyswap referral status          # referral code, referred wallets, claimable USD (also: code|apply|claim)
lazyswap mcp                      # MCP server for AI agents (stdio); read-only unless --allow-trading --max-usd <n>
lazyswap help                     # full command reference
```

---

## Entry point & architecture

**`main.go`** — if `os.Args > 1` → `cli.Run()`; otherwise → launches the Bubble Tea TUI.

Layers: **TUI → Services → DAO / Blockchain**

```
main.go
  ├── internal/cli        — non-interactive commands (swap, wallets, config, set password, mcp, referral)
  ├── internal/mcp        — MCP stdio server for AI agents (delegates to the same services; never prints)
  ├── internal/safety     — pre-swap token risk check (GoPlus; fail-closed, cached; CLI+TUI+MCP)
  ├── internal/api        — lazyswap backend client: SIWE auth (JWT in memory only) + OpenOcean swap route
  │                         + Monitor client (bearer API key, price-monitor watchlist, no wallet link)
  └── internal/tui        — Bubble Tea screens / panels / overlays / theme / keys
        ├── internal/wallet     — wallet CRUD + SQLite DAO (modernc/sqlite, cgo-free)
        ├── internal/swap       — quote + execute orchestration (EVM + BTC)
        ├── internal/dex        — Uniswap V2 / PancakeSwap quote/swap
        ├── internal/thorchain  — cross-chain BTC swap routing
        ├── internal/balance    — balance fetch + formatting
        ├── internal/explorer   — block explorer API
        ├── internal/pass       — LazySwapPass ERC-721 mint/validity (on-chain only)
        ├── internal/settings   — persisted chain/slippage/default-wallet/swap-mode (shared CLI+TUI)
        ├── internal/crypto     — AES-256-GCM + PBKDF2 (100k iters) key encryption
        ├── internal/chain      — CHAINS map + contract ABIs (SSOT for all chain config)
        ├── internal/paths      — filesystem SSOT (~/.lazyswap/, LAZYSWAP_DATA_DIR)
        └── internal/applog     — file logger (~/.lazyswap/lazyswap.log, never stdout)
```

---

## Key directories

| Path | Purpose |
|---|---|
| `main.go` | Entry point |
| `internal/chain/config.go` | **CHAINS map** — single source of truth for RPC URLs, router/token addresses |
| `internal/tui/` | Bubble Tea model, screens, panels, overlays, theme, keybindings |
| `internal/tui/screens/mainscreen/` | Main screen — split into `main.go`, `layout.go`, `messages.go`, `update.go`, `update_keys.go`, `update_panels.go`, `update_swap.go`, `update_wallet.go`, `update_alerts.go` |
| `internal/cli/` | Non-interactive CLI commands |
| `internal/mcp/` | MCP stdio server — read-only tools by default; `swap_execute`/`buy_pass` only with `--allow-trading` + `--max-usd` cap, password via `LAZYSWAP_PASSWORD` env only |
| `internal/safety/` | Pre-swap token risk check (GoPlus). Fail-closed: missing data/API failure → "unknown", never "safe". Testnets are always unknown (GoPlus has no coverage) |
| `internal/api/` | Backend client for hybrid swap. SIWE JWT lives in memory only; swap tx is signed/broadcast locally (`swap.Flow.ExecuteRawTx`). Also contains `Monitor` — bearer-key client for the price-monitor watchlist (no SIWE, no wallet link; key stored plaintext in `app_config` as `monitor_api_key`). API route needs `chain.Config.OpenOceanKey` (mainnets only); every front-end falls back to direct when the API is unavailable |
| `internal/wallet/` | Wallet CRUD + SQLite DAO. `pick.go` — `Pick()` resolves which wallet to use (explicit addr > default > only wallet) |
| `internal/crypto/` | AES-256-GCM + PBKDF2 encryption |
| `internal/pass/` | LazySwapPass ERC-721 (deployed on `bsc_testnet` only) |
| `internal/paths/` | Filesystem path resolution |
| `internal/applog/` | File-based logger |
| `internal/swap/` | Split: `flow.go` (Flow struct + ConvertUsdToTokenAmount), `flow_quote.go` (Flow.Quote), `flow_execute.go` (Flow.Execute), `flow_thorchain.go` (GetThorchainQuote/EstimateThorchain), `rawtx.go` (RawTx + ExecuteRawTx), `resolve.go` (ResolveToken/AvailableSymbols), `executor.go`, `fee.go` |
| `internal/tui/panels/alerts/` | Alerts tab (tab 3) — token price-monitor watchlist; keys: `j`/`k` navigate, `x` remove, `t` Telegram link, `R` register. Tokens added from Tokens tab with `a` |
| `internal/tui/panels/referral/` | Referral tab (tab 2) — display-only; shows referral code, earnings, claimable USD |
| `internal/tui/overlays/swapoverlay/` | Split: `swap.go` (Model/Update), `items.go` (Item + list builders), `view.go` (View) |
| `internal/tui/panels/swapbtc/` | Split: `panel.go` (Model/state), `messages.go` (all message types), `update.go` (Update/handleKey), `view.go` (View) |
| `e2e/` | End-to-end tests (build tag `e2e`): CLI quote, TUI smoke, funded swap |
| `internal/testrpc/` | Fake EVM JSON-RPC server for unit tests (getAmountsOut/balanceOf/allowance + tx lifecycle). Configure `RateNum`/`RateDen` for pricing, `Allowance`, `NativeBalance`, `TokenBalance`, `NoLiquidity`, `RevertTx`. Excluded from coverage denominator |
| `internal/cli/testdata/` | Golden files for CLI output; regenerate with `-update` flag |
| `scripts/build-release.sh` | Cross-compile release tarballs → `dist/` |
| `scripts/coverage-gate.sh` | Coverage gate script: ≥ 70% ex-TUI and ex-testrpc |
| `Makefile` | Local dev targets: `build`, `lint`, `test`, `cover`, `e2e`, `mutate`, `gate` |
| `REVIEW.md` | PR review checklist and gate thresholds |
| `.releaserc.json` | semantic-release config |
| `.golangci.yml` | golangci-lint config (standard linters + cyclop ≤15, funlen ≤80, revive file-length ≤500) |
| `.gremlins.yaml` | Mutation-testing config (excludes `_test.go`, `internal/tui/`, `e2e/`, `main.go`) |
| `.midnax/` | Machine-generated org-wide context (do not edit by hand) |

---

## Critical conventions

### Chain config
- **`internal/chain/config.go` `CHAINS` is the single source of truth.** Never hardcode chain-specific values (RPC URLs, router addresses, token addresses) anywhere else.
- Supported chains: `ethereum`, `bsc`, `bsc_testnet`, `sepolia`. Default: `bsc`.
- `OrderedKeys` controls stable display order (Go maps iterate randomly).
- `chain.Config.OpenOceanKey` is non-empty only on mainnets (`eth`, `bsc`); empty = API swap mode unavailable on that chain.
- `LAZYSWAP_RPC_URL` env overrides every chain's RPC endpoint at runtime (used by tests and CI).

### Swap routing (hybrid)
- Two routes: **direct** (on-chain V2 router, no fee) and **api** (lazyswap backend → OpenOcean, MEV-protected, 1% fee, needs a LazySwapPass).
- The active route is persisted in `settings.SwapMode` (`""` / `"direct"` / `"api"`). The TUI swap overlay's `t` key toggles it; `lazyswap config set-swap-mode` sets it from CLI.
- `mainscreen.Model` tracks `swapMode` (persisted preference) and `lastQuoteMode` (route that produced the confirmed quote) — execution always follows `lastQuoteMode`.
- `FlowQuote.Mode` (`"direct"` or `"api"`) and `FlowQuote.PriceImpact` (API only) are JSON-tagged — they are part of the MCP/API surface; do not rename without updating MCP tool schemas.
- `swap.Flow.ExecuteRawTx` signs and broadcasts a backend-built `RawTx` locally; it refuses a tx whose `ChainID` doesn't match the connected chain.
- `swap.ResolveToken(chain.Config, symbol)` maps a symbol to `TokenInfo` (case-insensitive; native sentinel for the chain's native asset). `AvailableSymbols` lists swappable symbols for error messages.

### Price-monitor / Alerts tab
- `internal/api.Monitor` authenticates with a **bearer API key** (no SIWE, no wallet). Registration is anonymous (`POST /api/v1/auth/register`); the returned key is stored plaintext in `app_config` under `monitor_api_key` / `monitor_user_id` — it can only manage the watchlist, not funds.
- The Alerts panel (tab 3) is data-pushed: the parent (`mainscreen`) runs all API calls off the UI thread and pushes results back via typed messages (`monitorRegisteredMsg`, `monitorListMsg`, etc.).
- Default alert threshold: **±5%** (`defaultThresholdPct`). Edit via backend API (`PATCH /watchlist/:id`) until the panel grows an in-TUI editor.
- From the Tokens tab, press **`a`** to add the selected token to the watchlist.
- Tab numbering: `tabTokens=1`, `tabReferral=2`, `tabAlerts=3`, `tabSettings=4`, `tabSwapBTC=5`, `tabPass=6`.

### Logging / stdout
- **Outside `internal/cli`, never write to stdout/stderr** (`fmt.Print*`, `log`, `println`) — it corrupts the Bubble Tea TUI and the MCP JSON-RPC stream.
- Use `internal/applog` everywhere else. It writes to `~/.lazyswap/lazyswap.log` and never panics.
- `LAZYSWAP_TEST=1` routes the log to `/dev/null`.

### Filesystem paths
- **All filesystem paths via `internal/paths` only.** Data dir: `~/.lazyswap/`; override with `LAZYSWAP_DATA_DIR`.
- `paths.Override` + `applog.SetPath` are used to isolate tests.

### Security
- Private keys are handled exclusively in `internal/wallet` + `internal/crypto`. Never log or print a private key in plaintext.
- `lazyswap set password` emits `export LAZYSWAP_PASSWORD=…` only when stdout is not a TTY (captured), to avoid terminal leakage.
- MCP trading mode reads the password from `LAZYSWAP_PASSWORD` env only — never from a tool parameter.
- Monitor API key is stored plaintext (it controls only the watchlist — no funds, no wallet link).

### Testing
- Test behavior and data only. **Do not test `View()` / layout / ASCII art** — they churn. TUI e2e smoke (teatest) asserts stable substrings only.
- Useful env vars in tests: `LAZYSWAP_PASSWORD` (skips prompt), `LAZYSWAP_DATA_DIR`, `LAZYSWAP_TEST=1`, `LAZYSWAP_RPC_URL` (overrides every chain's RPC — point it at `internal/testrpc`), `LAZYSWAP_API_URL` (overrides backend URL), `LAZYSWAP_THORNODE_URL` (overrides the THORnode API).
- `internal/testrpc` is the fake EVM JSON-RPC for unit tests (getAmountsOut/balanceOf/allowance + tx lifecycle). Configure `RateNum`/`RateDen` for pricing, `Allowance`, `NativeBalance`, `TokenBalance`, `NoLiquidity`, `RevertTx`. It is excluded from the coverage denominator.
- CLI goldens live in `internal/cli/testdata/*.golden`; regenerate with `go test ./internal/cli/ -update` and review the diff like code.
- E2E env: `LAZYSWAP_E2E_MNEMONIC` (funded bsc_testnet wallet — nightly only).

### TS parity
- Most packages mirror a TypeScript file in `lazyswap-old/` (noted as `// Mirrors src/...`). When changing behavior, keep parity with the Bun reference unless intentionally diverging.

### Release
- Version injected via `-ldflags "-X .../internal/cli.version=..."`. Default: `"dev"`.
- **Conventional Commits** required — semantic-release derives versions from commit history.
- `master` → `beta` prereleases; `stable` → full releases.
- **Never commit:** compiled binary (`lazyswap`), `dist/`, `*.db`, `*.log`, `.claude/`.

---

## CI/CD

Three GitHub Actions workflows:

**`.github/workflows/ci.yml`** — runs on every PR and push to `master`:
1. `lint` — `golangci-lint` (config: `.golangci.yml`).
2. `test` — coverage gate ≥ 70% ex-TUI (`scripts/coverage-gate.sh`); uploads `coverage.out` artifact.
3. `e2e` (needs `test`) — `go test -tags e2e -race ./e2e/...` with `LAZYSWAP_TEST=1`; funded swap self-skips (no `LAZYSWAP_E2E_MNEMONIC`).
4. `mutation-report` — report-only (`continue-on-error: true`); runs gremlins, posts efficacy to job summary; never blocks.

**`.github/workflows/nightly.yml`** — runs on schedule (`0 3 * * *`) and `workflow_dispatch`:
1. `mutation` — **blocking**; fails when efficacy < 50% (`--threshold-efficacy 50`).
2. `funded-e2e` — real tiny swap on `bsc_testnet`; runs only when `LAZYSWAP_E2E_MNEMONIC` secret is set.

**`.github/workflows/release.yml`** — runs on push to `master`:
1. Runs `scripts/build-release.sh` → cross-compiled tarballs in `dist/` (`CGO_ENABLED=0`; targets: `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`).
2. Runs `npx semantic-release@24` → creates GitHub Release with tarballs + SHA256 checksums if releasable commits are present.

See also `.midnax/ci-cd.md` for org-wide CI/CD context.

---

## Data & runtime

- Data directory: `~/.lazyswap/` (`wallets.db`, `lazyswap.log`). Override: `LAZYSWAP_DATA_DIR`.
- Connects at runtime to public EVM RPC endpoints (configured in `CHAINS`) and THORchain API.
- Backend API base URL: `https://api.lazyswap.org`; override with `LAZYSWAP_API_URL`.
- No server, no backend container — pure local binary.

---

## LazySwapPass (ERC-721)

`internal/pass` implements on-chain mint + validity/expiry reads for the LazySwapPass NFT. Surfaced as TUI tab 6 ("Lazyswap Pass"). Currently deployed on `bsc_testnet` only (`chain.Config.PassAddress`). Empty `PassAddress` = feature is inert on that chain. Required for the API swap route.
