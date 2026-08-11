## Organization context

This repository is part of the **FernandoPazCavalcante's Org** organization. Shared org-wide context (overview, tech stack, CI/CD, observability, infrastructure, ticket workflow, integrations) lives in [`.midnax/`](./.midnax/) — read `.midnax/overview.md` first, then the domain files.

## What this is

**lazyswap** is a self-custody terminal wallet — a Vim-style **TUI and a non-interactive CLI** — that executes crypto swaps directly on-chain from your machine. No exchange account, no custodian. It supports EVM chains (Ethereum, BSC) via Uniswap V2 / PancakeSwap, and cross-chain BTC swaps via THORchain.

Go rewrite of the original Bun/TypeScript app (reference: `lazyswap-old/`). Module: `github.com/FernandoPazCavalcante/lazyswap`. Requires **Go 1.26+**.

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
lazyswap wallets                  # list wallet addresses
lazyswap config show              # print chain/slippage/default-wallet
lazyswap mcp                      # MCP server for AI agents (stdio); read-only unless --allow-trading --max-usd <n>
lazyswap help                     # full command reference
```

---

## Entry point & architecture

**`main.go`** — if `os.Args > 1` → `cli.Run()`; otherwise → launches the Bubble Tea TUI.

Layers: **TUI → Services → DAO / Blockchain**

```
main.go
  ├── internal/cli        — non-interactive commands (swap, wallets, config, set password, mcp)
  ├── internal/mcp        — MCP stdio server for AI agents (delegates to the same services; never prints)
  ├── internal/safety     — pre-swap token risk check (GoPlus; fail-closed, cached; CLI+TUI+MCP)
  ├── internal/api        — lazyswap backend client: SIWE auth (JWT in memory only) + OpenOcean swap route
  └── internal/tui        — Bubble Tea screens / panels / overlays / theme / keys
        ├── internal/wallet     — wallet CRUD + SQLite DAO (modernc/sqlite, cgo-free)
        ├── internal/swap       — quote + execute orchestration (EVM + BTC)
        ├── internal/dex        — Uniswap V2 / PancakeSwap quote/swap
        ├── internal/thorchain  — cross-chain BTC swap routing
        ├── internal/balance    — balance fetch + formatting
        ├── internal/explorer   — block explorer API
        ├── internal/pass       — LazySwapPass ERC-721 mint/validity (on-chain only)
        ├── internal/settings   — persisted chain/slippage/default-wallet (shared CLI+TUI)
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
| `internal/cli/` | Non-interactive CLI commands |
| `internal/mcp/` | MCP stdio server — read-only tools by default; `swap_execute`/`buy_pass` only with `--allow-trading` + `--max-usd` cap, password via `LAZYSWAP_PASSWORD` env only |
| `internal/safety/` | Pre-swap token risk check (GoPlus). Fail-closed: missing data/API failure → "unknown", never "safe". Testnets are always unknown (GoPlus has no coverage) |
| `internal/api/` | Backend client for hybrid swap. SIWE JWT lives in memory only; swap tx is signed/broadcast locally (`swap.Flow.ExecuteRawTx`). API route needs `chain.Config.OpenOceanKey` (mainnets only); every front-end falls back to direct when the API is unavailable |
| `internal/wallet/` | Wallet CRUD + SQLite DAO |
| `internal/crypto/` | AES-256-GCM + PBKDF2 encryption |
| `internal/pass/` | LazySwapPass ERC-721 (deployed on `bsc_testnet` only) |
| `internal/paths/` | Filesystem path resolution |
| `internal/applog/` | File-based logger |
| `scripts/build-release.sh` | Cross-compile release tarballs → `dist/` |
| `.releaserc.json` | semantic-release config |
| `.midnax/` | Machine-generated org-wide context (do not edit by hand) |

---

## Critical conventions

### Chain config
- **`internal/chain/config.go` `CHAINS` is the single source of truth.** Never hardcode chain-specific values (RPC URLs, router addresses, token addresses) anywhere else.
- Supported chains: `ethereum`, `bsc`, `bsc_testnet`, `sepolia`. Default: `bsc`.
- `OrderedKeys` controls stable display order (Go maps iterate randomly).

### Logging / stdout
- **Outside `internal/cli`, never write to stdout/stderr** (`fmt.Print*`, `log`, `println`) — it corrupts the Bubble Tea TUI.
- Use `internal/applog` everywhere else. It writes to `~/.lazyswap/lazyswap.log` and never panics.
- `LAZYSWAP_TEST=1` routes the log to `/dev/null`.

### Filesystem paths
- **All filesystem paths via `internal/paths` only.** Data dir: `~/.lazyswap/`; override with `LAZYSWAP_DATA_DIR`.
- `paths.Override` + `applog.SetPath` are used to isolate tests.

### Security
- Private keys are handled exclusively in `internal/wallet` + `internal/crypto`. Never log or print a private key in plaintext.
- `lazyswap set password` emits `export LAZYSWAP_PASSWORD=…` only when stdout is not a TTY (captured), to avoid terminal leakage.

### Testing
- Test behavior and data only. **Do not test `View()` / layout / ASCII art** — they churn. TUI e2e smoke (teatest) asserts stable substrings only.
- Useful env vars in tests: `LAZYSWAP_PASSWORD` (skips prompt), `LAZYSWAP_DATA_DIR`, `LAZYSWAP_TEST=1`, `LAZYSWAP_RPC_URL` (overrides every chain's RPC — point it at `internal/testrpc`), `LAZYSWAP_THORNODE_URL` (overrides the THORnode API).
- `internal/testrpc` is the fake EVM JSON-RPC for unit tests (getAmountsOut/balanceOf/allowance + tx lifecycle). It is excluded from the coverage denominator.
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

GitHub Actions (`.github/workflows/release.yml`):
1. Triggered on push to `master`.
2. Runs `scripts/build-release.sh` → cross-compiled tarballs in `dist/`.
3. Runs `npx semantic-release@24` → creates GitHub Release with tarballs + SHA256 checksums if releasable commits are present.

See also `.midnax/ci-cd.md` for org-wide CI/CD context.

---

## Data & runtime

- Data directory: `~/.lazyswap/` (`wallets.db`, `lazyswap.log`). Override: `LAZYSWAP_DATA_DIR`.
- Connects at runtime to public EVM RPC endpoints (configured in `CHAINS`) and THORchain API.
- No server, no backend, no container — pure local binary.

---

## LazySwapPass (ERC-721)

`internal/pass` implements on-chain mint + validity/expiry reads for the LazySwapPass NFT. Surfaced as TUI tab 6 ("Lazyswap Pass"). Currently deployed on `bsc_testnet` only (`chain.Config.PassAddress`). Empty `PassAddress` = feature is inert on that chain.
