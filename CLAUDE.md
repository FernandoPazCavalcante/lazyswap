## Organization context

This repository is part of the **FernandoPazCavalcante's Org** organization. Shared org-wide context (overview, tech stack, CI/CD, observability, infrastructure, ticket workflow, integrations) lives in [`.midnax/`](./.midnax/) — read `.midnax/overview.md` first, then the domain files.

## What this is

**lazyswap** is a self-custody terminal wallet — a Vim-style **TUI and a non-interactive CLI** — that executes crypto swaps directly on-chain from your machine. No exchange account, no custodian. It supports EVM chains (Ethereum, BSC) via Uniswap V2 / PancakeSwap, and cross-chain BTC swaps via LI.FI and NEAR Intents (THORChain disabled).

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
bash scripts/build-release.sh           # CGO_ENABLED=0; targets: linux-x64/arm64, darwin-x64/arm64; version from git describe
bash scripts/build-release.sh v1.2.3    # stamp a version (CI passes the tag; default: git describe)

# Quality gates (see REVIEW.md for thresholds; CI enforces on every PR)
make setup                   # one-time: installs pinned golangci-lint + gremlins into $GOBIN
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
lazyswap                                        # launch TUI (no args)
lazyswap swap 0.50 BNB USDT                     # swap BNB → USDT
lazyswap swap 5 BNB USDT --yes                  # skip confirmation
lazyswap wallets                                # list wallet addresses
lazyswap wallets reveal                         # print recovery phrase (asks for password)
lazyswap wallets export --keystore key.json     # write encrypted keystore V3 JSON
lazyswap config show                            # print chain/slippage/default-wallet
lazyswap mcp                                    # MCP server for AI agents (stdio); read-only unless --allow-trading --max-usd <n>
lazyswap update                                 # self-update to the newest release (--check, --version <tag>)
lazyswap help                                   # full command reference
```

---

## Entry point & architecture

**`main.go`** — if `os.Args > 1` → `cli.Run()`; otherwise → launches the Bubble Tea TUI.

Layers: **TUI → Services → DAO / Blockchain**

```
main.go
  ├── internal/cli        — non-interactive commands (swap, wallets, config, set password, mcp)
  ├── internal/mcp        — MCP stdio server for AI agents (delegates to the same services; never prints)
  ├── internal/selfupdate — resolve + install a published release over the running binary (`lazyswap update`)
  ├── internal/safety     — pre-swap token risk check (GoPlus; fail-closed, cached; CLI+TUI+MCP)
  ├── internal/api        — Monitor client only: bearer API key, price-monitor watchlist, no wallet link
  │                         (SIWE auth + OpenOcean swap-routing client removed in v0.1.0)
  └── internal/tui        — Bubble Tea screens / panels / overlays / theme / keys
        ├── internal/wallet     — wallet CRUD + SQLite DAO (modernc/sqlite, cgo-free)
        ├── internal/swap       — quote + execute orchestration (EVM + BTC)
        ├── internal/dex        — Uniswap V2 / PancakeSwap quote/swap
        ├── internal/thorchain  — cross-chain BTC swap routing (quote/execute; disabled as active provider)
        ├── internal/balance    — balance fetch + formatting
        ├── internal/explorer   — block explorer API
        ├── internal/settings   — persisted chain/slippage/default-wallet (shared CLI+TUI)
        ├── internal/crypto     — AES-256-GCM + PBKDF2 (100k iters) key encryption
        ├── internal/chain      — CHAINS map + contract ABIs (SSOT for all chain config)
        ├── internal/paths      — filesystem SSOT (~/.lazyswap/, LAZYSWAP_DATA_DIR)
        └── internal/applog     — file logger (~/.lazyswap/lazyswap.log, never stdout)
