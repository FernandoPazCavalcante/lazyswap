package api

import (
	"context"
	"errors"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
)

func TestBuildRequestRefusesUnsupportedChain(t *testing.T) {
	// Testnets have no OpenOcean coverage — the error must fire before any
	// RPC/USD conversion is attempted (a nil Flow proves it).
	cfg := chain.Get("bsc_testnet")
	from := swap.TokenInfo{Symbol: "tBNB", Address: swap.NativeSentinel, Decimals: 18}
	to := swap.TokenInfo{Symbol: "USDT", Address: "0x1", Decimals: 18}

	_, _, _, err := buildRequest(context.Background(), nil, cfg, from, to, "5", 0.5)
	if !errors.Is(err, ErrChainUnsupported) {
		t.Fatalf("want ErrChainUnsupported, got %v", err)
	}
}

func TestBuildRequestRejectsBadUSD(t *testing.T) {
	cfg := chain.Get("bsc")
	from := swap.TokenInfo{Symbol: "BNB", Address: swap.NativeSentinel, Decimals: 18}
	to := swap.TokenInfo{Symbol: "USDT", Address: "0x1", Decimals: 18}

	for _, usd := range []string{"0", "-5", "abc", ""} {
		if _, _, _, err := buildRequest(context.Background(), nil, cfg, from, to, usd, 0.5); err == nil {
			t.Errorf("usd=%q must error", usd)
		}
	}
}

func TestExecuteFlowFailsClosed(t *testing.T) {
	// Any build failure yields a Success=false FlowResult, never a panic.
	cfg := chain.Get("bsc_testnet")
	from := swap.TokenInfo{Symbol: "tBNB", Address: swap.NativeSentinel, Decimals: 18}
	to := swap.TokenInfo{Symbol: "USDT", Address: "0x1", Decimals: 18}

	res := ExecuteFlow(context.Background(), New("http://127.0.0.1:1"), nil, cfg, "00", from, to, "5", 0.5)
	if res.Success {
		t.Fatal("must fail on unsupported chain")
	}
	if res.FromToken != "tBNB" || res.ToToken != "USDT" || res.InputAmount == "" {
		t.Fatalf("failure result incomplete: %+v", res)
	}
}

func TestFormatBaseMalformedPassthrough(t *testing.T) {
	if got := formatBase("not-a-number", 18); got != "not-a-number" {
		t.Fatalf("malformed input must pass through, got %q", got)
	}
	if got := formatBase("1500000000000000000", 18); got != "1.5" {
		t.Fatalf("formatBase = %q, want 1.5", got)
	}
}
