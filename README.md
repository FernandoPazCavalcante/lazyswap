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
lazyswap                          # launch the Vim-style TUI (press c to create a wallet)

lazyswap swap 0.50 BNB USDT       # swap $0.50 worth of BNB into USDT
lazyswap swap 5 BNB USDT --yes    # skip the y/N confirmation (handy in scripts)
lazyswap wallets                  # list wallet addresses
lazyswap wallets reveal           # print the recovery phrase (asks for the password)
lazyswap wallets export --keystore ~/backup.json  # encrypted keystore for MetaMask/Rabby/geth
lazyswap config show              # print current chain / slippage / default wallet
lazyswap mcp                      # serve the MCP server for AI agents (stdio)
lazyswap update                   # replace this binary with the newest release
lazyswap help                     # full command reference
```

Set `LAZYSWAP_PASSWORD` to skip the interactive password prompt when scripting.

### Backing up a wallet

Your lazyswap password protects the file on this machine; it does **not** back
anything up. The 12-word recovery phrase does.

- **Creating a wallet** in the TUI shows the phrase first and asks you to type
  back two of its words. The wallet is only saved once you do — esc discards it,
  so you can never fund a wallet whose phrase you never saw.
- **Seeing the phrase again:** press `b` in the TUI (it re-asks your password),
  or run `lazyswap wallets reveal`. The CLI refuses to print the phrase into a
  pipe or file unless you pass `--allow-pipe`.
- **Moving to another wallet app:** `lazyswap wallets export --keystore <path>`
  writes a standard keystore V3 file (mode `0600`, never overwrites) encrypted
  with a separate passphrase of your choice. MetaMask, Rabby and geth
  import it. The phrase restores the wallet anywhere that speaks BIP-39, at
  derivation path `m/44'/60'/0'/0/0`.

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
curl -sSL https://get.lazyswap.app | bash
```

Detects your platform, verifies the published SHA-256, and installs to the
first writable of `/usr/local/bin`, `~/.local/bin`, `~/bin`. Pin a release with
`LAZYSWAP_VERSION=v1.0.0-beta.1`, or choose the directory with
`LAZYSWAP_BIN_DIR=~/bin`. To remove it later:
`curl -sSL https://get.lazyswap.app/uninstall.sh | bash`.

If you would rather not pipe a script into a shell, the same tarballs are on
the [releases page](https://github.com/FernandoPazCavalcante/lazyswap/releases),
each with a `.sha256` beside it:

```bash
VERSION=v1.0.0-beta.1          # or whatever the latest release is
TARGET=linux-x64               # linux-x64 | linux-arm64 | darwin-x64 | darwin-arm64
BASE=https://github.com/FernandoPazCavalcante/lazyswap/releases/download

curl -fsSLO "$BASE/$VERSION/lazyswap-$TARGET.tar.gz"
curl -fsSLO "$BASE/$VERSION/lazyswap-$TARGET.tar.gz.sha256"
sha256sum -c "lazyswap-$TARGET.tar.gz.sha256"   # shasum -a 256 -c on macOS
tar -xzf "lazyswap-$TARGET.tar.gz"
install -m 0755 lazyswap ~/.local/bin/
```

Linux and macOS, x64 and arm64. There is no Windows build.

### With Go

```bash
go install github.com/FernandoPazCavalcante/lazyswap@master
```

Needs **Go 1.26+**. The build is cgo-free, so no C toolchain is required.

> **Pre-release:** no stable version is tagged yet, so `@latest` resolves to the
> `v0.0.0` development tag. Use `@master` until the stable line ships.

### First run

Run `lazyswap` (if you installed with Go, make sure `$GOBIN` is on your
`PATH`). **No wallet is created for you:** press `c` in the TUI, write down the
12-word recovery phrase it shows you, and confirm two of its words — the wallet
is only saved after that, so you cannot end up funding a wallet whose phrase you
never saw. Press Esc instead and nothing is written.

Your private key is encrypted with AES-256-GCM under a PBKDF2-derived key
(100k iterations) and never leaves the box in plaintext. Data lives in
`~/.lazyswap/` (`wallets.db`, `lazyswap.log`); override with
`LAZYSWAP_DATA_DIR`.

### Updating

```bash
lazyswap update            # download and install the newest release
lazyswap update --check    # just report what is available
```

It downloads the build for your platform, **verifies it against the SHA-256
published beside it**, and only then replaces the binary — by renaming into
place, so an interrupted update leaves the previous binary working. Your
wallets live in `~/.lazyswap/` and are not touched.

Pin or roll back with `lazyswap update --version v1.0.0-beta.2`.

If the binary sits somewhere you cannot write, such as `/usr/local/bin`, the
command says so and stops rather than escalating by itself — rerun it with
`sudo`, or reinstall with the script above.

Worth knowing what the checksum does and does not prove: it catches a corrupted
or tampered download, but it is served from the same place as the binary, so it
is not a defence against a compromised release account. Releases are not signed
yet; [SECURITY.md](SECURITY.md) tracks that. `go install` and building from
source avoid the published binaries entirely.

## Local wallet vs. an exchange: the honest trade-off

Self-custody is a different risk profile, not a strictly better one. Which side
you want depends on what you are afraid of.

| | lazyswap | A centralized exchange |
|---|---|---|
| Who holds the coins | You | The exchange (Mt. Gox, FTX) |
| Account can be frozen or closed | No | Yes |
| KYC / region lock | None | Usually |
| Trade is publicly verifiable | Yes, it is a transaction | No, an internal ledger entry |
| **If you lose your password** | **Only your written-down recovery phrase gets you back in. No phrase, no funds — there is no reset.** | Account recovery, support, 2FA reset |
| If *they* get breached | Your keys are on your machine | Your coins are in their hot wallet |
| Protects you from your own mistake | No — a signed transaction is final | Sometimes: address whitelists, holds |
| Cost | Network gas + the DEX pool fee | Trading fee + withdrawal fee |

The row in bold is the one that actually decides it for most people, and it is
why this is not "strictly safer". An exchange has a human you can email. A
self-custody wallet has you.

## What lazyswap does not do

Stated plainly, so you can rule it out quickly:

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
