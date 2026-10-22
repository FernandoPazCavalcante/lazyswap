# Security

lazyswap holds private keys and moves money. It has **not** been audited. This
document lists what I know is weak, in my own words, with where it lives in the
code — because the alternative is you finding it yourself and reasonably
concluding nobody looked.

## Reporting a vulnerability

Use **[GitHub private vulnerability reporting](https://github.com/FernandoPazCavalcante/lazyswap/security/advisories/new)**
(Security → Report a vulnerability). Please do not open a public issue for
anything that could lose someone funds.

- **Initial response: within 7 days.**
- Coordinated disclosure: 90 days, or sooner once a fix ships.
- **There is no cash bounty.** I would rather say so than imply one.

**In scope:** the Go client in this repository — key handling, the swap and
THORChain execution paths, the MCP server surface.

**Out of scope:** the price-monitor backend (separate service, not in this
repo), third-party RPC providers, the GoPlus risk API, `gateway.liquify.com`,
and THORChain itself. Weaknesses in how lazyswap *trusts* those are in scope;
weaknesses inside them are not mine to fix.

## Known weaknesses

Nothing here is a secret from an attacker who reads the code. Status is honest:
"open" means it is not fixed today.

### Key material at rest

| Issue | Where | Status |
|---|---|---|
| PBKDF2-HMAC-SHA256 at **100,000 iterations** — roughly 6x below current OWASP guidance, and not memory-hard. The parameter was inherited from the author's older TypeScript app for wire compatibility, which is a bad reason. | `internal/crypto/crypto.go:28` | **Open.** Argon2id behind a versioned envelope is planned. |
| Non-standard **16-byte GCM nonce** via `NewGCMWithNonceSize`, same JS-parity reason. The construction is sound (random IV per encryption, tag verified) but non-idiomatic. | `internal/crypto/crypto.go:25,73` | **Open.** v2 will use the standard 12-byte nonce; v1 stays read-only. |
| A **known-plaintext sentinel** (`"lazyswap-v1-ok"`) is stored encrypted to verify the password, which gives instant offline verification of a guess. Being precise: every encrypted keystore is an offline oracle — an AEAD tag serves the same role — so removing the sentinel is cleanup, **not** a fix. What changes an attacker's economics is KDF cost and password entropy. | `internal/crypto/crypto.go:29` | **Open**, and deliberately not advertised as a hardening item. |
| Password policy is **8 characters**, with no attempt throttling anywhere. | `internal/tui/screens/login/validate.go:14` | **Open.** 12-character minimum and backoff planned. |
| The data directory is `0700` but **`wallets.db` itself is never `chmod`ed**. | `internal/paths/paths.go:35` | **Open.** |

Combined, those mean: **a stolen `wallets.db` plus a weak password is
recoverable offline.** Use a long, random password.

### Key material in memory

Private keys and mnemonics are handled as **immutable Go strings** and passed by
value through several packages. Go strings cannot be zeroed, so they stay on the
heap until the garbage collector reclaims them.

lazyswap **does not defend against an attacker who can read your process
memory, your swap file, or a core dump.** If your machine is compromised, your
keys are compromised.

Related and worse: `wallet.Service.FetchAll` decrypts *every* wallet's key and
mnemonic to render a list. Narrowing that to a single-envelope lookup is the
highest-value fix on this page and is **open**.

### Backup and recovery

**A wallet created in-app cannot currently be exported.** There is an import
path but no reveal and no keystore export, so a generated wallet has no backup
and no recovery. Until that ships, **import a wallet you already back up
elsewhere** instead of generating one here. This is the next item being worked
on.

There is also no password-change path, so a password you suspect is compromised
has no remedy today.

### The password in your environment

`LAZYSWAP_PASSWORD` is the documented non-interactive path. An environment
variable is readable via `/proc/<pid>/environ` by anything running as you, and
lands in your shell history if you type it on a command line. Prefer:

```bash
eval "$(lazyswap set password)"
```

which refuses to print when stdout is a TTY, specifically to avoid leaking it to
your screen (`internal/cli/setpassword.go`).

### Cross-chain BTC — the highest-consequence path

An EVM → BTC swap is a plain value transfer to a vault address with a memo.
There is no revert: a wrong address is an unrecoverable send.

- The inbound vault address comes from **a single third-party gateway**
  (`gateway.liquify.com`), and is validated only as non-empty
  (`internal/thorchain/config.go:15`). A compromised or MITM'd response can
  redirect your funds. **Open** — second-source agreement and address
  validation are planned.
- Slippage tolerance on that leg is **hardcoded at 3%** and not user-settable
  (`internal/thorchain/config.go:22`). **Open.**
- It is the least-covered package in the repo. See issue #14.

### Privacy

Not a vulnerability, but undisclosed is as bad as untrue:

- The default RPC for every chain is a **public endpoint** (`publicnode.com`),
  which therefore sees your address and every transaction you broadcast. There
  is no MEV protection on that path.
- Block-explorer and **GoPlus risk lookups reveal the token you are about to
  buy, before you buy it** — a front-runnable signal.
- **There is no telemetry.** lazyswap sends no analytics, ever. The only
  outbound calls are the ones above plus the opt-in price monitor, which needs
  an API key you provide.

### Supply chain

- Releases are **not yet signed** and there is no build provenance. Verify the
  per-asset `.sha256` files for now. Signed, reproducible releases with
  attestations are planned.
- The `master` branch has no required review today, and the release workflow
  publishes on push. Treat `@master` as development code.
- The `curl | bash` installer at `get.lazyswap.app` is **not** documented in
  this README on purpose: it does no signature verification. Prefer
  `go install` or building from source.

## What was removed for safety

`v0.1.0` removes the backend-routed swap path that previously existed. It signed
transaction data (`to`, `value`, `data`) supplied by a server and granted an
ERC-20 allowance to a server-chosen spender, with no client-side validation —
and the hostname it trusted did not resolve. "The transaction is signed locally"
was true and irrelevant: signing a hostile payload locally loses the money just
as completely. Both that path and the raw-transaction executor behind it are
gone.

The LazySwap Pass NFT and the referral program were removed in the same release.
Their backend was never deployed, so they could not work, and shipping
advertised-but-dead paid features is its own kind of dishonesty.
