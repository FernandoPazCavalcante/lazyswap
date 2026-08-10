package swap

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

// TokenInfo identifies a token in the swap flow. Address may be NativeSentinel.
type TokenInfo struct {
	Symbol   string `json:"symbol"`
	Address  string `json:"address"`
	Decimals uint8  `json:"decimals"`
}

// FlowQuote is the full USD-denominated quote returned by Flow.Quote.
type FlowQuote struct {
	FromToken TokenInfo `json:"fromToken"`
	ToToken   TokenInfo `json:"toToken"`

	USDAmount          string `json:"usdAmount"`          // raw user input, e.g. "50"
	USDAmountFormatted string `json:"usdAmountFormatted"` // "$50.00"

	FromTokenAmount    string `json:"fromTokenAmount"`    // gross from-token amount (pre-fee)
	FromTokenPriceLine string `json:"fromTokenPriceLine"` // "@ $600.12/BNB"

	EstimatedOutput string `json:"estimatedOutput"` // raw DEX quote for the net amount
	MinOutput       string `json:"minOutput"`       // estimated * (1 - slippage/100)

	Slippage      float64 `json:"slippage"`
	NeedsApproval bool    `json:"needsApproval"`

	FeePercent         float64 `json:"feePercent"`
	FeeAmount          string  `json:"feeAmount"`          // formatted, e.g. "0.000125"
	NetFromTokenAmount string  `json:"netFromTokenAmount"` // gross - fee, formatted

	// Mode is "direct" (on-chain V2 router) or "api" (backend/OpenOcean route).
	// Empty means direct (pre-hybrid quotes).
	Mode        string `json:"mode,omitempty"`
	PriceImpact string `json:"priceImpact,omitempty"` // API mode only, e.g. "0.12%"

	// THORchain-specific fields, set when IsThorchain is true (EVM → BTC).
	IsThorchain          bool   `json:"isThorchain"`
	ThorEstimatedSeconds int    `json:"thorEstimatedSeconds,omitempty"` // settlement estimate in seconds
	ThorFees             string `json:"thorFees,omitempty"`             // total fees in BTC, formatted
	BTCAddress           string `json:"btcAddress,omitempty"`           // destination Bitcoin address (empty for estimate)
	ThorMemo             string `json:"thorMemo,omitempty"`             // encoded THORchain memo (empty for estimate)
	EstimatedOutputSats  int64  `json:"estimatedOutputSats,omitempty"`  // expected BTC output in sats (1e8 base units)

	// THORChain enforces a per-token minimum input (covers the BTC network
	// outbound fee). These describe it for the current token/price.
	ThorBelowMin       bool   `json:"thorBelowMin,omitempty"`       // true when the requested amount is under the minimum
	ThorMinTokenAmount string `json:"thorMinTokenAmount,omitempty"` // minimum input in from-token units, e.g. "4.016789"
	ThorMinUSD         string `json:"thorMinUSD,omitempty"`         // that minimum as approx USD, e.g. "$4.02"
}

// FlowResult is the outcome of Flow.Execute.
type FlowResult struct {
	Success      bool   `json:"success"`
	TxHash       string `json:"txHash"`
	FromToken    string `json:"fromToken"`
	ToToken      string `json:"toToken"`
	InputAmount  string `json:"inputAmount"` // formatted as "$<usd>"
	OutputAmount string `json:"outputAmount"`
	GasUsed      string `json:"gasUsed"`
	Err          string `json:"error,omitempty"`
}

// UsdConversion is the output of Flow.ConvertUsdToTokenAmount.
type UsdConversion struct {
	TokenAmount   string
	PricePerToken float64
}

// Flow orchestrates USD→token conversion, quoting, fee application, approval
// checks, and execution. Mirrors swap-flow.service.ts (THORchain excluded).
type Flow struct {
	chainKey string
	chain    chain.Config
	client   *ethclient.Client
}

// NewFlow dials the chain RPC and returns a configured Flow.
func NewFlow(chainKey string) (*Flow, error) {
	c := chain.Get(chainKey)
	client, err := ethclient.Dial(c.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", c.RPCURL, err)
	}
	return &Flow{chainKey: chainKey, chain: c, client: client}, nil
}

// Close releases the underlying RPC client.
func (f *Flow) Close() { f.client.Close() }

// NativeToken returns the SwapTokenInfo for the chain's gas token.
func (f *Flow) NativeToken() TokenInfo {
	return TokenInfo{
		Symbol:   f.chain.NativeSymbol,
		Address:  NativeSentinel,
		Decimals: f.chain.NativeDecimals,
	}
}

// ResolveAddress swaps the NativeSentinel for the chain's wrapped-native
// address; otherwise returns addr unchanged.
func (f *Flow) ResolveAddress(addr string) string {
	if addr == NativeSentinel {
		return f.chain.WrappedNative
	}
	return addr
}