```

> **Removed packages:** `internal/pass` (LazySwapPass service), `internal/tui/panels/lazyswappass`, `internal/tui/panels/referral`, `internal/swap/rawtx.go` (RawTx/ExecuteRawTx), and the SIWE/swap-routing parts of `internal/api` were deleted. The `referral` and `set-swap-mode` CLI subcommands were also removed.

---

## Key directories

| Path | Purpose |
|---|---|
| `main.go` | Entry point |
| `internal/chain/config.go` | **CHAINS map** — single source of truth for RPC URLs, router/token addresses; `NearIntentsChain` field identifies the chain name in the 1Click token catalog |
| `internal/tui/` | Bubble Tea model, screens, panels, overlays, theme, keybindings |
| `internal/tui/screens/mainscreen/` | Main screen — split into `main.go`, `layout.go`, `messages.go`, `update.go`, `update_keys.go`, `update_panels.go`, `update_swap.go`, `update_wallet.go`, `update_alerts.go` |
| `internal/cli/` | Non-interactive CLI commands |
| `internal/cli/wallets.go` | `wallets` / `wallets reveal` / `wallets export` subcommands |
| `internal/cli/update.go` | `lazyswap update` — delegates to `internal/selfupdate`; `newUpdater` and `exePath` are vars so tests can redirect to a local server |
| `internal/mcp/` | MCP stdio server — read-only tools by default; `swap_execute` only with `--allow-trading` + `--max-usd` cap + optional `--max-usd-total` cumulative budget; password via `LAZYSWAP_PASSWORD` env only |
| `internal/selfupdate/` | `lazyswap update`. Resolves the newest release (stable, else newest prerelease — same rule as the installer), verifies the published SHA-256, extracts only a regular file named `lazyswap`, and renames it over the running binary. **Download origin is compiled in — never add an env override**, and https-only including redirects. See the Supply chain section of `SECURITY.md`. Key types: `Updater`, `Release`. Key errors: `ErrNoAsset`, `ErrNotWritable`. `ExePath()` resolves symlinks before replacing. |
| `internal/safety/` | Pre-swap token risk check (GoPlus). Fail-closed: missing data/API failure → "unknown", never "safe". Testnets are always unknown (GoPlus has no coverage) |
| `internal/api/` | **Monitor only** — bearer-key client for the price-monitor watchlist (no SIWE, no wallet link; key stored plaintext in `app_config` as `monitor_api_key`). Base URL: `https://api.lazyswap.app`; override with `LAZYSWAP_API_URL`. The SIWE auth + OpenOcean swap-routing client was removed in v0.1.0. |
| `internal/wallet/` | Wallet CRUD + SQLite DAO. `pick.go` — `Pick()` resolves which wallet to use (explicit addr > default > only wallet). `export.go` — `GenerateMnemonic`, `Reveal`, `ExportKeystore`, `ErrNoMnemonic`, `ErrWalletNotFound` |
| `internal/crypto/` | AES-256-GCM + PBKDF2 encryption |
| `internal/paths/` | Filesystem path resolution |
| `internal/applog/` | File-based logger |
| `internal/swap/` | Split: `flow.go` (Flow struct + ConvertUsdToTokenAmount), `flow_quote.go` (Flow.Quote), `flow_execute.go` (Flow.Execute), `flow_thorchain.go` (GetThorchainQuote/ExecuteThorchain), `minoutput.go` (minOutputFor), `resolve.go` (ResolveToken/AvailableSymbols), `executor.go`. BTC provider abstraction: `btc_provider.go` (BTCProvider/ActiveBTCProviders catalog), `btc_quotes.go` (shared quote fetch + validation), `btc_lifi.go` (LI.FI route fetcher), `btc_near.go` (NEAR Intents fetcher). No fee logic — see `docs/FEES.md`. `rawtx.go` and `ExecuteRawTx` were deleted. `EstimateThorchain` was removed (merged into `GetThorchainQuote` with empty address). |
| `internal/settings/` | Persisted chain/slippage/default-wallet. `SwapMode` field and `set-swap-mode` command **removed**. |
| `internal/tui/overlays/seedoverlay/` | Backup quiz + reveal overlay. Two flows: **Backup** (shown after phrase generation — user must type back two random words before wallet is saved; Esc discards phrase, nothing is persisted); **Reveal** (re-prompts password, then shows phrase). Emits `ConfirmedMsg` (parent saves) or `CloseMsg`. Steps: `password → show → quiz → done`. `stepDone` is held from the moment the quiz passes until the parent finishes persisting; keys are inert in this state (Esc cannot discard a wallet already being created). |
| `internal/tui/panels/alerts/` | Alerts tab (tab 2) — token price-monitor watchlist; keys: `j`/`k` navigate, `x` remove, `t` Telegram link, `R` register. Tokens added from Tokens tab with `a` |
| `internal/tui/overlays/swapoverlay/` | Split: `swap.go` (Model/Update), `items.go` (Item + list builders), `view.go` (View) |
| `internal/tui/panels/swapbtc/` | EVM-to-BTC form, multi-provider quote comparison, and review. Split: `panel.go` (Model/state/New), `messages.go` (message types), `update.go` (Update/handleKey), `quotes.go` (quote ordering/selection/tick logic), `view.go` (View), `view_quotes.go` (quote table + review body). States: `stateForm → stateQuotes → stateAddress → stateReview → stateExecuting → stateDone`. `New(providers ...Provider)` takes provider IDs. `ShowingQuotes()` / `IsExecuting(id)` / `Reset()` / `SetNetwork(name)` are the key exported methods. |
| `e2e/` | End-to-end tests (build tag `e2e`): CLI quote, TUI smoke, funded swap |
| `internal/testrpc/` | Fake EVM JSON-RPC server for unit tests (getAmountsOut/balanceOf/allowance + tx lifecycle). Configure `RateNum`/`RateDen` for pricing, `Allowance`, `NativeBalance`, `TokenBalance`, `NoLiquidity`, `RevertTx`. Excluded from coverage denominator |
| `internal/cli/testdata/` | Golden files for CLI output; regenerate with `go test ./internal/cli/ -update` flag |
| `scripts/build-release.sh` | Cross-compile release tarballs → `dist/`; stamps version via `-X .../internal/cli.version=<ver>` ldflags; accepts optional version arg (default: `git describe --tags --always --dirty`) |
| `scripts/coverage-gate.sh` | Coverage gate script: ≥ 70% ex-TUI and ex-testrpc |
| `Makefile` | Local dev targets: `build`, `setup`, `lint`, `test`, `cover`, `e2e`, `mutate`, `gate` |
| `REVIEW.md` | PR review checklist and gate thresholds |
| `ARCHITECTURE.md` | Package map, the two contribution seams (add a DEX / add a chain), testrpc pattern |
| `CONTRIBUTING.md` | Review cadence, AI policy, open-core boundary, DCO sign-off requirement |
| `SECURITY.md` | Known weaknesses (key-at-rest, memory, backup/recovery, MCP, supply chain, selfupdate). Vulnerability reporting: GitHub private advisory, 7-day initial response, 90-day coordinated disclosure, no bounty. |
| `docs/FEES.md` | **Fee policy SSOT.** lazyswap takes no fee; DEX pool fees and THORchain fees documented here |
| `.releaserc.json` | semantic-release config (uses `@semantic-release/exec` to run `build-release.sh` in `prepare`) |
| `.golangci.yml` | golangci-lint config (standard linters + cyclop ≤15, funlen ≤80, revive file-length ≤500) |
| `.gremlins.yaml` | Mutation-testing config (excludes `_test.go`, `internal/tui/`, `e2e/`, `main.go`) |
| `.midnax/` | Machine-generated org-wide context (do not edit by hand) |

