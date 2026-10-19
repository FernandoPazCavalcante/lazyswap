# Architecture

A map for finding your way around. If you want to contribute, the two
**seams** below are where to start — skip to them.

## Shape

One static Go binary with three front-ends over one service layer:

```
                     main.go
        os.Args > 1 ? cli.Run() : TUI
                        │
      ┌─────────────────┼─────────────────┐
      │                 │                 │
  internal/tui     internal/cli     internal/mcp
  Bubble Tea       one-shot         stdio server
  TUI              commands         for AI agents
      └─────────────────┼─────────────────┘
                        │  all three call the same services
      ┌─────────────────┼──────────────────────────┐
      │         │       │        │        │        │
    swap     wallet  balance  safety  thorchain  pass
   quote +   CRUD +  fetch +  GoPlus  EVM→BTC    ERC-721
   execute   keys    format   risk    routing
      └─────────────────┼──────────────────────────┘
                        │
      ┌─────────────────┼─────────────────┐
      │                 │                 │
  wallet.DAO        chain.CHAINS      explorer
  SQLite            RPC, routers,     block explorer
  (cgo-free)        token addresses   API
                        │
                 EVM RPC / DEX router
                   (your signed tx)
```

The rule that matters: **a front-end never talks to the chain directly, and a
service never knows which front-end called it.** That is what lets the same
`swap.Flow` serve the TUI, a shell pipeline and an AI agent.

## Packages

| Package | LOC | What it owns |
|---|---:|---|
| `internal/swap` | 1343 | quote + execute orchestration, EVM and BTC |
| `internal/cli` | 840 | non-interactive commands, golden-tested output |
| `internal/mcp` | 702 | MCP stdio server; read-only unless `--allow-trading` |
| `internal/api` | 675 | backend client (currently inert — host not deployed) |
| `internal/thorchain` | 580 | cross-chain EVM→BTC routing, memos, vaults |
| `internal/balance` | 517 | balance fetch + display formatting |
| `internal/wallet` | 515 | wallet CRUD, SQLite DAO, key decryption |
| `internal/safety` | 380 | pre-swap token risk check (GoPlus), fail-closed |
| `internal/chain` | 251 | **CHAINS map** + contract ABIs |
| `internal/testrpc` | 194 | fake EVM JSON-RPC for tests |
| `internal/tui` + subpkgs | ~5000 | screens, panels, overlays, theme, keys |
| `internal/pass` | 162 | LazySwapPass ERC-721 reads (bsc_testnet only) |
| `internal/explorer` | 155 | block explorer API |
| `internal/applog` | 138 | file logger |
| `internal/settings` | 138 | persisted chain / slippage / default wallet |
| `internal/crypto` | 137 | AES-256-GCM + PBKDF2 key encryption |
| `internal/paths` | 65 | filesystem path resolution |
| `internal/dex` | 76 | Uniswap V2 / PancakeSwap quote + swap |

## The two seams

### Add a DEX — `internal/dex`, 76 lines

The entire DEX abstraction is `internal/dex/uniswap_v2.go`: 76 lines wrapping
`getAmountsOut` and the router's swap methods. A new V2-compatible DEX is a new
file next to it and a config entry.

A **Uniswap V3** adapter is a bigger job and labelled `needs design` for a
reason: the current provider is shaped around `getAmountsOut`, a plain `view`
call. V3 needs fee-tier selection, encoded multi-hop paths, and `QuoterV2`,
which is **not** a view function — you have to decode revert data to read the
quote. That means redesigning the provider interface and its callers, not
adding a file. Worth doing; just not a first PR.

### Add a chain — `internal/chain/config.go`

`CHAINS` is the single source of truth for everything chain-specific. One entry
per chain, plus a row in `OrderedKeys` (Go maps iterate randomly, so display
order lives there) and a case in the table test.

**`StablecoinAddr` is the trap.** It drives all USD pricing: `$50 worth of BNB`
is computed by asking the configured router how much of the chain's stablecoin
that much BNB buys. Point it at a token with no liquidity path through the
configured router and every quote on that chain is silently wrong — no error,
just bad numbers. When you add a chain, verify one quote against a known price
before opening the PR.

The existing `bsc_testnet` entry is a live example of the failure: it is
configured correctly but testnet pools have no liquidity, so USD quoting there
does not work. That is a known gap, not something to fix in code.

## Three single sources of truth

Breaking these is the most common review comment, so they are worth learning
before you write:

1. **`internal/chain/config.go`** — never hardcode an RPC URL, router address
   or token address anywhere else.
2. **`internal/paths`** — never build a filesystem path by hand. Everything
   lives under `~/.lazyswap/`, overridable with `LAZYSWAP_DATA_DIR`, and tests
   depend on `paths.Override` to stay out of your real home directory.
3. **`internal/applog`** — **outside `internal/cli`, never write to stdout or
   stderr.** Not `fmt.Print`, not `log`, not `println`. Stdout is load-bearing
   twice over: it is the Bubble Tea render target, and it is the MCP server's
   JSON-RPC channel. A stray print corrupts the TUI and breaks every AI-agent
   session. Use `applog`, which writes to `~/.lazyswap/lazyswap.log` and never
   panics.

## Testing against a fake chain

`internal/testrpc` is a fake EVM JSON-RPC server that understands just enough
of what lazyswap calls: `getAmountsOut`, `balanceOf`, `allowance`,
`eth_getBalance`, and the transaction lifecycle. It is the reason you can work
on this repo without a node, a faucet or any money.

It is the best-kept secret in the codebase, so here is the whole pattern:

```go
rpc := testrpc.New()
t.Cleanup(rpc.Close)
rpc.RateNum = big.NewInt(600)   // 1 in → 600 out; default is 1:1

p := provider(t, rpc)
got, err := p.GetPrice(context.Background(), wbnb, usdt, "2")
// got == "1200"
```

Knobs on `testrpc.Server`, all settable before the call under test:

| Field | Use it to |
|---|---|
| `RateNum` / `RateDen` | set the pool rate: `out = in * num / den` |
| `NoLiquidity` | make every `getAmountsOut` revert |
| `NativeBalance` | set the wallet's native balance |
| `TokenBalance` | set an ERC-20 balance |
| `Allowance` | control whether an approval is needed |
| `RevertTx` | make the swap transaction revert |
| `Calls` | assert which RPC methods were hit, and how often |
| `TxCount` | assert how many transactions were broadcast |

Point the whole app at it with `LAZYSWAP_RPC_URL=$rpc.URL`, which overrides
every chain's RPC at once.

`internal/testrpc` is excluded from the coverage denominator — its coverage is
its use by other packages' tests.

## Release

`master` cuts beta prereleases; `stable` cuts full releases. Versions come from
the commit history via semantic-release, which is why Conventional Commits are
required. The binary's version string is injected with
`-ldflags -X .../internal/cli.version=…`.
