package mcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/balance"
	"github.com/FernandoPazCavalcante/lazyswap/internal/paths"
	"github.com/FernandoPazCavalcante/lazyswap/internal/safety"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// newTestServer builds a server over a throwaway DB. No RPC is ever dialed —
// tests only exercise validation and guardrails.
func newTestServer(t *testing.T, opts Options) *server {
	t.Helper()
	t.Setenv("LAZYSWAP_TEST", "1")
	dir := t.TempDir()
	paths.Override(dir)
	t.Cleanup(func() { paths.Override("") })
	dao, err := wallet.OpenAt(filepath.Join(dir, "wallets.db"))
	if err != nil {
		t.Fatalf("open dao: %v", err)
	}
	t.Cleanup(func() { dao.Close() })
	return &server{
		dao:    dao,
		opts:   opts,
		safety: safety.NewWith(stubChecker{}),
		flows:  map[string]*swap.Flow{},
		bals:   map[string]*balance.Service{},
	}
}

// stubChecker returns a fixed report without any network.
type stubChecker struct{ rep safety.Report }

func (c stubChecker) Check(ctx context.Context, chainKey, tokenAddr string) (safety.Report, error) {
	return c.rep, nil
}

func TestRunRejectsBadOptions(t *testing.T) {
	// Trading without a cap.
	err := Run(context.Background(), Options{AllowTrading: true})
	if err == nil || !strings.Contains(err.Error(), "--max-usd") {
		t.Fatalf("expected --max-usd error, got %v", err)
	}

	// Unknown chain in the allowlist.
	err = Run(context.Background(), Options{Chains: []string{"nopechain"}})
	if err == nil || !strings.Contains(err.Error(), "nopechain") {
		t.Fatalf("expected unknown-chain error, got %v", err)
	}

	// Trading without LAZYSWAP_PASSWORD.
	t.Setenv("LAZYSWAP_TEST", "1")
	t.Setenv("LAZYSWAP_PASSWORD", "")
	dir := t.TempDir()
	paths.Override(dir)
	t.Cleanup(func() { paths.Override("") })
	err = Run(context.Background(), Options{AllowTrading: true, MaxUSD: 10})
	if err == nil || !strings.Contains(err.Error(), "LAZYSWAP_PASSWORD") {
		t.Fatalf("expected password error, got %v", err)
	}
}

func TestResolveSwapValidation(t *testing.T) {
	s := newTestServer(t, Options{})

	// Non-positive USD refused.
	if _, _, _, _, _, err := s.resolveSwap(swapIn{USD: 0, From: "BNB", To: "USDT"}); err == nil {
		t.Fatal("expected error for usd=0")
	}

	// Unknown token refused.
	if _, _, _, _, _, err := s.resolveSwap(swapIn{USD: 5, From: "NOPECOIN", To: "USDT"}); err == nil {
		t.Fatal("expected error for unknown token")
	}

	// Unknown chain refused.
	if _, _, _, _, _, err := s.resolveSwap(swapIn{USD: 5, From: "BNB", To: "USDT", Chain: "nopechain"}); err == nil {
		t.Fatal("expected error for unknown chain")
	}

	// Valid input resolves on the default chain (bsc) with default slippage.
	key, from, to, usd, slip, err := s.resolveSwap(swapIn{USD: 5, From: "bnb", To: "USDT"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if key != "bsc" || from.Symbol != "BNB" || to.Symbol != "USDT" || usd != "5" || slip <= 0 {
		t.Fatalf("resolved wrong: key=%s from=%+v to=%+v usd=%s slip=%v", key, from, to, usd, slip)
	}
}

func TestSwapExecuteRefusesOverCap(t *testing.T) {
	s := newTestServer(t, Options{AllowTrading: true, MaxUSD: 10})
	_, _, err := s.swapExecute(context.Background(), nil, swapIn{USD: 50, From: "BNB", To: "USDT"})
	if err == nil || !strings.Contains(err.Error(), "--max-usd") {
		t.Fatalf("expected cap refusal, got %v", err)
	}
}

func TestSwapExecuteRefusesHighRisk(t *testing.T) {
	s := newTestServer(t, Options{AllowTrading: true, MaxUSD: 100})
	s.safety = safety.NewWith(stubChecker{rep: safety.Report{
		Level:    safety.LevelHigh,
		Honeypot: true,
		Flags:    []safety.Flag{{Key: "honeypot", Desc: "honeypot: selling is blocked", Severity: safety.LevelHigh}},
	}})
	// CAKE is a real non-stablecoin token on bsc, so the risk check applies.
	// Must refuse BEFORE any wallet unlock or RPC dial.
	_, _, err := s.swapExecute(context.Background(), nil, swapIn{USD: 5, From: "BNB", To: "CAKE"})
	if err == nil || !strings.Contains(err.Error(), "HIGH") {
		t.Fatalf("expected high-risk refusal, got %v", err)
	}

	// --allow-risky lifts the gate: execution proceeds past the risk check and
	// fails later on the empty test wallet instead.
	s.opts.AllowRisky = true
	_, _, err = s.swapExecute(context.Background(), nil, swapIn{USD: 5, From: "BNB", To: "CAKE"})
	if err != nil && strings.Contains(err.Error(), "HIGH") {
		t.Fatalf("risk gate must be lifted with AllowRisky, got %v", err)
	}
}

func TestSwapExecuteRefusesDisallowedChain(t *testing.T) {
	s := newTestServer(t, Options{AllowTrading: true, MaxUSD: 10, Chains: []string{"bsc_testnet"}})
	// Default chain is bsc — not in the allowlist. Must refuse before any
	// wallet unlock or RPC dial.
	_, _, err := s.swapExecute(context.Background(), nil, swapIn{USD: 5, From: "BNB", To: "USDT"})
	if err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("expected allowlist refusal, got %v", err)
	}
}