---

## Critical conventions

### Chain config
- **`internal/chain/config.go` `CHAINS` is the single source of truth.** Never hardcode chain-specific values (RPC URLs, router addresses, token addresses) anywhere else.
- Supported chains: `ethereum`, `bsc`, `bsc_testnet`, `sepolia`. Default: `bsc`.
- `NearIntentsChain` field on `Config` identifies the chain name used in the NEAR 1Click token catalog (`"eth"` for Ethereum, `"bsc"` for BSC).
- `OrderedKeys` controls stable display order (Go maps iterate randomly).
- `LAZYSWAP_RPC_URL` env overrides every chain's RPC endpoint at runtime (used by tests and CI).

### Fees
- **lazyswap takes no fee.** The full input amount reaches the router. `docs/FEES.md` is the single source of truth — if anything else disagrees, `docs/FEES.md` is right and the other place is a bug.
- `internal/swap/fee.go` and `CalcFee`/`SwapFeePercent`/`FeeBreakdown` **no longer exist** (deleted). `FlowQuote` has no `FeePercent`, `FeeAmount`, `NetFromTokenAmount`, `Mode`, or `PriceImpact` fields.
- Users pay: network gas (to validators), DEX pool fee (0.30% Uniswap V2 / 0.25% PancakeSwap V2, charged by the router, already in the quoted output), and THORchain network + outbound fees (shown in BTC quote). THORchain affiliate fee is 0 bps (`AffiliateBPS` in `internal/thorchain/config.go`).

