package api

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

// OONativeAddress is OpenOcean's pseudo-address for the chain's native token.
const OONativeAddress = "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"

// defaultGasPriceWei is sent when we have no better estimate; the backend
// forwards it to OpenOcean for route gas costing only.
const defaultGasPriceWei = "1000000000"

// ooAddress maps our token addressing (native sentinel) to OpenOcean's.
func ooAddress(t swap.TokenInfo) string {
	if t.Address == swap.NativeSentinel {
		return OONativeAddress
	}
	return t.Address
}

// ErrChainUnsupported means the chain has no OpenOcean coverage (testnets).
var ErrChainUnsupported = errors.New("API swap mode is not available on this chain")

// buildRequest converts a USD-denominated swap intent into a backend request.
// The USD→token conversion runs on the user's own RPC via the Flow.
func buildRequest(
	ctx context.Context,
	f *swap.Flow,
	cfg chain.Config,
	from, to swap.TokenInfo,
	usdAmount string,
	slippage float64,
) (SwapRequest, swap.UsdConversion, *big.Int, error) {
	if cfg.OpenOceanKey == "" {
		return SwapRequest{}, swap.UsdConversion{}, nil, ErrChainUnsupported
	}
	usd, err := strconv.ParseFloat(usdAmount, 64)
	if err != nil || usd <= 0 {
		return SwapRequest{}, swap.UsdConversion{}, nil, errors.New("USD amount must be a positive number")
	}
	conv, err := f.ConvertUsdToTokenAmount(ctx, from, usd)
	if err != nil {
		return SwapRequest{}, swap.UsdConversion{}, nil, err
	}
	amountBase, err := balance.ParseUnits(conv.TokenAmount, from.Decimals)
	if err != nil {
		return SwapRequest{}, swap.UsdConversion{}, nil, fmt.Errorf("parse amount: %w", err)
	}
	return SwapRequest{
		Chain:            cfg.OpenOceanKey,
		InTokenAddress:   ooAddress(from),
		OutTokenAddress:  ooAddress(to),
		AmountDecimals:   amountBase.String(),
		GasPriceDecimals: defaultGasPriceWei,
		Slippage:         slippage,
	}, conv, amountBase, nil
}

// formatBase renders a base-unit decimal string in token units.
func formatBase(baseAmount string, decimals uint8) string {
	v, ok := new(big.Int).SetString(baseAmount, 10)
	if !ok {
		return baseAmount
	}
	return balance.FormatUnits(v, decimals)
}

// QuoteFlow fetches an API-mode quote and adapts it to a FlowQuote so every
// front-end renders it exactly like a direct quote.
func QuoteFlow(
	ctx context.Context,
	c *Client,
	f *swap.Flow,
	cfg chain.Config,
	from, to swap.TokenInfo,
	usdAmount string,
	slippage float64,
) (swap.FlowQuote, error) {
	req, conv, _, err := buildRequest(ctx, f, cfg, from, to, usdAmount, slippage)
	if err != nil {
		return swap.FlowQuote{}, err
	}
	qr, err := c.SwapQuote(ctx, req)
	if err != nil {
		return swap.FlowQuote{}, err
	}

	usd, _ := strconv.ParseFloat(usdAmount, 64)
	gross, _ := strconv.ParseFloat(conv.TokenAmount, 64)
	feeAmount := gross * qr.FeePercent / 100

	return swap.FlowQuote{
		Mode:               "api",
		FromToken:          from,
		ToToken:            to,
		USDAmount:          usdAmount,
		USDAmountFormatted: swap.FormatUSD(usd),
		FromTokenAmount:    conv.TokenAmount,
		FromTokenPriceLine: fmt.Sprintf("@ %s/%s", swap.FormatUSD(conv.PricePerToken), from.Symbol),
		EstimatedOutput:    formatBase(qr.OutAmount, to.Decimals),
		MinOutput:          formatBase(qr.MinOutAmount, to.Decimals),
		Slippage:           slippage,
		FeePercent:         qr.FeePercent,
		FeeAmount:          strconv.FormatFloat(feeAmount, 'f', 6, 64),
		NetFromTokenAmount: conv.TokenAmount,
		PriceImpact:        qr.PriceImpact,
	}, nil
}

// ExecuteFlow builds the swap tx on the backend (fee injected), signs it
// locally and broadcasts it via the user's own RPC.
func ExecuteFlow(
	ctx context.Context,
	c *Client,
	f *swap.Flow,
	cfg chain.Config,
	privateKeyHex string,
	from, to swap.TokenInfo,
	usdAmount string,
	slippage float64,
) swap.FlowResult {
	fail := func(msg string) swap.FlowResult {
		usd, _ := strconv.ParseFloat(usdAmount, 64)
		return swap.FlowResult{
			Success:     false,
			FromToken:   from.Symbol,
			ToToken:     to.Symbol,
			InputAmount: swap.FormatUSD(usd),
			Err:         msg,
		}
	}

	req, _, amountBase, err := buildRequest(ctx, f, cfg, from, to, usdAmount, slippage)
	if err != nil {
		return fail(err.Error())
	}
	tx, meta, err := c.SwapTx(ctx, req)
	if err != nil {
		return fail(err.Error())
	}
	txHash, gasUsed, err := f.ExecuteRawTx(ctx, privateKeyHex, swap.RawTx{
		To:       tx.To,
		Data:     tx.Data,
		Value:    tx.Value,
		GasPrice: tx.GasPrice,
		GasLimit: tx.GasLimit,
		ChainID:  tx.ChainID,
	}, from, amountBase)
	if err != nil {
		return fail(err.Error())
	}

	usd, _ := strconv.ParseFloat(usdAmount, 64)
	return swap.FlowResult{
		Success:      true,
		TxHash:       txHash,
		FromToken:    from.Symbol,
		ToToken:      to.Symbol,
		InputAmount:  swap.FormatUSD(usd),
		OutputAmount: formatBase(meta.OutAmount, to.Decimals),
		GasUsed:      gasUsed,
	}
}
