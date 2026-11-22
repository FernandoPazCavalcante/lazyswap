package swap

import "context"

type BTCProviderID string

const THORChain BTCProviderID = "THORChain"

type BTCRouteQuote struct {
	Provider   BTCProviderID
	Quote      FlowQuote
	FeeBTC     string
	ETASeconds int
	Err        error
}

type BTCProvider struct {
	ID      BTCProviderID
	Enabled bool
	Quote   func(flow *Flow, ctx context.Context, from TokenInfo, usd, btcAddress string) BTCRouteQuote
	Execute func(flow *Flow, ctx context.Context, privateKey string, quote FlowQuote) FlowResult
}

// ActiveBTCProviders is the catalog used for both discovery and dispatch.
func ActiveBTCProviders() []BTCProvider {
	catalog := []BTCProvider{
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

func quoteTHORChain(f *Flow, ctx context.Context, from TokenInfo, usd, address string) BTCRouteQuote {
	q, err := f.GetThorchainQuote(ctx, from, usd, address)
	return BTCRouteQuote{Provider: THORChain, Quote: q, FeeBTC: q.ThorFees, ETASeconds: q.ThorEstimatedSeconds, Err: err}
}
