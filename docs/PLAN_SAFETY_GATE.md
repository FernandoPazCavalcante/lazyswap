# Plan — AI Pre-Swap Safety Gate

*Status: phases 1–4 implemented (branch `feat/safety-gate`) · 2026-08-10 — GoPlus checker + CLI/TUI/MCP wiring; local eth_call sim (phase 5) and AI summary (phase 6) still pending*

## Goal
Before the user signs a swap, show a risk verdict for the token being **bought**
(honeypot? buy/sell tax? mintable / owner-takeback / LP-lock?) inline in the
confirm step — in both the CLI and the TUI. Advisory by default; block on high
risk (configurable). The deterministic gate stands on its own; the **"AI" layer
is an optional LLM** that turns the raw flags into a one-line plain-English verdict.

## Risk sources (both verified live during research)
1. **External (mainnets):**
   - GoPlus Token Security — `GET https://api.gopluslabs.io/api/v1/token_security/{chainId}?contract_addresses={addr}`. Free, no key (~30 req/min), widest flag set.
   - Honeypot.is — `GET https://api.honeypot.is/v2/IsHoneypot?address={addr}&chainID={id}` — real buy/sell **simulation** confirm on ETH(1)/BSC(56)/Base(8453).
   - **Gotchas:** GoPlus flags are STRING `"0"/"1"` (parse explicitly); result keyed by **lowercased** address; **a missing field means "unknown", not "safe"**; **no testnet support** (no Sepolia 11155111 / bsc_testnet 97).
2. **Local (any chain incl. testnet, privacy-preserving):** eth_call honeypot
   simulation via go-ethereum **`gethclient`** state overrides — simulate buy then
   sell against the router on the user's own RPC; revert-on-sell = honeypot,
   expected-vs-actual output = buy/sell tax. The token address never leaves the
   machine. `gethclient` ships with the go-ethereum **v1.17.3 already in `go.mod`**
   (additive import, **no new dependency**). Needs storage-slot discovery
   (slotseek-style) to fake the post-buy balance + router allowance for the sell leg.

> Note: GoPlus/Honeypot cover the default `bsc` + `ethereum` but **not** the testnet
> chains where the Pass lives — the local sim is the only option there.

## Recommended phasing
**GoPlus first** (one HTTP call, instant value on mainnets) → **local sim**
(testnet / privacy / fallback) → **optional AI summary**.

## Architecture
New `internal/safety` package:
```go
type Level int // Low, Medium, High
type Report struct {
    Score      int
    Level      Level
    Honeypot   bool
    BuyTaxBps  int
    SellTaxBps int
    Flags      []Flag
    Source     string // "goplus" | "honeypot.is" | "local-sim"
    Unknown    bool   // could not assess → caution, never "safe"
}
type Checker interface { Check(ctx context.Context, chainKey, tokenAddr string) (Report, error) }
```
- `goplus.Checker` (HTTP), `local.Checker` (gethclient sim), `multi` (run available
  sources, merge **fail-closed**).
- `FormatReport(Report) string` for terminal rendering (theme dim/error styles).
- Cache by `(chainKey, tokenAddr)` for a few minutes (stay under GoPlus 30/min; cheap re-renders).

## Insertion points
Only assess the **destination** token (skip native / known stablecoins). After the
quote, before signing:
- **CLI** — `internal/cli/swap.go` `runSwap`: after `flow.Quote` / `printQuote`,
  before `confirm()`. Flags `--no-safety`, `--safety-block`.
