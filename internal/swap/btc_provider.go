package swap

import "context"

type BTCProviderID string

const (
	THORChain   BTCProviderID = "THORChain"
	LIFI        BTCProviderID = "LI.FI"
	NEARIntents BTCProviderID = "NEAR Intents"
)

type BTCQuoteRequest struct {
	From                     TokenInfo
	USD, Destination, Sender string
	Slippage                 float64
}

type BTCRouteQuote struct {
	Provider   BTCProviderID
	Quote      FlowQuote
	FeeBTC     string
	ETASeconds int
	QuoteOnly  bool
	Err        error
}

type BTCProvider struct {
	ID      BTCProviderID
	Enabled bool
	Quote   func(flow *Flow, ctx context.Context, req BTCQuoteRequest) BTCRouteQuote
	Execute func(flow *Flow, ctx context.Context, privateKey string, quote FlowQuote) FlowResult
}

// ActiveBTCProviders is the catalog used for both discovery and dispatch.
func ActiveBTCProviders() []BTCProvider {
	catalog := []BTCProvider{
		{ID: LIFI, Enabled: true, Quote: providerQuote(LIFI, fetchLIFI)},
		{ID: NEARIntents, Enabled: true, Quote: providerQuote(NEARIntents, fetchNear)},
		{ID: THORChain, Enabled: false, Quote: quoteTHORChain, Execute: (*Flow).ExecuteThorchain},
	}
	var active []BTCProvider
	for _, p := range catalog {
		if p.Enabled {
			active = append(active, p)
		}
	}
	return active
}

func quoteTHORChain(f *Flow, ctx context.Context, req BTCQuoteRequest) BTCRouteQuote {
	q, err := f.GetThorchainQuote(ctx, req.From, req.USD, req.Destination)
	return BTCRouteQuote{Provider: THORChain, Quote: q, FeeBTC: q.ThorFees, ETASeconds: q.ThorEstimatedSeconds, Err: err}
}
