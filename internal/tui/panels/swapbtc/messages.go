package swapbtc

import (
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

// ─── Messages ──────────────────────────────────────────────────────────────────

// QuoteRequestMsg asks the parent for a THORchain quote (panel → parent).
type QuoteRequestMsg struct {
	From       swap.TokenInfo
	USDAmount  string
	BTCAddress string
}

// ExecuteRequestMsg asks the parent to broadcast the swap (panel → parent).
type ExecuteRequestMsg struct {
	From       swap.TokenInfo
	USDAmount  string
	BTCAddress string
}

// EstimateRequestMsg asks the parent for a destination-less price preview
// (panel → parent), fired when the user confirms a USD amount.
type EstimateRequestMsg struct {
	From      swap.TokenInfo
	USDAmount string
}

// EstimateResultMsg carries a price-preview estimate back (parent → panel).
type EstimateResultMsg struct {
	Quote swap.FlowQuote
	Err   error
}

// QuoteResultMsg carries a quote back to the panel (parent → panel).
type QuoteResultMsg struct {
	Quote swap.FlowQuote
	Err   error
}

// ExecutionResultMsg carries an execution result back (parent → panel).
type ExecutionResultMsg struct {
	Result swap.FlowResult
}

// TickMsg drives the preview re-quote countdown.
type TickMsg struct{}