### Swap routing (direct only)
- **Only one route remains: direct** (on-chain V2 router, no auth). The API route (SIWE + OpenOcean) was removed in v0.1.0 — the client code, `--api`/`--direct` flags, `set-swap-mode` command, `settings.SwapMode` field, `ModeToggleMsg`, `swapMode`/`lastQuoteMode`/`apiClient` fields, and `RawTx`/`ExecuteRawTx` are all gone.

### BTC swap providers
- **Active providers: LI.FI and NEAR Intents. THORChain is disabled** (`Enabled: false` in `ActiveBTCProviders()`).
- `internal/swap/btc_provider.go` — `BTCProvider` struct, `BTCProviderID` constants (`THORChain`, `LIFI`, `NEARIntents`), `ActiveBTCProviders()` catalog, `BTCQuoteRequest`, `BTCRouteQuote`.
- `internal/swap/btc_quotes.go` — shared `fetchProviderQuote` + `validateBTCProviderRequest` + `providerJSON` HTTP helper. `ErrBTCDestinationRequired` is returned by NEAR Intents when no BTC address is given (LI.FI can estimate without one).
- `internal/swap/btc_lifi.go` — LI.FI route fetcher (`https://li.quest/v1/advanced/routes`). THORswap routes are filtered out. API key via `LAZYSWAP_LIFI_API_KEY` env (optional).
- `internal/swap/btc_near.go` — NEAR Intents fetcher (`https://1click.chaindefuser.com/v0`). Requires a BTC destination address. API key via `LAZYSWAP_NEAR_INTENTS_API_KEY` env (optional).
- Both providers only support **quote review** (`QuoteOnly: true`); neither has an `Execute` func wired up. THORChain retains `Execute: (*Flow).ExecuteThorchain` but is disabled.
- `mainscreen` holds `btcProviders []swap.BTCProvider` and fans out a `QuoteRequestMsg` to all active providers in parallel (`tea.Batch`), feeding each result back as a `QuoteResultMsg`.
- On network change or wallet change, `mainscreen` calls `m.swapbtc.Reset()` to clear stale quotes.

### Min-output calculation
- `internal/swap/minoutput.go` `minOutputFor(estimatedOutput, slippage, decimals)` computes the minimum acceptable output, formatted to the output token's own decimal precision (capped at 8 places to avoid float64 noise). It **refuses** (returns an error) rather than produce a zero or near-zero `amountOutMin` — a swap with no slippage protection. Both `Flow.Quote` and `Flow.Execute` use it.

### FlowQuote fields (BTC-relevant)
- `ExpiresAt time.Time` — quote validity deadline (not serialised to JSON). Providers that omit an expiry get `time.Now() + 30s`.
- `SourceGasBudget string` — estimated EVM gas cost in native currency (e.g. `"0.0003 BNB"`), not serialised.
- `ThorInbound string` — vault address recorded in a reviewed THORChain quote, not serialised.
- `ThorBelowMin bool` — **removed**.