- **TUI** — `internal/tui/overlays/swapoverlay/swap.go`: on `QuoteResultMsg`, fire
  an **async** `safety.Check` (don't stall the UI), render the verdict in
  `previewBody` (stepPreview) above the confirm line ("checking…" → result). Add a
  `riskReport` field to the overlay Model.

Both paths call the same `safety` package + `FormatReport`.

## Local sim recipe (phase 2)
Open the configured RPC with `rpc.DialContext(cfg.RPCURL)`, wrap with
`gethclient.New` (the standard `ethclient` **cannot** carry overrides). Then:
1. `getAmountsOut(ethIn, [WETH, token])` — proves liquidity (a revert here = "unknown / no liquidity", **never** "honeypot") and gives expected buy output.
2. Simulate `swapExactETHForTokens` with a native-`Balance` override on a scratch caller → actual tokens → `buyTax`.
3. Discover the token's `balanceOf` slot (keccak `pad(holder)||pad(slot)` brute-force + `eth_getStorageAt`); `StateDiff`-override `balanceOf[scratch]=bought` and `allowance[scratch][router]=MaxUint256` — this satisfies "router approval for the sell" **without an approve tx**.
4. Simulate `swapExactTokensForETH` — revert = honeypot/non-sellable; success → `sellTax`.
5. On a plain-sell revert, retry `…SupportingFeeOnTransferTokens` to distinguish heavily-taxed-but-sellable from a true honeypot.
Capability-probe the RPC once (a trivial Balance-override eth_call); if the gateway
strips the override param, degrade to "state-override not supported by this RPC".

## AI layer (optional, premium)
Feed the raw flags to an LLM (backend or BYO-key) → one-liner, e.g. *"Honeypot:
sell blocked + 35% sell tax, owner can mint — do not buy."* The deterministic gate
is the free core; the AI summary + deeper checks (creator reputation, GoPlus
approval-security on the router/spender) are **Pass/Pro-gated**, matching the
monetization model.

## Files
- `internal/safety/report.go` *(new)* — types, scoring/policy, `FormatReport`.
- `internal/safety/goplus.go` *(new)* — HTTP client; parse string flags; missing→unknown.
- `internal/safety/local.go` *(new, phase 2)* — gethclient sim (above).
- `internal/safety/checker.go` *(new)* — source selection (mainnet→GoPlus; else/also→local) + merge.
- `internal/chain/abi.go` — add router `…SupportingFeeOnTransferTokens` entries + minimal ERC20 `balanceOf`/`allowance` ABI for slot work.
- `internal/cli/swap.go` — wire check + flags.
- `internal/tui/overlays/swapoverlay/swap.go` — async check + render; add `riskReport` to Model.
- `internal/settings` *(optional)* — persisted `safety_enabled` / `safety_block_high`.

## Build sequence
1. `safety` package + GoPlus checker + `Report`/`FormatReport` + parse tests.
2. Wire CLI (advisory print) → then TUI preview.
3. Block-on-high policy + flags/settings.
4. Local eth_call sim (testnet + privacy + fallback); probe RPC first, degrade gracefully.
5. *(optional/premium)* AI one-line summary + approval-security + creator reputation; gate behind Pass/Pro.

## Tests
- GoPlus parse fixtures: string `"0"/"1"` flags, lowercased key, **missing field → `Unknown`**.
- Policy: score/level thresholds; block decision.
- Local sim: unit-test the **slot-key computation** (`keccak(pad(holder)||pad(slot))`) and the tax math (expected vs actual → bps); gate the live sim behind a build tag / manual.
- CLI: `--no-safety` skips; high-risk + block refuses without override.

## Correctness / security
- **Fail-closed:** API timeout/outage/unknown → explicit warning, never silent allow. External call is in the swap hot path → short timeout + cache + graceful degrade.
- Local sim is **read-only** (eth_call / eth_getStorageAt) — gas-free, no broadcast, no approve tx (allowance via override).
- Don't mislabel **fee-on-transfer** tokens as honeypots (SupportingFeeOnTransfer retry).
- The sim **cannot** catch time/trade-count/threshold-gated honeypots — state this clearly; it is a risk reducer, not a guarantee.
- GoPlus result key is the **lowercased** address — look up case-insensitively.

## Decisions to confirm
- **D1** — v1 source: GoPlus only *(rec; fast, mainnet)* vs. include local sim now (testnet + privacy, more work).
- **D2** — advisory *(rec)* vs. blocking by default.
- **D3** — AI (LLM) summary in v1, or a later premium layer *(rec: deterministic gate first)*.
- **D4** — free core vs. Pass/Pro gating for deeper/AI checks *(rec: free gate, premium AI — matches monetization)*.

## Sources
GoPlus Token Security + Approval API (docs.gopluslabs.io; live-verified no-key tier) ·
Honeypot.is v2 (docs.honeypot.is) · go-ethereum `gethclient` state overrides
(pkg.go.dev/.../ethclient/gethclient; geth eth_call override docs) · slotseek
(github.com/d3or/slotseek) · valamidev honeypot checker.
