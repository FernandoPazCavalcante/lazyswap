package swap

import (
	"context"
	"fmt"
	"strconv"

	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/dex"
)

// Execute re-converts the USD amount to a fresh on-chain price, applies the
// LazySwap fee, requotes for minOutput, and dispatches the appropriate swap.
func (f *Flow) Execute(
	ctx context.Context,
	privateKey string,
	fromToken, toToken TokenInfo,
	usdAmount string,
	slippage float64,
) FlowResult {
	applog.Tracef("swap.Flow.Execute — %s → %s $%s slippage=%g%%",
		fromToken.Symbol, toToken.Symbol, usdAmount, slippage)

	usd, err := strconv.ParseFloat(usdAmount, 64)
	if err != nil || usd <= 0 {
		return failResult(fromToken, toToken, usdAmount, "USD amount must be a positive number")
	}

	executor, err := NewExecutor(f.client, privateKey, f.chainKey)
	if err != nil {
		return failResult(fromToken, toToken, usdAmount, err.Error())
	}

	conv, err := f.ConvertUsdToTokenAmount(ctx, fromToken, usd)
	if err != nil {
		return failResult(fromToken, toToken, usdAmount, err.Error())
	}

	fromTokenAmount := conv.TokenAmount

	fromAddr := f.ResolveAddress(fromToken.Address)
	toAddr := f.ResolveAddress(toToken.Address)

	provider := dex.NewUniswapV2(f.client, f.chainKey)
	quoteSvc := NewQuote(provider)
	dexQuote, err := quoteSvc.Get(ctx, fromAddr, toAddr, fromTokenAmount)
	if err != nil {
		return failResult(fromToken, toToken, usdAmount, err.Error())
	}
	minOutput, err := minOutputFor(dexQuote.EstimatedOutput, slippage, toToken.Decimals)
	if err != nil {
		return failResult(fromToken, toToken, usdAmount, err.Error())
	}

	result, err := executor.ExecuteFullSwap(ctx, fromToken.Address, toToken.Address, fromTokenAmount, minOutput, slippage)
	if err != nil {
		applog.Error("swap.Flow.Execute failed", err)
		return failResult(fromToken, toToken, usdAmount, err.Error())
	}

	applog.Infof("swap executed — txHash=%s in=%s out=%s gas=%s",
		result.TxHash, result.InputAmount, result.OutputAmount, result.GasUsed)

	return FlowResult{
		Success:      true,
		TxHash:       result.TxHash,
		FromToken:    fromToken.Symbol,
		ToToken:      toToken.Symbol,
		InputAmount:  fmt.Sprintf("$%s", usdAmount),
		OutputAmount: result.OutputAmount,
		GasUsed:      result.GasUsed,
	}
}
