package swap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/common"
)

var ErrBTCDestinationRequired = errors.New("BTC address required")

type providerAmounts struct {
	expected, minimum, fee, gas string
	seconds                     int
}

type fetchBTCQuote func(context.Context, chain.Config, BTCQuoteRequest, string) (providerAmounts, error)

func providerQuote(id BTCProviderID, fetch fetchBTCQuote) func(*Flow, context.Context, BTCQuoteRequest) BTCRouteQuote {
	return func(f *Flow, ctx context.Context, req BTCQuoteRequest) BTCRouteQuote {
		route, err := fetchProviderQuote(ctx, f, id, req, fetch)
		route.Err = err
		return route
	}
}

func fetchProviderQuote(ctx context.Context, f *Flow, id BTCProviderID, req BTCQuoteRequest, fetch fetchBTCQuote) (BTCRouteQuote, error) {
	var route BTCRouteQuote
	if err := validateBTCProviderRequest(f, req, id); err != nil {
		return route, err
	}
	usd, err := strconv.ParseFloat(req.USD, 64)
	if err != nil || !(usd > 0 && usd <= math.MaxFloat64) {
		return route, errors.New("invalid USD amount")
	}
	conv, err := f.ConvertUsdToTokenAmount(ctx, req.From, usd)
	if err != nil {
		return route, err
	}
	amount, err := balance.ParseUnits(conv.TokenAmount, req.From.Decimals)
	if err != nil || amount.Sign() <= 0 {
		return route, errors.New("input amount below token precision")
	}
	a, err := fetch(ctx, f.chain, req, amount.String())
	if err != nil {
		return route, err
	}
	sats, err := strconv.ParseInt(a.expected, 10, 64)
	minimum, minErr := strconv.ParseInt(a.minimum, 10, 64)
	if err != nil || minErr != nil || minimum <= 0 || sats < minimum || sats > 21_000_000*100_000_000 {
		return route, errors.New("invalid provider BTC amounts")
	}
	route = BTCRouteQuote{
		Provider: id, QuoteOnly: true, ETASeconds: a.seconds, FeeBTC: a.fee,
		Quote: FlowQuote{
			FromToken: req.From, ToToken: btcToken, USDAmount: req.USD,
			USDAmountFormatted: FormatUSD(usd), FromTokenAmount: conv.TokenAmount,
			EstimatedOutputSats: sats, EstimatedOutput: balance.FormatUnits(big.NewInt(sats), 8),
			MinOutput: balance.FormatUnits(big.NewInt(minimum), 8), BTCAddress: req.Destination,
			SourceGasBudget: a.gas, ExpiresAt: time.Now().Add(30 * time.Second),
		},
	}
	return route, nil
}

func validateBTCProviderRequest(f *Flow, req BTCQuoteRequest, id BTCProviderID) error {
	if f.chainKey != "ethereum" && f.chainKey != "bsc" {
		return errors.New("provider requires Ethereum or BNB Chain mainnet")
	}
	if req.Destination == "" && id == NEARIntents {
		return ErrBTCDestinationRequired
	}
	if req.Destination != "" {
		address, err := btcutil.DecodeAddress(req.Destination, &chaincfg.MainNetParams)
		if err != nil || !address.IsForNet(&chaincfg.MainNetParams) {
			return errors.New("invalid Bitcoin mainnet address")
		}
	}
	if !common.IsHexAddress(req.Sender) {
		return errors.New("invalid source wallet address")
	}
	return nil
}

func providerJSON(ctx context.Context, endpoint string, body any, header, key string, result any) error {
	var payload []byte
	var err error
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set(header, key)
	}
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(result)
}
