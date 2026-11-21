package swap

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/FernandoPazCavalcante/lazyswap/internal/applog"
	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/thorchain"
)

// ── THORchain cross-chain (EVM → BTC) ────────────────────────────────────────

// btcToken is the pseudo-token shown in the ToToken slot of a THORchain quote.
var btcToken = TokenInfo{Symbol: "BTC", Address: "bitcoin", Decimals: 8}

// GetThorchainQuote converts USD to the input token and quotes it against THORNode.
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
	fromTokenAmount := conv.TokenAmount

	fromAsset, err := f.thorAssetString(fromToken)
	if err != nil {
		return FlowQuote{}, err
	}
	thorAmount, err := thorchain.ToThorBaseUnits(fromTokenAmount)
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
		IsThorchain:          true,
		ExpiresAt:            quote.ExpiresAt,
		SourceGasBudget:      f.thorSourceGasBudget(ctx, quote.GasLimit),
		ThorInbound:          quote.InboundAddress,
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
	fromTokenAmount := conv.TokenAmount

	fromAsset, err := f.thorAssetString(fromToken)
	if err != nil {
		return FlowQuote{}, err
	}
	thorAmount, err := thorchain.ToThorBaseUnits(fromTokenAmount)
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
		IsThorchain:          true,
		ThorEstimatedSeconds: quote.EstimatedSeconds,
		ThorFees:             quote.TotalFees,
		EstimatedOutputSats:  quote.ExpectedSats,
		ThorMinTokenAmount:   minTok,
		ThorMinUSD:           minUSD,
	}, nil
}

// thorSourceGasBudget prices the deposit's gas budget in the source asset.
// Approval, when needed, is a separate transaction.
func (f *Flow) thorSourceGasBudget(ctx context.Context, gasLimit int64) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	price, err := f.client.SuggestGasPrice(ctx)
	if err != nil || price.Sign() <= 0 {
		return ""
	}
	cost := new(big.Int).Mul(price, big.NewInt(gasLimit))
	return balance.FormatUnits(cost, f.chain.NativeDecimals) + " " + f.chain.NativeSymbol
}

// ExecuteThorchain submits the reviewed amount and memo after rechecking the
// current vault.
func (f *Flow) ExecuteThorchain(ctx context.Context, privateKey string, quote FlowQuote) FlowResult {
	fail := func(reason string) FlowResult {
		return failResult(quote.FromToken, btcToken, quote.USDAmount, reason)
	}
	if !quote.IsThorchain || quote.ThorMemo == "" || quote.BTCAddress == "" {
		return fail("THORChain quote is required")
	}
	if !time.Now().Before(quote.ExpiresAt) {
		return fail("quote expired — refresh quotes before confirming")
	}
	executor, err := thorchain.NewExecutor(f.client, privateKey, f.chainKey)
	if err != nil {
		return fail(err.Error())
	}
	inbound, err := thorchain.NewProvider().GetInboundAddress(ctx, f.chainKey)
	if err != nil {
		return fail(err.Error())
	}
	if quote.ThorInbound != "" && !strings.EqualFold(quote.ThorInbound, inbound.VaultAddress) {
		return fail("THORChain vault changed — refresh quotes before confirming")
	}

	res, err := f.sendThorSwap(ctx, executor, inbound, quote)
	if err != nil {
		applog.Error("swap.Flow.ExecuteThorchain failed", err)
		return fail(err.Error())
	}
	applog.Infof("thorchain swap submitted — txHash=%s expectedBtc=%s gas=%s",
		res.TxHash, quote.EstimatedOutput, res.GasUsed)

	return FlowResult{
		Success:      true,
		TxHash:       res.TxHash,
		FromToken:    quote.FromToken.Symbol,
		ToToken:      "BTC",
		InputAmount:  quote.USDAmountFormatted,
		OutputAmount: quote.EstimatedOutput,
		GasUsed:      res.GasUsed,
	}
}

// sendThorSwap dispatches the EVM deposit transaction to the inbound vault,
// routing native and ERC-20 inputs to their executor variants.
func (f *Flow) sendThorSwap(
	ctx context.Context,
	executor *thorchain.Executor,
	inbound thorchain.InboundAddress,
	quote FlowQuote,
) (thorchain.Result, error) {
	fromToken := quote.FromToken
	if fromToken.Address == NativeSentinel {
		return executor.SwapNativeToBtc(ctx, inbound.VaultAddress, quote.FromTokenAmount, quote.ThorMemo, quote.ExpiresAt)
	}
	routerAddress := inbound.Router
	if routerAddress == "" {
		routerAddress = f.chain.RouterAddress
	}
	return executor.SwapErc20ToBtc(ctx, routerAddress, inbound.VaultAddress,
		fromToken.Address, fromToken.Symbol, fromToken.Decimals, quote.FromTokenAmount, quote.ThorMemo, quote.ExpiresAt)
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
