package swap

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/thorchain"
)

// ── THORchain cross-chain (EVM → BTC) ────────────────────────────────────────

// btcToken is the pseudo-token shown in the ToToken slot of a THORchain quote.
var btcToken = TokenInfo{Symbol: "BTC", Address: "bitcoin", Decimals: 8}

// GetThorchainQuote converts USD → from-token via the DEX, applies the LazySwap
// fee, then asks Midgard how much BTC the net amount yields. Mirrors
// swap-flow.service.ts getThorchainQuote.
func (f *Flow) GetThorchainQuote(
	ctx context.Context,
	fromToken TokenInfo,
	usdAmount, btcAddress string,
) (FlowQuote, error) {
	usd, err := strconv.ParseFloat(usdAmount, 64)
	if err != nil || usd <= 0 {
		return FlowQuote{}, errors.New("USD amount must be a positive number")
	}
	if strings.TrimSpace(btcAddress) == "" {
		return FlowQuote{}, errors.New("bitcoin address is required")
	}

	conv, err := f.ConvertUsdToTokenAmount(ctx, fromToken, usd)
	if err != nil {
		return FlowQuote{}, err
	}
	gross, _ := strconv.ParseFloat(conv.TokenAmount, 64)
	fee := CalcFee(gross)
	netFromTokenAmount := strconv.FormatFloat(fee.NetAmount, 'f', 6, 64)

	fromAsset, err := f.thorAssetString(fromToken)
	if err != nil {
		return FlowQuote{}, err
	}
	thorAmount, err := thorchain.ToThorBaseUnits(netFromTokenAmount)
	if err != nil {
		return FlowQuote{}, err
	}

	quote, err := thorchain.NewProvider().GetSwapQuote(ctx, fromAsset, thorAmount, btcAddress)
	if err != nil {
		var bm *thorchain.BelowMinError
		if errors.As(err, &bm) {
			return FlowQuote{}, belowMinErr(fromToken.Symbol, bm.MinIn, conv.PricePerToken)
		}
		return FlowQuote{}, err
	}

	minTok, minUSD := thorMinFields(quote.RecommendedMinIn, conv.PricePerToken)
	return FlowQuote{
		FromToken:            fromToken,
		ToToken:              btcToken,
		USDAmount:            usdAmount,
		USDAmountFormatted:   FormatUSD(usd),
		FromTokenAmount:      conv.TokenAmount,
		FromTokenPriceLine:   fmt.Sprintf("@ %s/%s", FormatUSD(conv.PricePerToken), fromToken.Symbol),
		EstimatedOutput:      quote.ExpectedOutput,
		MinOutput:            quote.MinOutput,
		Slippage:             0, // THORchain enforces slippage via the memo min-output
		NeedsApproval:        fromToken.Address != NativeSentinel,
		FeePercent:           fee.FeePercent,
		FeeAmount:            strconv.FormatFloat(fee.FeeAmount, 'f', 6, 64),
		NetFromTokenAmount:   netFromTokenAmount,
		IsThorchain:          true,
		ThorEstimatedSeconds: quote.EstimatedSeconds,
		ThorFees:             quote.TotalFees,
		BTCAddress:           btcAddress,
		ThorMemo:             quote.Memo,
		EstimatedOutputSats:  quote.ExpectedSats,
		ThorMinTokenAmount:   minTok,
		ThorMinUSD:           minUSD,
	}, nil
}

