# Fees

**lazyswap takes no fee.** The amount you ask to swap is the amount that
reaches the router. There is no fee constant in the code, no fee recipient
address, and no fee line in any quote.

This document is the single source of truth. If anything in the README, the
`--help` output, the TUI or a blog post disagrees with this file, this file is
right and the other place is a bug — please open an issue.

## What you do pay

| Cost | Who receives it | Typical size |
|---|---|---|
| Network gas | The chain's validators | Varies; shown before you confirm |
| DEX pool fee | The pool's liquidity providers | 0.30% on Uniswap V2, 0.25% on PancakeSwap V2 |
| THORChain network + outbound fee | THORChain | Shown as `Fees: … BTC` in the BTC quote |

None of that money goes to lazyswap or to its author. The DEX pool fee is
charged by the router contract itself and is already reflected in the quoted
output — it is not something lazyswap adds.

The THORChain affiliate fee is set to **0 basis points**
(`AffiliateBPS` in `internal/thorchain/config.go`). THORChain lets an
integrating client take a cut of every swap; lazyswap does not.

## What changed, and when

Up to and including `v0.1.0-beta.7`, the code did something I need to state
plainly: a constant `SwapFeePercent = 0.15` in `internal/swap/fee.go` deducted
0.15% from the input of **every** swap, on every route, including the route the
README called "free". The deducted amount was not collected by anyone — it
simply stayed in your wallet as unswapped tokens — but it meant
`lazyswap swap 50 BNB USDT` did not swap $50 worth, and the quote displayed a
"fee" line for money nothing received. Meanwhile the README advertised 1% on
the API route and the `--help` text said the direct route had no fee. Four
places, four different claims.

That mechanism is now deleted — not set to zero, deleted, along with the
`FeeReceiver` constant that pointed at the burn address. A regression test in
`internal/swap/flow_rpc_test.go` asserts that a 1:1 pool returns the full
input, and a test in `internal/cli/swap_run_test.go` asserts that no quote
surface mentions a fee at all.

## If this ever changes

If lazyswap ever charges a fee, it will arrive as all three of the following,
or it is a bug:

1. A real transfer to a named address published in this file — never a silent
   reduction of the amount you asked to swap.
2. A line item in the quote, in the TUI, the CLI and the MCP `swap_quote`
   response, shown **before** you confirm.
3. An announcement in the release notes of the version that introduces it, in
   its own commit, ahead of the release.

## Removed from this release

The paid API route (OpenOcean aggregation), the LazySwap Pass NFT and the
referral program are **not** part of `v0.1.0`. Their backend was never
deployed, so they could not work; shipping them as advertised-but-dead paid
features would be worse than not shipping them. They are not a hidden cost —
they are absent. See the roadmap in the README for what is planned.
