# Plan — MCP Server (`lazyswap mcp`)

*Status: implemented (branch `feat/mcp-server`) · 2026-08-10 · SDK v1.7.0, all phases incl. gated trading*

## Goal
Expose lazyswap to AI agents (Claude Code, Cursor, Codex) over the Model Context
Protocol (MCP) so a user can drive their own wallet from an agent. This is the
"AI tool the user can actually use" — the terminal-native, self-custody analog of
Kraken CLI / OKX Agent Trade Kit / Trust Wallet Agent Kit.

## SDK choice
Use the **official** SDK: `github.com/modelcontextprotocol/go-sdk/mcp` (v1.6.1,
stable, maintained with Google; the ecosystem is converging on it — GitHub
migrated its MCP server to it). Community `mark3labs/mcp-go` is popular but
pre-1.0 / solo-maintained; only pick it if SSE/in-process transports are needed.

- Requires **Go 1.25+** — `go.mod` is already `go 1.26.3`, so **no bump needed**.
- Typed In/Out structs → automatic JSON Schema via `json:`/`jsonschema:` tags.
- Transport: **stdio** (`&mcp.StdioTransport{}`), exactly how agents launch local servers.

## Hard rule: stdout is the JSON-RPC channel
In MCP mode nothing may write to stdout except the protocol stream. lazyswap
already routes logs to a file via `internal/applog`, but the CLI's normal stdout
prints (`runSwap`, `printQuote`) would corrupt it. **MCP tools must call the
services directly** (`swap.Flow`, `balance.Service`, `settings`, `wallet`, `pass`)
— never the `runX` CLI funcs that print.

## Architecture
New `internal/mcp` package + an `mcp` case in `cli.Run`. Each tool = a typed
struct + handler that delegates to the same services the CLI/TUI use. Pure
transport + serialization; no business-logic duplication.

Minimal shape (official SDK):
```go
type SwapQuoteIn struct {
    From string `json:"from" jsonschema:"source token symbol"`
    To   string `json:"to"   jsonschema:"destination token symbol"`
    USD  string `json:"usd"  jsonschema:"USD amount to swap"`
}
func (h *Handler) SwapQuote(ctx context.Context, req *mcp.CallToolRequest, in SwapQuoteIn) (*mcp.CallToolResult, FlowQuoteOut, error) {
    // call h.flow.Quote(...) and map to FlowQuoteOut
}
// server
s := mcp.NewServer(&mcp.Implementation{Name: "lazyswap", Version: version}, nil)
mcp.AddTool(s, &mcp.Tool{Name: "swap_quote", Description: "quote a swap"}, h.SwapQuote)
s.Run(ctx, &mcp.StdioTransport{})
```

## Tools (read-first, then guarded writes)
- `list_chains` — supported chains + tokens (from `chain.CHAINS`).
- `get_wallets` — wallet addresses only (**never** private keys).
- `get_balances {wallet?, chain?}` — balances + USD (`balance.Service.FetchAll`).
- `swap_quote {from, to, usd, slippage?, wallet?, chain?}` → `FlowQuote` (read-only, safe).
- `get_settings` / `set_settings` — chain / slippage / default wallet.
- `get_pass_status` / `buy_pass` (optional) — reuse `internal/pass`.
- `swap_execute {...}` — **danger surface; see Security.**

## Security / key handling (the crux)
Signing needs the decrypted key; MCP has no interactive prompt.
- **Read-only by default.** Omit `swap_execute` unless launched with `--allow-trading`.
- When trading is enabled, read the password from the server's **`LAZYSWAP_PASSWORD`
  env** (reuse `wallet.Unlock`). **Never** accept the key or password as a tool
  parameter — it would land in the agent/model context.
- Code-enforced guardrails: `--max-usd` per-swap cap + chain allowlist, so a
  prompt-injected agent can't drain the wallet (directly answers the Grok ~$200K /
  Step Finance ~$40M agent-drain incidents).
- Surface domain/validation failures via `CallToolResult.IsError` (so the agent
  sees them), not just the Go `error` (which is a protocol-level failure).

## Files
- `go.mod` — `go get github.com/modelcontextprotocol/go-sdk`.
- `internal/cli/cli.go` — add `case "mcp": return runMcp(args[1:])` + usage line.
- `internal/mcp/server.go` *(new)* — build server, `AddTool` per tool, serve stdio; hold shared services (open DAO, lazily dial Flow/Balance per chain).
- `internal/mcp/tools.go` *(new)* — In/Out structs (`json`+`jsonschema` tags) + handlers.
- `internal/cli/mcp.go` *(new)* — `runMcp` flag parsing (`--allow-trading`, `--max-usd`, `--chain`).
- `swap.FlowQuote` / `swap.FlowResult` — add `json` tags (currently untagged) for clean structured output.
- `README.md` / `CLAUDE.md` — document `lazyswap mcp` + a sample `.mcp.json` client config.

## Build sequence
1. **Skeleton** — add SDK; `lazyswap mcp` serves an empty server over stdio; verify `claude mcp add lazyswap -- lazyswap mcp` connects and lists tools.
2. **Read-only tools** — `list_chains`, `get_wallets`, `get_balances`, `swap_quote`, `get_settings`. Verify an agent can quote a swap.
3. **Guarded writes** — `set_settings`; `swap_execute` behind `--allow-trading` + `LAZYSWAP_PASSWORD` + `--max-usd` cap.
4. **Polish** — `IsError` mapping, pass tools, docs + sample config.

## Tests (behavior/data; no transport e2e in unit)
- Handler unit tests with a stub Flow: `swap_quote` returns expected Out; `swap_execute` refused when trading disabled; `--max-usd` rejects oversize.
- In-struct unmarshal from sample args; `AddTool` registration compiles.
- Documented manual stdio smoke test.

## Risks
- **stdout contamination** → JSON-RPC corruption. Audit: nothing in the MCP path prints; `applog` (file) only.
- **Key safety** — never accept key/password as a tool arg; env-only; default read-only.
- SDK still evolving HTTP/streamable transports; stdio-only is low risk — pin the version (`go list -m -versions …` before pinning).
- Concurrency — agents fire rapid calls; reuse a lazily-dialed Flow per chain or guard with a mutex.

## Decisions to confirm
- **D1** — expose `swap_execute` in v1, or read-only (quote/balances) first? *(rec: read-only first; gated trading as a follow-up)*
- **D2** — trading auth = env password + caps *(rec)* vs. a confirmation handshake.

## Sources
Official SDK: github.com/modelcontextprotocol/go-sdk · pkg.go.dev/.../mcp · modelcontextprotocol.io/docs/sdk · GitHub MCP server migration changelog.
