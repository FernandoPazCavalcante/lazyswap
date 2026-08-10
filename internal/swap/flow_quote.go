package swap

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/FernandoPazCavalcante/lazyswap/internal/dex"
)

// Quote fetches a full USD-denominated swap quote for UI preview.
func (f *Flow) Quote(
	ctx context.Context,
	fromToken, toToken TokenInfo,
	usdAmount string,
	slippage float64,
	walletAddress string,
) (FlowQuote, error) {
	usd, err := strconv.ParseFloat(usdAmount, 64)
	if err != nil || usd <= 0 {
		return FlowQuote{}, errors.New("USD amount must be a positive number")
	}

	conv, err := f.ConvertUsdToTokenAmount(ctx, fromToken, usd)
	if err != nil {
		return FlowQuote{}, err
	}

	gross, _ := strconv.ParseFloat(conv.TokenAmount, 64)
	fee := CalcFee(gross)
	netFromTokenAmount := strconv.FormatFloat(fee.NetAmount, 'f', 6, 64)

	fromAddr := f.ResolveAddress(fromToken.Address)
	toAddr := f.ResolveAddress(toToken.Address)

	provider := dex.NewUniswapV2(f.client, f.chainKey)
	quoteSvc := NewQuote(provider)
	dexQuote, err := quoteSvc.Get(ctx, fromAddr, toAddr, netFromTokenAmount)
	if err != nil {
		return FlowQuote{}, err
	}

	estimated, _ := strconv.ParseFloat(dexQuote.EstimatedOutput, 64)
	minOutput := strconv.FormatFloat(estimated*(1-slippage/100), 'f', 6, 64)

	needsApproval := false
	if fromToken.Address != NativeSentinel {
		needsApproval = f.checkNeedsApproval(ctx, fromAddr, walletAddress, fromToken.Decimals, conv.TokenAmount)
	}

	return FlowQuote{
		FromToken:          fromToken,
		ToToken:            toToken,
		USDAmount:          usdAmount,
		USDAmountFormatted: FormatUSD(usd),
		FromTokenAmount:    conv.TokenAmount,
		FromTokenPriceLine: fmt.Sprintf("@ %s/%s", FormatUSD(conv.PricePerToken), fromToken.Symbol),
		EstimatedOutput:    dexQuote.EstimatedOutput,
		MinOutput:          minOutput,
		Slippage:           slippage,
		NeedsApproval:      needsApproval,
		FeePercent:         fee.FeePercent,
		FeeAmount:          strconv.FormatFloat(fee.FeeAmount, 'f', 6, 64),
		NetFromTokenAmount: netFromTokenAmount,
	}, nil
}