// EstimateThorchain returns a destination-less price preview for fromToken →
// BTC: USD → token (DEX) + LazySwap fee, then a THORchain quote with no memo.
// Used to show "≈ X BTC (Y sats)" as soon as the user enters a USD amount,
// before they type a Bitcoin address.
func (f *Flow) EstimateThorchain(
	ctx context.Context,
	fromToken TokenInfo,
	usdAmount string,
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

	fromAsset, err := f.thorAssetString(fromToken)
	if err != nil {
		return FlowQuote{}, err
	}
	thorAmount, err := thorchain.ToThorBaseUnits(netFromTokenAmount)
	if err != nil {
		return FlowQuote{}, err
	}

	quote, err := thorchain.NewProvider().GetSwapQuote(ctx, fromAsset, thorAmount, "")
	if err != nil {
		// Below the minimum is a displayable state, not a hard error: return a
		// quote flagged ThorBelowMin so the panel can show the disclaimer + block.
		var bm *thorchain.BelowMinError
		if errors.As(err, &bm) {
			minTok, minUSD := thorMin(bm.MinIn, conv.PricePerToken)
			return FlowQuote{
				FromToken:          fromToken,
				ToToken:            btcToken,
				USDAmount:          usdAmount,
				USDAmountFormatted: FormatUSD(usd),
				FromTokenAmount:    conv.TokenAmount,
				FromTokenPriceLine: fmt.Sprintf("@ %s/%s", FormatUSD(conv.PricePerToken), fromToken.Symbol),
				NeedsApproval:      fromToken.Address != NativeSentinel,
				NetFromTokenAmount: netFromTokenAmount,
				IsThorchain:        true,
				ThorBelowMin:       true,
				ThorMinTokenAmount: minTok,
				ThorMinUSD:         minUSD,
			}, nil
		}
		return FlowQuote{}, err
	}

	minTok, minUSD := thorMinFields(quote.RecommendedMinIn, conv.PricePerToken)
	return FlowQuote{
		FromToken:            fromToken,
		ToToken:              btcToken,
		USDAmount:            usdAmount,
		USDAmountFormatted:   FormatUSD(usd),
		FromTokenAmount:      conv.TokenAmount,
		FromTokenPriceLine:   fmt.Sprintf("@ %s/%s", FormatUSD(conv.PricePerToken), fromToken.Symbol),
		EstimatedOutput:      quote.ExpectedOutput,
		MinOutput:            quote.MinOutput,
		NeedsApproval:        fromToken.Address != NativeSentinel,
		FeePercent:           fee.FeePercent,
		FeeAmount:            strconv.FormatFloat(fee.FeeAmount, 'f', 6, 64),
		NetFromTokenAmount:   netFromTokenAmount,
		IsThorchain:          true,
		ThorEstimatedSeconds: quote.EstimatedSeconds,
		ThorFees:             quote.TotalFees,
		EstimatedOutputSats:  quote.ExpectedSats,
		ThorMinTokenAmount:   minTok,
		ThorMinUSD:           minUSD,
	}, nil
}