### THORChain quote changes
- `GetThorchainQuote` now accepts an empty `btcAddress` (estimate-only, no memo). `EstimateThorchain` was deleted.
- `thorchain.SwapQuote.GasLimit` is now `int64` (was `string`). `ExpiresAt time.Time` and `InboundAddress string` added.
- `thorchain.executor` — `SwapNativeToBtc` and `SwapErc20ToBtc` now accept `expiresAt time.Time`; the context deadline and the `depositWithExpiry` on-chain expiry are both derived from it.
- `MemoSlippageBPS = 300` constant added; `MemoSlippage = float64(MemoSlippageBPS)/10000`. `BTCEstimatedSeconds` fallback constant removed.
- `liquidity_tolerance_bps=300` is sent to THORNode when a destination address is provided.

### Wallet creation — backup-first flow
- **A wallet is not saved until the user confirms the backup quiz.** `c` → confirm → `y` generates a phrase (via `wallet.GenerateMnemonic`) and opens the `seedoverlay` in Backup mode. The user must type back two randomly chosen words. Only on `ConfirmedMsg` does `saveConfirmedCmd` call `svc.Import(phrase)` and persist the wallet. Pressing Esc discards the phrase; nothing is written.
- `b` key in the TUI opens the Reveal overlay for the selected wallet (re-prompts password, then shows the phrase).
- `wallet.GenerateMnemonic()` — generates a fresh 12-word BIP-39 phrase without persisting anything.
- `wallet.Service.Reveal(address)` — returns the stored plaintext phrase; errors with `ErrNoMnemonic` if the row has no phrase, `ErrWalletNotFound` if the address is unknown.
- `wallet.Service.ExportKeystore(address, passphrase, outPath)` — writes keystore V3 JSON (go-ethereum standard scrypt, `0600`, `O_EXCL`). Passphrase must be ≥ 8 chars (`MinKeystorePassphrase`). Never overwrites.

### Seedoverlay `stepDone`
- After the quiz passes, the overlay transitions to `stepDone` before emitting `ConfirmedMsg`. The parent then calls `svc.Import` (PBKDF2 key derivation — takes noticeable time), so the overlay stays visible in this state. **Keys are inert in `stepDone`**: Esc cannot discard a wallet already being created, and Enter cannot re-run the quiz or re-emit `ConfirmedMsg`. `Model.Step()` returns `"done"` in this state.

### Wallets CLI (subcommands)
- `lazyswap wallets reveal [--address X] [--allow-pipe]` — prints the 12-word BIP-39 recovery phrase. Refuses to print to a non-TTY stdout unless `--allow-pipe` is given (a phrase in a pipe tends to end up in logs or shell history).
- `lazyswap wallets export --keystore <path> [--address X]` — writes an encrypted keystore V3 JSON file. Never overwrites an existing file. Passphrase from `LAZYSWAP_KEYSTORE_PASSPHRASE` env or interactive prompt.

### Self-update (`lazyswap update`)
- `lazyswap update` — downloads the newest release for the current platform, verifies SHA-256, and replaces the running binary via `rename(2)`. An interrupted update leaves the old binary intact.
- `lazyswap update --check` — reports available version, installs nothing.
- `lazyswap update --version <tag>` — installs an exact release tag.
- **The download origin (`https://api.github.com`, repo `FernandoPazCavalcante/lazyswap`) is compiled in.** There is no env override — a settable origin would be an attacker vector. Only tests set `Updater.APIBase`.
- Every redirect must stay on https. A release with no published checksum is refused. Only a regular file named exactly `lazyswap` is extracted from the tarball.
- If the binary directory is not writable, `ErrNotWritable` is returned with instructions to use `sudo` or reinstall. The command never escalates privileges on its own.
- Implementation: `internal/selfupdate/selfupdate.go` (resolve/pick), `internal/selfupdate/install.go` (download/verify/extract/rename). `selfupdate.Target(goos, goarch)` maps Go platform to release asset name (e.g. `linux-x64`).

