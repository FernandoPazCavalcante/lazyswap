# lazyswap

[![release](https://github.com/FernandoPazCavalcante/lazyswap/actions/workflows/release.yml/badge.svg)](https://github.com/FernandoPazCavalcante/lazyswap/actions/workflows/release.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/FernandoPazCavalcante/lazyswap)](go.mod)
[![License: MIT](https://img.shields.io/github/license/FernandoPazCavalcante/lazyswap)](LICENSE)

A Vim-style terminal wallet — a **TUI and a CLI** — that swaps crypto
**directly on-chain**, from your machine. Drive it interactively or script it
from the command line. No exchange account, no deposits, no custodian. You hold
the keys; the trade goes straight to a DEX.

Runs on EVM chains (Ethereum, BSC) for on-chain DEX swaps (Uniswap V2 /
PancakeSwap), plus cross-chain BTC swaps via THORchain.

## Highlights

- **Cross-chain swaps to native BTC.** Send an EVM token and receive native
  Bitcoin, routed through THORchain — no bridge, no wrapped BTC. One direction
  only today: EVM → BTC. BTC → EVM is not implemented.
- **Self-custody, no exchange.** You hold the keys; trades go straight to a DEX.
  No account, no KYC, no deposits, no withdrawal queue.
- **Encrypted on disk.** Your private key is sealed with AES-256-GCM under a
  PBKDF2-derived key (100k iterations) and never stored or logged in plaintext.
- **Vim-style TUI + scriptable CLI.** A fast, keyboard-driven terminal UI, plus
  a non-interactive CLI for one-shot swaps in scripts and pipelines.
- **Pre-swap safety check.** Before you sign, the token you're buying is
  risk-checked (honeypot, buy/sell tax, owner powers) via GoPlus and the
  verdict is shown inline — advisory by default, `--safety-block` to refuse
  HIGH-risk swaps. Fail-closed: an unreachable risk API warns, never passes.
- **Multi-chain EVM.** Ethereum and BSC today, with chain config in one place —
  RPC URLs, routers and token addresses are never hardcoded elsewhere.
- **No fee.** The amount you ask to swap is the amount that reaches the
  router. lazyswap takes nothing — see [docs/FEES.md](docs/FEES.md) for what
  you *do* pay (gas and the DEX pool's own fee) and who receives it.

## Usage

Launch the TUI, or drive everything from the command line.

![lazyswap demo](docs/demo.gif)

```bash
lazyswap                          # launch the Vim-style TUI (first run creates a wallet)

lazyswap swap 0.50 BNB USDT       # swap $0.50 worth of BNB into USDT
lazyswap swap 5 BNB USDT --yes    # skip the y/N confirmation (handy in scripts)
lazyswap config set-swap-mode api # make the API route the default
lazyswap wallets                  # list wallet addresses
lazyswap config show              # print current chain / slippage / default wallet
lazyswap mcp                      # serve the MCP server for AI agents (stdio)
lazyswap help                     # full command reference
```

Set `LAZYSWAP_PASSWORD` to skip the interactive password prompt when scripting.

## MCP server (AI agents)

`lazyswap mcp` serves your wallet to AI agents (Claude Code, Cursor, …) over
the Model Context Protocol on stdio. **Read-only by default**: agents can list
chains and wallets, fetch balances, quote swaps, and read/write settings — but
not trade.

```bash
claude mcp add lazyswap -- lazyswap mcp        # register with Claude Code
```

Or in `.mcp.json`:

```json
{
  "mcpServers": {
    "lazyswap": { "command": "lazyswap", "args": ["mcp"] }
  }
}
```

To let an agent actually execute swaps, opt in explicitly:

```bash
eval "$(lazyswap set password)"   # exports LAZYSWAP_PASSWORD without touching your shell history
lazyswap mcp --allow-trading --max-usd 20 --max-usd-total 60 --chain bsc
```

- `--allow-trading` registers the `swap_execute` tool; without it the tool does
  not exist, so an agent acting on an injected instruction has nothing to call.
- `--max-usd` is a code-enforced **per-swap** cap (required with trading).
- `--max-usd-total` is the **cumulative** budget for the whole server run, and
  it is the control that matters. A per-swap cap on its own is a rate limit, not
  a budget: an agent can call a $20 swap in a loop. The budget is booked before
  signing, so provoking failures cannot be used to retry past it. It defaults to
  `--max-usd` — i.e. **one swap** — so the conservative behaviour is what you
  get without thinking about it.
- `--chain` optionally restricts trading to an allowlist of chains.
- Every `swap_execute` attempt — allowed, refused or failed — is appended to
  `~/.lazyswap/lazyswap.log` with the amount and the remaining budget, so you
  can audit what an agent did after the fact.
- Tools carry MCP annotations: the read-only ones are marked `readOnlyHint` and
  `swap_execute` is marked `destructiveHint`, which is what makes a well-behaved
  host prompt a human before it moves funds.
- The wallet password comes from the `LAZYSWAP_PASSWORD` environment only —
  it is never a tool parameter, so it cannot be echoed back through a tool
  call. Note the agent host's process environment is inside the trust
  boundary: anything that can read it can read the password.

## Installation

```bash
go install github.com/FernandoPazCavalcante/lazyswap@latest
```

Needs **Go 1.26+** (and a C toolchain — the SQLite driver is cgo-free, but
go-ethereum pulls in cgo on some platforms).

> **Pre-release:** no stable version is tagged yet, so `@latest` tracks the
> `v0.0.0` development tag. For the newest code, use `@master` or
> [build from source](#build-from-source).

Once installed, run `lazyswap` (if `$GOBIN` is on your `PATH`). Data lives in
`~/.lazyswap/` (`wallets.db`, `lazyswap.log`); override with `LAZYSWAP_DATA_DIR`.
First launch creates a wallet; your private key is encrypted with AES-256-GCM
under a PBKDF2-derived key (100k iterations) and never leaves the box in
plaintext.

## Local wallet vs. an exchange: the honest trade-off

Self-custody is a different risk profile, not a strictly better one. Which side
you want depends on what you are afraid of.

| | lazyswap | A centralized exchange |
|---|---|---|
| Who holds the coins | You | The exchange (Mt. Gox, FTX) |
| Account can be frozen or closed | No | Yes |
| KYC / region lock | None | Usually |
| Trade is publicly verifiable | Yes, it is a transaction | No, an internal ledger entry |
| **If you lose your password** | **Funds are gone. There is no reset and, today, no seed export.** | Account recovery, support, 2FA reset |
| If *they* get breached | Your keys are on your machine | Your coins are in their hot wallet |
| Protects you from your own mistake | No — a signed transaction is final | Sometimes: address whitelists, holds |
| Cost | Network gas + the DEX pool fee | Trading fee + withdrawal fee |

The row in bold is the one that actually decides it for most people, and it is
why this is not "strictly safer". An exchange has a human you can email. A
self-custody wallet has you.

## What lazyswap does not do

Stated plainly, so you can rule it out quickly:

- **No seed phrase export yet.** A wallet created in-app cannot currently be
  revealed or exported, so it cannot be backed up or restored elsewhere. Until
  that ships, import a wallet you already back up rather than generating one
  here. This is the next thing being fixed.
- **No pre-sign simulation.** Browser wallets like Rabby simulate a transaction
  and show you the balance changes before you approve. lazyswap does not — it
  shows you a quote, then sends the transaction.
- **No hardware wallet support.** No Ledger, no Trezor. The key lives in an
  encrypted file on disk.
- **Two mainnets, two DEXes, V2 routers only.** Ethereum and BSC, via Uniswap V2
  and PancakeSwap. No V3, no aggregation.
- **EVM → BTC only** on the cross-chain path, and its 3% slippage tolerance is
  not yet configurable.
- **No Windows build.** Linux and macOS.
- **Not audited.** See [SECURITY.md](SECURITY.md) for the known weaknesses,
  written down by the author rather than left to be discovered.

## Architecture

```
                          ┌──────────────┐
                          │   main.go    │  open DAO, build TUI, run
                          └──────┬───────┘
                                 │
                     ┌───────────▼────────────┐
                     │  TUI  (Bubble Tea)      │  internal/tui
                     │  screens · panels ·     │  login, mainscreen,
                     │  overlays · theme · keys│  swap/import overlays
                     └───────────┬────────────┘
                                 │ calls
              ┌──────────────────┼──────────────────────┐
              │                  │                       │
       ┌──────▼──────┐    ┌──────▼───────┐        ┌──────▼───────┐
       │   wallet    │    │     swap     │        │   balance    │
       │ CRUD + DAO  │    │ orchestration│        │  fetch/format│
       └──────┬──────┘    └──────┬───────┘        └──────┬───────┘
              │                  │                       │
      ┌───────▼───────┐   ┌──────▼──────┬─────────┐      │
      │    crypto     │   │     dex     │ thorchain│      │
      │ AES-256-GCM   │   │ Uniswap V2 /│ cross-   │      │
      │ + PBKDF2      │   │ PancakeSwap │ chain BTC│      │
      └───────┬───────┘   └──────┬──────┴────┬─────┘      │
              │                  │           │            │
       ┌──────▼──────┐    ┌──────▼───────────▼────────────▼──────┐
       │  SQLite DAO │    │     chain config  ·  explorer API    │
       │ wallets.db  │    │  RPC URLs, routers, token addresses  │
       └─────────────┘    └──────────────────────────────────────┘
                                       │
                                ┌──────▼──────┐
                                │  EVM RPC /  │  on-chain
                                │ DEX router  │  (your signed tx)
                                └─────────────┘
```

Layers: **TUI → Services → DAO / Blockchain**. `internal/chain/config.go` is the
single source of truth for RPC URLs, router and token addresses — nothing
chain-specific is hardcoded elsewhere. `internal/paths` owns filesystem
locations; `internal/applog` writes to `lazyswap.log` (never stdout).

## Build from source

```bash
git clone https://github.com/FernandoPazCavalcante/lazyswap.git
cd lazyswap
go build -o lazyswap .   # produces ./lazyswap
go run .                 # or run straight from source, no binary
go test ./...            # run the test suite
```

## Contributing

Contributions are welcome, and the repo is set up so you can work on a wallet
without risking any money: `go test ./...` runs against a fake EVM JSON-RPC
server, and no workflow triggered by a pull request has access to a wallet.

```bash
make setup   # installs the pinned lint + mutation tools
make gate    # the same gate CI runs on your PR
```

- **[CONTRIBUTING.md](CONTRIBUTING.md)** — review cadence, the AI policy, the
  open-core boundary, DCO, and where PRs are welcome (and where to open an
  issue first).
- **[ARCHITECTURE.md](ARCHITECTURE.md)** — how the three front-ends, the
  services and the chain layer fit together. The two easiest places to
  contribute are a new DEX adapter (`internal/dex` is 76 lines) and a new chain
  (`internal/chain/config.go` is one entry per chain).
- **[good first issues](https://github.com/FernandoPazCavalcante/lazyswap/contribute)**
  — each one names the file to start in.

I review PRs on weekends; expect a decision within a week.

## Author

Built by **Fernando Paz Cavalcante** ([@FernandoPazCavalcante](https://github.com/FernandoPazCavalcante)).

## License

[MIT](LICENSE)
