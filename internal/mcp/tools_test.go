package mcp

import (
	"context"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/settings"
	"github.com/FernandoPazCavalcante/lazyswap/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

func TestListChains(t *testing.T) {
	s := newTestServer(t, Options{})
	_, out, err := s.listChains(context.Background(), nil, listChainsIn{})
	if err != nil {
		t.Fatalf("listChains: %v", err)
	}
	if len(out.Chains) != len(chain.OrderedKeys) {
		t.Fatalf("chains = %d, want %d", len(out.Chains), len(chain.OrderedKeys))
	}
	defaults := 0
	for i, c := range out.Chains {
		if c.Key != chain.OrderedKeys[i] {
			t.Fatalf("order broken at %d: %q", i, c.Key)
		}
		if c.Default {
			defaults++
			if c.Key != "bsc" {
				t.Fatalf("default chain = %q, want bsc", c.Key)
			}
		}
		if len(c.Tokens) == 0 {
			t.Fatalf("%s: no tokens listed", c.Key)
		}
	}
	if defaults != 1 {
		t.Fatalf("exactly one default expected, got %d", defaults)
	}
}

func TestGetWallets(t *testing.T) {
	s := newTestServer(t, Options{})

	_, out, err := s.getWallets(context.Background(), nil, getWalletsIn{})
	if err != nil || len(out.Wallets) != 0 {
		t.Fatalf("empty DB: %+v err=%v", out, err)
	}

	if err := s.dao.Insert(&wallet.Wallet{ID: "w1", Address: "0xAAA", PrivateKey: "enc"}); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetDefaultWallet(s.dao, "0xAAA"); err != nil {
		t.Fatal(err)
	}
	_, out, err = s.getWallets(context.Background(), nil, getWalletsIn{})
	if err != nil || len(out.Wallets) != 1 {
		t.Fatalf("seeded: %+v err=%v", out, err)
	}
	if out.Wallets[0].Address != "0xAAA" || !out.Wallets[0].Default {
		t.Fatalf("wallet row wrong: %+v", out.Wallets[0])
	}
}

func TestGetSetSettings(t *testing.T) {
	s := newTestServer(t, Options{})

	_, st, err := s.getSettings(context.Background(), nil, getSettingsIn{})
	if err != nil || st.Chain != "bsc" {
		t.Fatalf("defaults: %+v err=%v", st, err)
	}

	// Valid updates round-trip.
	slip := 1.25
	_, st, err = s.setSettings(context.Background(), nil, setSettingsIn{Chain: "ethereum", Slippage: &slip})
	if err != nil || st.Chain != "ethereum" || st.Slippage != 1.25 {
		t.Fatalf("update: %+v err=%v", st, err)
	}

	// Invalid chain refused.
	if _, _, err := s.setSettings(context.Background(), nil, setSettingsIn{Chain: "nope"}); err == nil {
		t.Fatal("unknown chain must error")
	}
	// Unknown default wallet refused.
	if _, _, err := s.setSettings(context.Background(), nil, setSettingsIn{DefaultWallet: "0xNope"}); err == nil {
		t.Fatal("unknown wallet must error")
	}
}

func TestSwapQuoteOverFakeRPC(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)

	s := newTestServer(t, Options{})
	if err := s.dao.Insert(&wallet.Wallet{ID: "w1", Address: "0xA502F4896E1b2B93080EabfFC60018b8D089b872", PrivateKey: "enc"}); err != nil {
		t.Fatal(err)
	}
	flow, err := swap.NewFlowAt("bsc", rpc.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(flow.Close)
	s.flows["bsc"] = flow

	_, out, err := s.swapQuote(context.Background(), nil, swapIn{USD: 5, From: "BNB", To: "USDT"})
	if err != nil {
		t.Fatalf("swapQuote: %v", err)
	}
	if out.USDAmountFormatted != "$5.00" || out.Mode != "direct" {
		t.Fatalf("quote wrong: %+v", out.FlowQuote)
	}
	// Buying a stablecoin skips the risk check entirely.
	if out.Safety != nil {
		t.Fatalf("safety must be nil for stablecoin buys, got %+v", out.Safety)
	}
}

func TestGetBalancesOverFakeRPC(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	t.Setenv("LAZYSWAP_RPC_URL", rpc.URL)

	s := newTestServer(t, Options{})
	if err := s.dao.Insert(&wallet.Wallet{ID: "w1", Address: "0xA502F4896E1b2B93080EabfFC60018b8D089b872", PrivateKey: "enc"}); err != nil {
		t.Fatal(err)
	}
	_, out, err := s.getBalances(context.Background(), nil, getBalancesIn{})
	if err != nil {
		t.Fatalf("getBalances: %v", err)
	}
	if out.Chain != "bsc" || len(out.Balances) == 0 {
		t.Fatalf("balances wrong: %+v", out)
	}
	if out.Balances[0].Symbol != "BNB" {
		t.Fatalf("first row must be native: %+v", out.Balances[0])
	}
}

func TestGetPassStatusOverFakeRPC(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	t.Setenv("LAZYSWAP_RPC_URL", rpc.URL)

	s := newTestServer(t, Options{})
	if err := s.dao.Insert(&wallet.Wallet{ID: "w1", Address: "0xA502F4896E1b2B93080EabfFC60018b8D089b872", PrivateKey: "enc"}); err != nil {
		t.Fatal(err)
	}
	// bsc has no pass contract → inert, Deployed=false.
	_, out, err := s.getPassStatus(context.Background(), nil, passIn{Chain: "bsc"})
	if err != nil || out.Deployed {
		t.Fatalf("bsc must be inert: %+v err=%v", out, err)
	}
	// bsc_testnet has the contract; fake RPC reports zero balance.
	_, out, err = s.getPassStatus(context.Background(), nil, passIn{Chain: "bsc_testnet"})
	if err != nil {
		t.Fatalf("testnet status: %v", err)
	}
	if !out.Deployed || out.HasValidPass {
		t.Fatalf("expected deployed + no pass: %+v", out)
	}
}

func TestRiskReportSkipsSafeTokens(t *testing.T) {
	s := newTestServer(t, Options{})
	c := chain.Get("bsc")

	native := swap.TokenInfo{Symbol: c.NativeSymbol, Address: swap.NativeSentinel}
	if rep := s.riskReport(context.Background(), "bsc", native); rep != nil {
		t.Fatalf("native token must skip the risk check, got %+v", rep)
	}
	meme := swap.TokenInfo{Symbol: "MEME", Address: "0xAbCdEF0000000000000000000000000000000001"}
	if rep := s.riskReport(context.Background(), "bsc", meme); rep == nil {
		t.Fatal("arbitrary token must be checked (stub checker returns a report)")
	}
}
