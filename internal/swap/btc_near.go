package swap

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

const nearAPI = "https://1click.chaindefuser.com/v0"

type nearAsset struct {
	AssetID, Blockchain, Symbol, ContractAddress string
	Decimals                                     uint8
}

func nearAssetIDs(tokens []nearAsset, c chain.Config, from TokenInfo) (string, string, error) {
	var origin, destination string
	for _, t := range tokens {
		if t.Blockchain == "btc" && t.Symbol == "BTC" && t.Decimals == 8 && t.ContractAddress == "" {
			destination = t.AssetID
		}
		if t.Blockchain != c.NearIntentsChain || t.Decimals != from.Decimals {
			continue
		}
		native := from.Address == NativeSentinel && t.ContractAddress == "" && t.Symbol == c.NativeSymbol
		if native || strings.EqualFold(t.ContractAddress, from.Address) {
			origin = t.AssetID
		}
	}
	if origin == "" || destination == "" {
		return "", "", errors.New("token pair unavailable on NEAR Intents")
	}
	return origin, destination, nil
}

func fetchNear(ctx context.Context, c chain.Config, r BTCQuoteRequest, amount string) (providerAmounts, error) {
	key := os.Getenv("LAZYSWAP_NEAR_INTENTS_API_KEY")
	var tokens []nearAsset
	if err := providerJSON(ctx, nearAPI+"/tokens", nil, "X-API-Key", key, &tokens); err != nil {
		return providerAmounts{}, err
	}
	origin, dest, err := nearAssetIDs(tokens, c, r.From)
	if err != nil {
		return providerAmounts{}, err
	}
	body := map[string]any{
		"dry": true, "swapType": "EXACT_INPUT",
		"slippageTolerance": int(math.Round(r.Slippage * 100)),
		"originAsset":       origin, "destinationAsset": dest, "amount": amount,
		"depositType": "ORIGIN_CHAIN", "refundType": "ORIGIN_CHAIN", "refundTo": r.Sender,
		"recipientType": "DESTINATION_CHAIN", "recipient": r.Destination,
		"deadline": time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
	}
	var resp struct {
		Quote struct {
			AmountIn, AmountOut, MinAmountOut string
			TimeEstimate                      int
		}
	}
	if err := providerJSON(ctx, nearAPI+"/quote", body, "X-API-Key", key, &resp); err != nil {
		return providerAmounts{}, err
	}
	if resp.Quote.AmountIn != amount {
		return providerAmounts{}, errors.New("provider returned a different input amount")
	}
	return providerAmounts{expected: resp.Quote.AmountOut, minimum: resp.Quote.MinAmountOut, seconds: resp.Quote.TimeEstimate}, nil
}
