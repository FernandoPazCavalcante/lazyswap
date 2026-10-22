# Product Marketing Context

*Last updated: 2026-06-22*

## Product Overview
**One-liner:** A vim-style terminal wallet that swaps crypto directly on-chain — the lazygit of DeFi.
**What it does:** LazySwap is a keyboard-driven TUI (plus a scriptable CLI) that manages encrypted EVM wallets, shows live balances with USD pricing, and executes swaps straight to a DEX (Uniswap V2 / PancakeSwap) — no exchange account, no browser, no extensions. It also does **cross-chain BTC swaps via THORchain** (native BTC ↔ EVM tokens) from the same terminal. Keys are AES-256-GCM encrypted locally and never leave the machine.
**Product category:** Terminal crypto wallet / DeFi terminal tool (now also cross-chain swap client).
**Product type:** Open-source Go application — interactive TUI + non-interactive CLI (MIT license). *(The original Bun/TS app is now `lazyswap-old/`; the Go rewrite is the primary product.)*
**Business model:** Open-core. The self-custody swap client is free and open-source; revenue comes from a premium layer — the **Lazyswap Pass** NFT and a **pro subscription** that unlock backend-powered features (price/token monitoring, watchlists, alerts, higher rate limits), plus **swap fees** routed to a fee vault. See *Monetization* below.

## Target Audience
**Target users:** Developers, hackers, and terminal-native traders who live in the command line and want the fastest path from intent to executed trade.
**Decision-makers:** Individual users — no enterprise buying process. They discover it on GitHub, Reddit, or Hacker News and install it themselves.
**Primary use case:** Swapping tokens (and BTC) on-chain without leaving the terminal.
**Jobs to be done:**
- Execute buys/sells on DEXes as fast as possible, without browser context-switching
- Swap between BTC and EVM tokens without a CEX or a bridge UI
- Manage multiple encrypted wallets from one keyboard-driven interface
- Check live balances and USD values without opening a web app
**Use cases:**
- Developer swaps tokens between tasks without leaving their terminal workflow
- Trader wants a distraction-free, keyboard-optimized interface for fast execution
- Someone wants native BTC ↔ EVM exposure without depositing to an exchange
- Power user managing multiple wallets wants a single encrypted local tool

## Personas
B2C, single-user — no buying committee. Self-serve install and self-serve upgrade. (Full B2B persona table N/A.)

| Persona | Cares about | Challenge | Value we promise |
|---------|-------------|-----------|------------------|
| Terminal-native trader | Speed, self-custody, no context-switch | Browser DeFi flow is slow and breaks focus | Swap in a few keystrokes, keys stay local |
| Crypto-curious developer | Trust, transparency, scriptability | Doesn't want a custodial CEX or a flaky extension | OSS, on-chain, CLI you can script |

## Problems & Pain Points
**Core problem:** Swapping tokens today means opening a browser, connecting a wallet extension, navigating a DEX frontend, and clicking through confirmations — or depositing to a custodial exchange. For people who live in the terminal, both paths are slow and break flow.
**Why alternatives fall short:**
- MetaMask / Rabby require a browser — you leave your terminal workflow every time
- Web DEX frontends (Uniswap, PancakeSwap) are mouse-driven, slow, and full of distractions
- Centralized exchanges are custodial, require KYC, and gate deposits/withdrawals
- Foundry's `cast` is powerful but scriptable-only — no interactive UI, no wallet management
- Cross-chain BTC normally means a CEX or a clunky bridge UI
- No existing tool combines wallet management + live balances + EVM swaps + cross-chain BTC in one terminal UI
**What it costs them:** Context-switching overhead on every trade. Minutes lost per swap that add up. Custody risk and withdrawal friction on exchanges. Mental friction that discourages quick trades.
**Emotional tension:** Frustration at leaving the terminal for what should be a 3-keystroke operation. Unease about browser extensions or exchanges holding private keys.

## Competitive Landscape
**Direct:** No widely-known TUI crypto wallet exists in this niche — LazySwap is creating the category.
**Secondary:** MetaMask / Rabby (browser extensions) — work but force you into a browser. Foundry `cast` — powerful for scripting, no interactive UI or wallet UX.
**Indirect:** Web DEX frontends (app.uniswap.org, pancakeswap.finance) — full-featured but mouse-driven and distracting. Centralized exchanges (Binance, Coinbase) — custodial, KYC, not self-sovereign. Bridge UIs / wrapped BTC — extra steps and trust assumptions vs. native THORchain swaps.

## Differentiation
**Key differentiators:**
- Vim-style keyboard navigation — zero mouse, zero browser
- Part of the "lazy" terminal-tools ecosystem (lazygit, lazydocker) — instantly familiar UX
- Local-first encrypted storage — keys stay on your machine (AES-256-GCM + PBKDF2)
- **Cross-chain BTC via THORchain** — native BTC ↔ EVM from the terminal, no bridge, no wrapped BTC
- Self-custody, straight-to-DEX execution — no account, no KYC, no deposit/withdrawal queue
- USD-denominated swap input — think in dollars, execute in tokens
- Multi-chain (Ethereum, BSC, BSC Testnet, ETH Sepolia) with single-source-of-truth chain config
- Both an interactive TUI *and* a scriptable CLI for one-shot/automated swaps
**How we do it differently:** One self-contained terminal app replaces the browser wallet + DEX frontend + block explorer + (for BTC) the exchange.
**Why that's better:** Fewer tools, fewer clicks, fewer context switches, no custodian. Trade in seconds, not minutes.
**Why users choose us:** It's the most practical way to swap — including BTC — if you already live in the terminal and want to keep custody.

