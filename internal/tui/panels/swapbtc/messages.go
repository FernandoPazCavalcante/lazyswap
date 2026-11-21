package swapbtc

import (
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

type Provider string

const THORChain Provider = "THORChain"

// QuoteRequestMsg asks the parent for the available BTC routes.
type QuoteRequestMsg struct {
	ID         uint64
	From       swap.TokenInfo
	USDAmount  string
	BTCAddress string
}

// ExecuteRequestMsg asks the parent to broadcast the swap (panel → parent).
type ExecuteRequestMsg struct {
	ID       uint64
	Provider Provider
	Quote    swap.FlowQuote
}

// EstimateRequestMsg asks the parent for a destination-less price preview
// (panel → parent), fired when the user confirms a USD amount.
type EstimateRequestMsg struct {
	ID        uint64
	From      swap.TokenInfo
	USDAmount string
}

// EstimateResultMsg carries a price-preview estimate back (parent → panel).
type EstimateResultMsg struct {
	ID    uint64
	Quote swap.FlowQuote
	Err   error
}

// RouteQuote holds the result for a provider. A zero quote with no error is pending.
type RouteQuote struct {
	Provider   Provider
	Quote      swap.FlowQuote
	FeeBTC     string
	ETASeconds int
	Err        error
}

// QuoteResultMsg carries one provider's response back to the panel.
type QuoteResultMsg struct {
	ID    uint64
	Route RouteQuote
}

// ExecutionResultMsg carries an execution result back (parent → panel).
type ExecutionResultMsg struct {
	ID     uint64
	Result swap.FlowResult
}

// TickMsg updates quote expiry and the visible countdown.
type TickMsg struct {
	ID uint64
	At time.Time
}
