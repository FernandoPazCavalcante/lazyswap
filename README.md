# lazyswap

A Vim-style terminal wallet that swaps crypto **directly on-chain**, from your
machine. No exchange account, no deposits, no custodian. You hold the keys; the
trade goes straight to a DEX.

Go rewrite of the original Bun/TS TUI. Runs on EVM chains (Ethereum, BSC) for
on-chain DEX swaps (Uniswap V2 / PancakeSwap), plus cross-chain BTC swaps via
THORchain.

## Build & install

Needs **Go 1.26+** (and a C toolchain — the SQLite driver is cgo-free, but
go-ethereum pulls in cgo on some platforms).

```bash
# Install the binary to $GOBIN (or ~/go/bin) — no clone needed
go install github.com/FernandoPazCavalcante/lazyswap-tui@latest

# Or build from a clone
git clone https://github.com/FernandoPazCavalcante/lazyswap-tui.git
cd lazyswap-tui
go build -o lazyswap-tui .   # produces ./lazyswap-tui
```

Then run it:

```bash
./lazyswap-tui      # from a build
lazyswap-tui        # if installed and $GOBIN is on PATH
go run .            # straight from source, no binary
```

Data lives in `~/.lazyswap/` (`wallets.db`, `lazyswap.log`). Override with
`LAZYSWAP_DATA_DIR`. First launch creates a wallet; your private key is encrypted
with AES-256-GCM under a PBKDF2-derived key (100k iterations) and never leaves
the box in plaintext.

## Why local beats a centralized exchange

Running lazyswap on your own machine is strictly safer than trading on a CEX:

- **You keep custody.** Keys are encrypted on disk under your password. On a CEX
  the exchange holds your coins — "not your keys, not your coins." Exchanges get
  hacked, freeze withdrawals, and go insolvent (Mt. Gox, FTX). Here the funds are
  in *your* wallet the whole time.
- **No deposit, no withdrawal queue.** The swap executes against the DEX router
  from your address in one signed transaction. Nothing to deposit first, nothing
  to wait to withdraw.
- **No account, no KYC, no gatekeeper.** No sign-up, no identity upload, no
  region lock, no account suspension. Just a wallet and an RPC.
- **The key never leaves your machine.** RPC reads, quotes, and signing all happen
  locally. Your password and private key are never transmitted to a server.
- **On-chain transparency.** Every trade is a public transaction you can verify on
  a block explorer — not an internal ledger entry you have to trust.

Trade-off: you pay network gas and you are responsible for your own backup. Lose
the password and the encrypted key with no seed backup, and it's gone — same rule
as any self-custody wallet.

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

## License

[MIT](LICENSE)