### MCP server (cumulative budget)
- `--allow-trading` registers only `swap_execute` (not `buy_pass` — removed). Requires `--max-usd > 0`.
- `--max-usd-total <usd>` sets a **cumulative** USD budget for the server's lifetime. A per-swap cap alone is a rate limit; an agent acting on an injected instruction can call `swap_execute` in a loop. The cumulative budget bounds total loss. Must be ≥ `--max-usd` if set.
- Budget is consumed conservatively: a failed swap still counts against the total (an agent cannot retry past the ceiling by provoking failures).
- MCP tool annotations: read-only tools carry `ReadOnlyHint: true`; `swap_execute` carries `DestructiveHint: true`. MCP hosts use these to decide auto-permissions.

### Price-monitor / Alerts tab
- `internal/api.Monitor` authenticates with a **bearer API key** (no SIWE, no wallet). Registration is anonymous (`POST /api/v1/auth/register`); the returned key is stored plaintext in `app_config` under `monitor_api_key` / `monitor_user_id` — it can only manage the watchlist, not funds.
- The Alerts panel (tab **2**) is data-pushed: the parent (`mainscreen`) runs all API calls off the UI thread and pushes results back via typed messages (`monitorRegisteredMsg`, `monitorListMsg`, etc.).
- Default alert threshold: **±5%** (`defaultThresholdPct`). Edit via backend API (`PATCH /watchlist/:id`) until the panel grows an in-TUI editor.
- From the Tokens tab, press **`a`** to add the selected token to the watchlist.
- Tab numbering: `tabTokens=1`, `tabAlerts=2`, `tabSettings=3`, `tabSwapBTC=4`. (`tabReferral` and `tabPass` were removed.)

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
- `lazyswap wallets reveal` refuses to print to a non-TTY unless `--allow-pipe` is passed.
- MCP trading mode reads the password from `LAZYSWAP_PASSWORD` env only — never from a tool parameter.
- Monitor API key is stored plaintext (it controls only the watchlist — no funds, no wallet link).
- Known weaknesses (PBKDF2 cost, GCM nonce size, `FetchAll` decrypts all wallets, no password-change path, selfupdate checksum-not-signed, etc.) are documented in `SECURITY.md`.