## Monetization
**Free core:** The self-custody swap client (TUI + CLI, EVM swaps, BTC via THORchain, encrypted wallets) is free and MIT-licensed.
**Premium layer (emerging):**
- **Lazyswap Pass** — an ERC-721 access pass minted on-chain (0.01 native, ~1-year validity). Surfaced in-app via the "Lazyswap Pass" tab (one-click buy). Gates access to the LazySwap backend API.
- **Pro subscription (Stripe)** — unlocks higher tiers of backend-powered features: larger watchlists, price/token monitoring, alerts/notifications, longer history, and higher rate limits. Volume-based tiers also adjust limits by 30-day swap volume.
- **Swap fees** — a protocol fee on swaps routed to a fee vault/treasury.
*Status: early — the Pass mints on BSC Testnet today (mainnet pending); backend tiers exist; pricing not finalized.*

## Objections
| Objection | Response |
|-----------|----------|
| "Is it safe to put private keys in a terminal app?" | Keys are AES-256-GCM encrypted at rest with PBKDF2 (100k iterations) — same standard as password managers. They never leave your machine and are never logged in plaintext. |
| "Why not just use MetaMask?" | If you're already in a browser, MetaMask is fine. LazySwap is for people who spend the day in the terminal and don't want to context-switch for every trade — and it does native BTC, which MetaMask doesn't. |
| "Why pay for a Pass / pro when the app is free?" | The swap client is and stays free. The Pass/pro only unlock optional backend features (monitoring, alerts, watchlists, higher limits) for people who want them. |
| "It only supports Uniswap V2 / PancakeSwap — what about other DEXes?" | V2 routers cover the vast majority of pairs on Ethereum and BSC; THORchain covers cross-chain BTC. More protocols and chains are on the roadmap. |

**Anti-persona:** People who prefer GUI apps and rarely use a terminal. Institutional traders needing order books, limit orders, and advanced charting. Users who want a custodial experience with support.

## Switching Dynamics
**Push:** Frustration with browser-based DeFi flow — too many clicks, slow, constant terminal↔browser switching. Custody anxiety and withdrawal friction on exchanges.
**Pull:** "I can swap tokens — and BTC — without leaving my terminal." Vim keybindings feel like home. Encrypted local storage feels safer than extensions. Instant familiarity for lazygit/lazydocker users.
**Habit:** MetaMask already installed and connected. Muscle memory for web DEX UIs. CEX account already funded. "I've always done it this way."
**Anxiety:** "Can I trust a new tool with my keys?" "What if a bug loses funds?" "It's early-stage." (Mitigated by: open-source, local-only, encryption at rest, on-chain transparency.)

## Customer Language
**How they describe the problem:**
- "I hate switching to the browser just to do a quick swap"
- "MetaMask is fine but I wish I could do this from my terminal"
- "I just want to buy some tokens without all the clicking"
- "Moving BTC into an EVM token means going through an exchange — annoying"
**How they describe us:**
- "It's like lazygit but for crypto"
- "The most practical way to swap tokens"
- "A terminal wallet that actually lets you trade — even BTC"
- "Finally, DeFi without a browser"
**Words to use:** terminal, swap, fast, keyboard-driven, local, encrypted, vim-style, on-chain, self-custody, cross-chain, practical, no-nonsense
**Words to avoid:** easy (implies dumbed-down), simple (undercuts power), revolutionary (overpromises), Web3 (overused/cringe in dev circles), dApp (we're a terminal tool, not a dApp)
**Glossary:**
| Term | Meaning |
|------|---------|
| TUI | Terminal User Interface — a rich interactive UI that runs in your terminal |
| EVM | Ethereum Virtual Machine — runtime shared by Ethereum, BSC, and other chains |
| DEX | Decentralized Exchange — on-chain swap protocol (Uniswap, PancakeSwap) |
| THORchain | Cross-chain liquidity protocol enabling native BTC ↔ EVM swaps |
| Lazyswap Pass | On-chain ERC-721 access pass that unlocks premium backend features |

## Brand Voice
**Tone:** Technical but human. Confident, not boastful. Hacker-friendly.
**Style:** Direct and concise — say it in fewer words. Show, don't sell. Let the tool speak for itself.
**Personality:** Fast, minimal, trustworthy, sharp, unapologetic about being a terminal tool.
**Brand lineage:** lazygit, lazydocker, btop, neovim — terminal power tools for people who chose the command line on purpose.

## Proof Points
**Metrics:** (early-stage — to be populated as adoption grows)
**Customers:** Built by the author for daily personal use — dogfooded from day one.
**Testimonials:**
> "I built LazySwap because it was the most practical way to exchange tokens." — Fernando, creator
**Value themes:**
| Theme | Proof |
|-------|-------|
| Speed | Swap in a few keystrokes — no browser, no mouse |
| Self-custody | Straight-to-DEX execution; keys never leave your machine |
| Cross-chain | Native BTC ↔ EVM via THORchain, from one terminal |
| Security | AES-256-GCM + PBKDF2; keys never logged in plaintext |
| Transparency | MIT-licensed, open-source, on-chain trades you can verify |

## Goals
**Business goal:** Grow adoption among terminal-native crypto users; convert a slice into Pass/pro revenue. Become the default "lazy" tool for DeFi.
**Conversion action:** Top-funnel — `go install github.com/FernandoPazCavalcante/lazyswap@latest` and start swapping. Revenue — buy a Lazyswap Pass / upgrade to pro.
**Current metrics:** Pre-launch. No public usage data yet.
