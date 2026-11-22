package swap

import (
	"context"
	"errors"
	"math/big"
	"os"
	"strings"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/ethereum/go-ethereum/common"
)

const lifiBitcoinChainID = 20000000000001

type lifiStep struct {
	Tool          string
	IncludedSteps []lifiStep
	Estimate      struct {
		ExecutionDuration int
		GasCosts          []struct {
			Amount string
			Token  struct {
				ChainID uint64
				Address string
			}
		}
	}
}

func (s lifiStep) disabledTool() bool {
	if strings.Contains(strings.ToLower(s.Tool), "thor") {
		return true
	}
	for _, step := range s.IncludedSteps {
		if step.disabledTool() {
			return true
		}
	}
	return false
}

type lifiRoute struct {
	FromChainID, ToChainID            uint64
	FromAmount, ToAmount, ToAmountMin string
	ToToken                           struct {
		Decimals int
		Symbol   string
	}
	Steps []lifiStep
}

func fetchLIFI(ctx context.Context, c chain.Config, r BTCQuoteRequest, amount string) (providerAmounts, error) {
	from := r.From.Address
	if from == NativeSentinel {
		from = common.Address{}.Hex()
	}
	body := map[string]any{
		"fromChainId": c.ChainID, "fromTokenAddress": from, "fromAmount": amount,
		"toChainId": lifiBitcoinChainID, "toTokenAddress": "bitcoin",
		"options": map[string]any{
			"order": "CHEAPEST", "slippage": r.Slippage / 100,
			"bridges": map[string]any{"deny": []string{"thorswap"}},
		},
	}
	if r.Destination != "" {
		body["toAddress"] = r.Destination
		body["fromAddress"] = r.Sender
	}
	var resp struct{ Routes []lifiRoute }
	err := providerJSON(ctx, "https://li.quest/v1/advanced/routes", body, "x-lifi-api-key", os.Getenv("LAZYSWAP_LIFI_API_KEY"), &resp)
	if err != nil {
		return providerAmounts{}, err
	}
	for _, route := range resp.Routes {
		if route.FromChainID != c.ChainID || route.ToChainID != lifiBitcoinChainID || route.FromAmount != amount || route.ToToken.Decimals != 8 || route.ToToken.Symbol != "BTC" || len(route.Steps) == 0 {
			continue
		}
		gas, secs, ok := lifiCosts(route.Steps, c)
		if ok {
			return providerAmounts{expected: route.ToAmount, minimum: route.ToAmountMin, gas: gas, seconds: secs}, nil
		}
	}
	return providerAmounts{}, errors.New("no eligible LI.FI route to native BTC")
}

func lifiCosts(steps []lifiStep, c chain.Config) (string, int, bool) {
	gas := new(big.Int)
	secs := 0
	for _, s := range steps {
		if s.disabledTool() {
			return "", 0, false
		}
		secs += s.Estimate.ExecutionDuration
		for _, g := range s.Estimate.GasCosts {
			if g.Token.ChainID != c.ChainID || !strings.EqualFold(g.Token.Address, common.Address{}.Hex()) {
				continue
			}
			n, ok := new(big.Int).SetString(g.Amount, 10)
			if ok && n.Sign() > 0 {
				gas.Add(gas, n)
			}
		}
	}
	if gas.Sign() == 0 {
		return "", secs, true
	}
	return balance.FormatUnits(gas, c.NativeDecimals) + " " + c.NativeSymbol, secs, true
}