### Testing
- Test behavior and data only. **Do not test `View()` / layout / ASCII art** — they churn. TUI e2e smoke (teatest) asserts stable substrings only.
- Useful env vars in tests: `LAZYSWAP_PASSWORD` (skips prompt), `LAZYSWAP_DATA_DIR`, `LAZYSWAP_TEST=1`, `LAZYSWAP_RPC_URL` (overrides every chain's RPC — point it at `internal/testrpc`), `LAZYSWAP_API_URL` (overrides backend URL), `LAZYSWAP_THORNODE_URL` (overrides the THORnode API), `LAZYSWAP_KEYSTORE_PASSPHRASE` (skips keystore export prompt), `LAZYSWAP_LIFI_API_KEY` (LI.FI partner key), `LAZYSWAP_NEAR_INTENTS_API_KEY` (NEAR 1Click partner key).
- `internal/testrpc` is the fake EVM JSON-RPC for unit tests (getAmountsOut/balanceOf/allowance + tx lifecycle). Configure `RateNum`/`RateDen` for pricing, `Allowance`, `NativeBalance`, `TokenBalance`, `NoLiquidity`, `RevertTx`. It is excluded from the coverage denominator.
- CLI goldens live in `internal/cli/testdata/*.golden`; regenerate with `go test ./internal/cli/ -update` and review the diff like code.
- E2E env: `LAZYSWAP_E2E_MNEMONIC` (funded bsc_testnet wallet — nightly only).
- `internal/wallet/export_test.go` — lowers keystore scrypt cost to `LightScryptN/P` via `init()` to keep tests fast; the format is identical.
- `internal/cli/update_test.go` — stubs `newUpdater` and `exePath` vars to point at a local `httptest.Server` shaped like the GitHub API; uses `selfupdate.Target` to skip on unsupported platforms.
- BTC provider tests use `btcTransport` / `quoteTransport` (function types implementing `http.RoundTripper`) to mock `http.DefaultTransport` without a real server.

### TS parity
- Most packages mirror a TypeScript file in `lazyswap-old/` (noted as `// Mirrors src/...`). When changing behavior, keep parity with the Bun reference unless intentionally diverging.

### Release
- Version injected via `-ldflags "-X .../internal/cli.version=<ver>"` by `scripts/build-release.sh`, which takes the version as `$1` (default: `git describe --tags --always --dirty`). semantic-release passes `${nextRelease.gitTag}` through the exec plugin's `prepareCmd`; a local build with no argument falls back to `git describe`.
- **Conventional Commits** required — semantic-release derives versions from commit history.
- `master` → `beta` prereleases; `stable` → full releases.
- **Never commit:** compiled binary (`lazyswap`), `dist/`, `*.db`, `*.log`, `.claude/`.

### Contributing
- Run `make setup` once on a fresh clone to install pinned tools (`golangci-lint v2.15.0`, `gremlins v0.5.0`). Then `make gate` mirrors the CI PR gate exactly.
- DCO sign-off required (`git commit -s`). No CLA.
- See `CONTRIBUTING.md` for review cadence, AI policy, and open-core boundary. See `ARCHITECTURE.md` for the two easiest contribution seams (new DEX adapter in `internal/dex`; new chain in `internal/chain/config.go`).

---

## CI/CD

Three GitHub Actions workflows:

**`.github/workflows/ci.yml`** — runs on every PR and push to `master`:
1. `lint` — `golangci-lint v2.15.0` (config: `.golangci.yml`).
2. `test` — coverage gate ≥ 70% ex-TUI (`scripts/coverage-gate.sh`); uploads `coverage.out` artifact.
3. `e2e` (needs `test`) — `go test -tags e2e -race ./e2e/...` with `LAZYSWAP_TEST=1`; funded swap self-skips (no `LAZYSWAP_E2E_MNEMONIC`).
4. `mutation-report` — report-only (`continue-on-error: true`); runs gremlins, posts efficacy to job summary; never blocks.

**`.github/workflows/nightly.yml`** — runs on schedule (`0 3 * * *`) and `workflow_dispatch`:
1. `mutation` — **blocking**; fails when efficacy < 50% (`--threshold-efficacy 50`).
2. `funded-e2e` — real tiny swap on `bsc_testnet`; runs only when `LAZYSWAP_E2E_MNEMONIC` secret is set.

**`.github/workflows/release.yml`** — runs on push to `master`:
1. Runs `npx -p semantic-release@24 -p @semantic-release/exec@7 semantic-release`.
2. The cross-compile is the exec plugin's `prepareCmd` (see `.releaserc.json`): `scripts/build-release.sh ${nextRelease.gitTag}` (`CGO_ENABLED=0`; targets `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`). Running it inside `prepare` stamps the binaries with the tag being published and skips the build entirely on pushes with no releasable commits.

See also `.midnax/ci-cd.md` for org-wide CI/CD context.

---

## Data & runtime

- Data directory: `~/.lazyswap/` (`wallets.db`, `lazyswap.log`). Override: `LAZYSWAP_DATA_DIR`.
- Connects at runtime to public EVM RPC endpoints (configured in `CHAINS`) and THORchain API.
- Backend API base URL: `https://api.lazyswap.app`; override with `LAZYSWAP_API_URL`.
- BTC quote providers: LI.FI (`https://li.quest/v1/advanced/routes`) and NEAR Intents (`https://1click.chaindefuser.com/v0`). Optional API keys via `LAZYSWAP_LIFI_API_KEY` and `LAZYSWAP_NEAR_INTENTS_API_KEY`.
- No server, no backend container — pure local binary.

---

## LazySwapPass & Referral (removed)

The `internal/pass` package, `internal/tui/panels/lazyswappass`, and `internal/tui/panels/referral` were **deleted**. The TUI Pass tab and referral tab no longer exist. `chain.Config.PassAddress` remains in the chain config as a field but is not consumed by any active service. The `referral` CLI subcommand was also removed.
