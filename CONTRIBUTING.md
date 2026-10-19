# Contributing to lazyswap

Thanks for looking. This is a small project with one maintainer, so here is
what you can expect before you spend any time:

- **I review PRs on weekends.** Expect a decision within a week. If something
  is urgent, say so in the PR and I will try, but I would rather publish the
  cadence I can actually hold than an SLA I break.
- **I will not force-push your branch**, and I will not close your PR and
  reimplement it myself. If I disagree with an approach, I will say so in
  review and we will work it out.
- **Your commits keep your authorship.** If I need to change something to land
  it, I push a commit on top or ask you to — I do not rewrite your history.

## AI

I build lazyswap with Claude Code. That is not hidden and it is not an
apology: the commit history shows it, and `CLAUDE.md` in this repo is the
instruction file I use. Review comments you get from me are **mine** — I read
the code before I comment on it.

AI-assisted pull requests are welcome, with one condition: **you ran it and you
understand it.** Concretely, that means you can answer "why does this line
exist?" for every line you are asking me to merge, and you ran `make gate`
locally before opening the PR.

What I will close without a detailed review: a large PR whose description is
clearly generated, touching code the description does not accurately describe.
That is not an anti-AI rule — it is the same standard as a human PR that nobody
read before sending.

If you used an assistant, a one-line note in the PR is appreciated but not
required.

## What stays open source

lazyswap is open-core, so you deserve to know the line before you contribute.
As of 2026-10-19:

**Everything in this repository stays MIT, permanently.** That includes the
TUI, the CLI, the MCP server, the DEX adapters, the chain configuration, and
all wallet, keystore and crypto code. If that ever changes, it will not be
applied retroactively to contributed code — the MIT grant on what is already
here is irrevocable, and I am not asking anyone to sign that away.

What may become a paid service is a **hosted swap-aggregation backend**, which
lives outside this repository and is not currently deployed. The client will
always work fully without it.

That is why there is **no CLA**. Contributions are covered by the
[Developer Certificate of Origin](https://developercertificate.org/): sign off
your commits with `git commit -s`. It asserts you wrote the code and may
contribute it — nothing more, and it transfers nothing to me.

## Working on a wallet without risking your own money

This is the part that usually stops people, so it is worth being explicit.

**`go test ./...` touches no real chain and no real wallet.** The suite runs
against `internal/testrpc`, a fake EVM JSON-RPC server, and a fake THORNode
endpoint. Set `LAZYSWAP_TEST=1` to route the app log to `/dev/null`.

**No workflow that runs on a pull request has access to any wallet.** The
funded end-to-end swap lives in `nightly.yml`, which triggers only on a
schedule and on manual dispatch — never on `pull_request`. The PR pipeline's
e2e job deliberately leaves `LAZYSWAP_E2E_MNEMONIC` unset, and
`e2e/cli_swap_funded_test.go` skips itself when it is missing. **Opening a PR
cannot spend my money, or yours.**

Useful environment variables when developing:

| Variable | Effect |
|---|---|
| `LAZYSWAP_TEST=1` | sends the app log to `/dev/null` |
| `LAZYSWAP_DATA_DIR` | moves `wallets.db` and the log out of `~/.lazyswap/` |
| `LAZYSWAP_RPC_URL` | overrides every chain's RPC — point it at `internal/testrpc` |
| `LAZYSWAP_THORNODE_URL` | overrides the THORNode API |
| `LAZYSWAP_PASSWORD` | skips the interactive password prompt |

If you want to try the app for real, use `bsc_testnet` and a wallet you created
for the purpose. `--quote-only` costs nothing and needs no funds at all.

## Setting up

```bash
git clone https://github.com/FernandoPazCavalcante/lazyswap.git
cd lazyswap
make setup     # installs the pinned golangci-lint and gremlins into $GOBIN
make gate      # lint + coverage — the same gate CI runs on your PR
```

`make setup` exists because the tool versions are pinned: a local `make gate`
and CI must agree, or the gate is meaningless. Needs Go 1.26+ (see `go.mod`).

## The gate

`make gate` is the whole PR gate — `golangci-lint` plus a coverage floor. CI
also runs the test suite and an e2e job.

- **Coverage must stay at or above 70%**, excluding `internal/tui` and
  `internal/testrpc` (`scripts/coverage-gate.sh`). The margin is currently
  thin, so new code generally needs its own tests to keep the gate green.
- **Lint has no `//nolint` anywhere in the tree.** Please keep it that way; if
  a linter is wrong about your code, say so in the PR and we will adjust the
  config instead.
- Limits worth knowing before you write: cyclomatic complexity 15, functions
  80 lines, files 500 lines.
- Mutation testing (gremlins) runs report-only on PRs and blocks nightly at 50%
  efficacy. A drop is not your problem to fix in a PR.

Testing conventions:

- Test behaviour and data. **Do not test `View()`**, layout or ASCII art — it
  churns and the tests rot.
- CLI output is covered by golden files in `internal/cli/testdata/`. Regenerate
  with `go test ./internal/cli/ -update` and **read the diff like code** — a
  golden diff is a user-visible change.

## Where PRs are welcome — and where they're not

Very welcome, and the fastest to merge:

- **A new DEX adapter.** `internal/dex/uniswap_v2.go` is 76 lines. That is the
  whole seam.
- **A new chain.** `internal/chain/config.go` is one entry per chain. Read
  `ARCHITECTURE.md` first — `StablecoinAddr` drives all USD pricing and is the
  trap.
- **CLI surface**: a `--json` output mode, shell completions, better errors.
- **Tests**, especially for `internal/thorchain`, the highest-consequence and
  least-covered path in the repo.
- **Docs and correctness fixes** of any size. If the README says something the
  code does not do, that is a bug and I want to know.

Please open an issue before starting on:

- **Key handling** — `internal/crypto` and the decrypt paths in
  `internal/wallet`. Not because outside eyes are unwelcome; the opposite, I
  want them. But this code decides whether people lose money, so I own the
  changes and will not merge a drive-by refactor of it. **Reviews, reports and
  written critiques of this code are the most valuable contribution you can
  make** — see `SECURITY.md`.
- **Anything that moves funds in a new way**, including new THORChain paths and
  anything that signs a transaction the client did not construct itself.
- **Large refactors.** Not forbidden, just expensive to review from a standing
  start. Tell me the plan first and you will not waste a weekend.

## Commits

[Conventional Commits](https://www.conventionalcommits.org/) are required —
semantic-release derives versions from the commit history, so the prefix
decides the next version number.

```
fix(swap): quote the full input amount
feat(cli): add --json output
docs: document LAZYSWAP_RPC_URL
```

Sign off with `git commit -s` (DCO, above).

## Good first issues

Issues labelled [`good first issue`](https://github.com/FernandoPazCavalcante/lazyswap/labels/good%20first%20issue)
are scoped so you can finish them in a sitting, and each one names the file to
start in. Issues labelled `help wanted` are bigger but still self-contained.

If something is unclear, ask in the issue — a question is not a bother, it is
usually a sign the docs are missing something.
