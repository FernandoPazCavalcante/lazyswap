// Package swap orchestrates DEX quoting, approvals, and execution — both
// same-chain EVM swaps through a Uniswap V2 router and cross-chain EVM → BTC
// swaps routed via THORChain.
//
// lazyswap takes no fee. The amount the user asks to swap is the amount that
// reaches the router; see docs/FEES.md.
package swap