// ConvertUsdToTokenAmount fetches a fresh DEX-derived price for `token` and
// returns how much of it the given USD buys, plus the implied per-token price.
// Tries direct stablecoin→token first, falls back to stablecoin→wrappedNative→token.
func (f *Flow) ConvertUsdToTokenAmount(ctx context.Context, token TokenInfo, usd float64) (UsdConversion, error) {
	stable := f.chain.StablecoinAddr
	stableDecimals := f.stablecoinDecimals()
	tokenAddr := f.ResolveAddress(token.Address)

	amountIn, err := balance.ParseUnits(strconv.FormatFloat(usd, 'f', int(stableDecimals), 64), stableDecimals)
	if err != nil {
		return UsdConversion{}, fmt.Errorf("parse usd: %w", err)
	}

	router := common.HexToAddress(f.chain.RouterAddress)
	contract := bind.NewBoundContract(router, chain.RouterABI, f.client, nil, nil)

	var tokenAmountRaw *big.Int

	// Try direct route: stable → token.
	if !strings.EqualFold(tokenAddr, stable) {
		if raw, err := callAmountsOut(ctx, contract, amountIn, []common.Address{
			common.HexToAddress(stable),
			common.HexToAddress(tokenAddr),
		}); err == nil {
			tokenAmountRaw = raw
		}
	}

	// Fallback: stable → wrappedNative → token.
	if tokenAmountRaw == nil && !strings.EqualFold(tokenAddr, f.chain.WrappedNative) {
		raw, err := callAmountsOut(ctx, contract, amountIn, []common.Address{
			common.HexToAddress(stable),
			common.HexToAddress(f.chain.WrappedNative),
			common.HexToAddress(tokenAddr),
		})
		if err == nil {
			tokenAmountRaw = raw
		}
	}

	// Last resort (token IS wrappedNative): stable → wrappedNative direct.
	if tokenAmountRaw == nil {
		raw, err := callAmountsOut(ctx, contract, amountIn, []common.Address{
			common.HexToAddress(stable),
			common.HexToAddress(tokenAddr),
		})
		if err == nil {
			tokenAmountRaw = raw
		}
	}

	if tokenAmountRaw == nil || tokenAmountRaw.Sign() == 0 {
		return UsdConversion{}, fmt.Errorf("cannot price %s — no liquidity path from stablecoin", token.Symbol)
	}

	tokenAmountStr := balance.FormatUnits(tokenAmountRaw, token.Decimals)
	tokenAmountF, _ := strconv.ParseFloat(tokenAmountStr, 64)
	pricePerToken := 0.0
	if tokenAmountF != 0 {
		pricePerToken = usd / tokenAmountF
	}
	return UsdConversion{TokenAmount: tokenAmountStr, PricePerToken: pricePerToken}, nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func (f *Flow) stablecoinDecimals() uint8 {
	lower := strings.ToLower(f.chain.StablecoinAddr)
	for _, t := range f.chain.Tokens {
		if strings.ToLower(t.Address) == lower {
			return t.Decimals
		}
	}
	return 18
}

func (f *Flow) checkNeedsApproval(ctx context.Context, tokenAddr, walletAddr string, decimals uint8, requiredAmount string) bool {
	if !common.IsHexAddress(walletAddr) {
		return true
	}
	contract := bind.NewBoundContract(common.HexToAddress(tokenAddr), chain.ERC20ABI, f.client, nil, nil)
	var out []interface{}
	err := contract.Call(&bind.CallOpts{Context: ctx}, &out, "allowance",
		common.HexToAddress(walletAddr), common.HexToAddress(f.chain.RouterAddress))
	if err != nil || len(out) == 0 {
		return true
	}
	raw, ok := out[0].(*big.Int)
	if !ok {
		return true
	}
	current, _ := strconv.ParseFloat(balance.FormatUnits(raw, decimals), 64)
	want, _ := strconv.ParseFloat(requiredAmount, 64)
	return current < want
}

func callAmountsOut(ctx context.Context, contract *bind.BoundContract, amountIn *big.Int, path []common.Address) (*big.Int, error) {
	var out []interface{}
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &out, "getAmountsOut", amountIn, path); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("empty result")
	}
	amounts, ok := out[0].([]*big.Int)
	if !ok || len(amounts) == 0 {
		return nil, errors.New("unexpected amounts type")
	}
	return amounts[len(amounts)-1], nil
}

func failResult(from, to TokenInfo, usdAmount, msg string) FlowResult {
	return FlowResult{
		Success:      false,
		Err:          msg,
		FromToken:    from.Symbol,
		ToToken:      to.Symbol,
		InputAmount:  fmt.Sprintf("$%s", usdAmount),
		OutputAmount: "0",
	}
}

// ── pure helpers (exported for UI / tests) ──────────────────────────────────

// ApplySlippage returns estimated * (1 - slip/100) as a 6-decimal string.
// Returns "0" for unparseable inputs (matches the Bun reference).
func ApplySlippage(estimated string, slippagePercent float64) string {
	v, err := strconv.ParseFloat(estimated, 64)
	if err != nil {
		return "0"
	}
	return strconv.FormatFloat(v*(1-slippagePercent/100), 'f', 6, 64)
}

// FormatUSD renders amount as "$1,234.56". Reuses the balance package helper.
func FormatUSD(amount float64) string { return balance.FormatUSD(amount) }

// FormatSwapResult renders a status-bar line for the UI.
func FormatSwapResult(r FlowResult) string {
	if !r.Success {
		msg := r.Err
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Sprintf("Swap failed: %s", msg)
	}
	shortHash := r.TxHash
	if len(shortHash) > 10 {
		shortHash = shortHash[:10]
	}
	return fmt.Sprintf("Swap complete — %s → %s %s  TX: %s...",
		r.InputAmount, r.OutputAmount, r.ToToken, shortHash)
}
