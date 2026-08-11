# REVIEW.md — lazyswap (Go)

How to review changes to this repo — especially AI-generated code — and the
quality gates every PR must pass.

## The gates

| Gate | Threshold | Enforced by | Run locally |
|---|---|---|---|
| Lint (incl. complexity/size) | cyclomatic ≤ 15/func · funcs ≤ 80 lines · files ≤ 500 lines | CI `ci.yml` → lint (golangci-lint, `.golangci.yml`) | `make lint` |
| Tests + coverage | all green (`-race`) · total ≥ 70% excluding `internal/tui` | CI → test (`scripts/coverage-gate.sh`) | `make cover` |
| E2E | quote-only CLI e2e + TUI smoke green | CI → e2e | `make e2e` |
| Mutation | efficacy ≥ 50% | `nightly.yml` (blocking); PRs get a report-only job | `make mutate` |
| Funded swap e2e | real tiny swap on bsc_testnet | nightly, only when `LAZYSWAP_E2E_MNEMONIC` secret exists | env var + `make e2e` |
| Everything (PR mirror) | — | — | `make gate` |

Notes:
- `internal/tui` is excluded from the coverage denominator (View()/layout is
  never tested, per CLAUDE.md) — its Update logic still runs in `go test ./...`
  and still counts toward mutation.
- Mutation tool: gremlins v0.5.0 (verified against Go 1.26; dry-run showed
  ~485 runnable mutants). If a Go upgrade breaks it, fallbacks in order:
  build gremlins from master → avito-tech/go-mutesting.
- `release.yml` (semantic-release on master push) runs independently of CI —
  a red `ci.yml` does NOT block a release by itself. The compensating control
  is a GitHub branch-protection rule on master requiring the `ci.yml` checks;
  set it once the current branch stack merges.

## Reviewing a PR (checklist)

- [ ] **Which test fails on the parent commit?** Every behavior change and
      every bugfix must come with a test that fails without it. Ask for the
      answer explicitly; "coverage went up" is not it.
- [ ] **Tests assert behavior/data, not implementation.** Never View() output,
      never layout, never exact ANSI (CLAUDE.md rule). A test that survives a
      mental mutation of the code under test is not a test.
- [ ] **Money paths get explicit tests** — quote math, slippage application,
      fee arithmetic, allowance logic, chain-id guards. Error paths too.
- [ ] **No gate-gaming.** `//nolint` without an inline justification is a
      review blocker (the burn-down used zero). So are coverage-farming tests
      (calls with no meaningful asserts) and weakened gate scripts.
- [ ] **Secrets discipline.** Private keys only inside `internal/wallet` +
      `internal/crypto`; `LAZYSWAP_PASSWORD` from env only; nothing secret in
      diffs, goldens, or fixtures.
- [ ] **Stdout discipline.** Outside `internal/cli`, no stdout/stderr writes
      (corrupts the TUI and the MCP JSON-RPC stream) — applog only.
- [ ] **Diff size sanity.** AI-generated PRs that rewrite more than needed are
      a smell; ask for the minimal diff.
- [ ] **Chain config SSOT** — no hardcoded RPC/router/token addresses outside
      `internal/chain/config.go`.

## Regression policy

- The full suite runs on every PR; a red suite blocks merge.
- Bugfix PRs name the regression test that fails on the parent commit.
- Golden files (`internal/cli/testdata/*.golden`) are updated only via the
  `-update` flag, with the resulting diff reviewed like code.

## Repo specifics

- E2E lives in `e2e/` behind the `e2e` build tag; plain `go test ./...` never
  touches the network beyond unit-level fakes.
- TUI smoke uses teatest and asserts **stable substrings only** — if a smoke
  test breaks on a copy tweak, fix the marker, not the copy.
- E2E env: `LAZYSWAP_E2E_MNEMONIC` (funded bsc_testnet wallet, nightly only);
  `LAZYSWAP_RPC_URL` overrides every chain's RPC (fake RPC in unit tests, a
  paid endpoint in CI if public-RPC flake becomes chronic).
- Coverage-denominator exclusions are a closed list: `internal/tui` (View rule)
  and `internal/testrpc` (test support). Adding to it needs review sign-off.
- Known gap: bsc_testnet's configured router has **no stablecoin liquidity
  path**, so USD-quoting fails there — the quote e2e runs read-only against
  bsc mainnet, and the funded nightly swap will fail until the testnet chain
  config gets a routable stablecoin.
- `make gate` before pushing; CI is the same commands.