// ExecuteThorchain re-quotes at execution time (fresh price + memo), fetches the
// current inbound vault, and sends the EVM transaction. Mirrors
// swap-flow.service.ts executeThorchain.
func (f *Flow) ExecuteThorchain(
	ctx context.Context,
	privateKey string,
	fromToken TokenInfo,
	usdAmount, btcAddress string,
) FlowResult {
	applog.Tracef("swap.Flow.ExecuteThorchain — %s → BTC $%s btc=%s",
		fromToken.Symbol, usdAmount, btcAddress)

	usd, err := strconv.ParseFloat(usdAmount, 64)
	if err != nil || usd <= 0 {
		return failResult(fromToken, btcToken, usdAmount, "USD amount must be a positive number")
	}
	if strings.TrimSpace(btcAddress) == "" {
		return failResult(fromToken, btcToken, usdAmount, "Bitcoin address is required")
	}

	executor, err := thorchain.NewExecutor(f.client, privateKey, f.chainKey)
	if err != nil {
		return failResult(fromToken, btcToken, usdAmount, err.Error())
	}

	conv, err := f.ConvertUsdToTokenAmount(ctx, fromToken, usd)
	if err != nil {
		return failResult(fromToken, btcToken, usdAmount, err.Error())
	}
	gross, _ := strconv.ParseFloat(conv.TokenAmount, 64)
	fee := CalcFee(gross)
	netFromTokenAmount := strconv.FormatFloat(fee.NetAmount, 'f', 6, 64)

	fromAsset, err := f.thorAssetString(fromToken)
	if err != nil {
		return failResult(fromToken, btcToken, usdAmount, err.Error())
	}
	thorAmount, err := thorchain.ToThorBaseUnits(netFromTokenAmount)
	if err != nil {
		return failResult(fromToken, btcToken, usdAmount, err.Error())
	}

	provider := thorchain.NewProvider()
	quote, err := provider.GetSwapQuote(ctx, fromAsset, thorAmount, btcAddress)
	if err != nil {
		var bm *thorchain.BelowMinError
		if errors.As(err, &bm) {
			return failResult(fromToken, btcToken, usdAmount,
				belowMinErr(fromToken.Symbol, bm.MinIn, conv.PricePerToken).Error())
		}
		return failResult(fromToken, btcToken, usdAmount, err.Error())
	}
	// Always fetch a fresh inbound vault — THORchain rotates them.
	inbound, err := provider.GetInboundAddress(ctx, f.chainKey)
	if err != nil {
		return failResult(fromToken, btcToken, usdAmount, err.Error())
	}

	res, err := f.sendThorSwap(ctx, executor, inbound, fromToken, netFromTokenAmount, quote.Memo)
	if err != nil {
		applog.Error("swap.Flow.ExecuteThorchain failed", err)
		return failResult(fromToken, btcToken, usdAmount, err.Error())
	}

	applog.Infof("thorchain swap executed — txHash=%s expectedBtc=%s gas=%s",
		res.TxHash, quote.ExpectedOutput, res.GasUsed)

	return FlowResult{
		Success:      true,
		TxHash:       res.TxHash,
		FromToken:    fromToken.Symbol,
		ToToken:      "BTC",
		InputAmount:  fmt.Sprintf("$%s", usdAmount),
		OutputAmount: quote.ExpectedOutput,
		GasUsed:      res.GasUsed,
	}
}

// sendThorSwap dispatches the EVM deposit transaction to the inbound vault,
// routing native and ERC-20 inputs to their executor variants.
func (f *Flow) sendThorSwap(
	ctx context.Context,
	executor *thorchain.Executor,
	inbound thorchain.InboundAddress,
	fromToken TokenInfo,
	netFromTokenAmount, memo string,
) (thorchain.Result, error) {
	if fromToken.Address == NativeSentinel {
		return executor.SwapNativeToBtc(ctx, inbound.VaultAddress, netFromTokenAmount, memo)
	}
	routerAddress := inbound.Router
	if routerAddress == "" {
		routerAddress = f.chain.RouterAddress
	}
	return executor.SwapErc20ToBtc(ctx, routerAddress, inbound.VaultAddress,
		fromToken.Address, fromToken.Symbol, fromToken.Decimals, netFromTokenAmount, memo)
}

// thorMin renders THORChain's recommended minimum (1e8 base units) as a
// from-token amount and its approximate USD value at pricePerToken.
func thorMin(minIn int64, pricePerToken float64) (tokenAmount, usd string) {
	mt := float64(minIn) / 1e8
	return strconv.FormatFloat(mt, 'f', 6, 64), FormatUSD(mt * pricePerToken)
}

// thorMinFields is thorMin but yields empty strings when the minimum is unknown
// (0), so callers can show it only when present.
func thorMinFields(minIn int64, pricePerToken float64) (tokenAmount, usd string) {
	if minIn <= 0 {
		return "", ""
	}
	return thorMin(minIn, pricePerToken)
}

// belowMinErr builds the user-facing "below minimum" error for symbol.
func belowMinErr(symbol string, minIn int64, pricePerToken float64) error {
	t, u := thorMin(minIn, pricePerToken)
	return fmt.Errorf("amount below THORChain minimum: need ≥ %s %s (≈ %s) to cover the Bitcoin network fee", t, symbol, u)
}

// thorAssetString builds the THORchain asset identifier for fromToken.
func (f *Flow) thorAssetString(fromToken TokenInfo) (string, error) {
	if fromToken.Address == NativeSentinel {
		return thorchain.BuildNativeAssetString(f.chainKey, fromToken.Symbol)
	}
	return thorchain.BuildErc20AssetString(f.chainKey, fromToken.Symbol, fromToken.Address)
}
