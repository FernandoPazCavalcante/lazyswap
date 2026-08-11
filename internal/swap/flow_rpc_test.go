package swap

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
)

// fakeFlow wires a Flow to the fake RPC on the bsc config.
func fakeFlow(t *testing.T, rpc *testrpc.Server) *Flow {
	t.Helper()
	client, err := ethclient.Dial(rpc.URL)
	if err != nil {
		t.Fatalf("dial fake rpc: %v", err)
	}
	t.Cleanup(client.Close)
	return &Flow{chainKey: "bsc", chain: chain.Get("bsc"), client: client}
}

func bscToken(t *testing.T, symbol string) TokenInfo {
	t.Helper()
	tok, err := ResolveToken(chain.Get("bsc"), symbol)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestConvertUsdToTokenAmount(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	// $600 buys 1 BNB: stable→token at rate 1/600.
	rpc.RateNum, rpc.RateDen = big.NewInt(1), big.NewInt(600)

	f := fakeFlow(t, rpc)
	conv, err := f.ConvertUsdToTokenAmount(context.Background(), bscToken(t, "BNB"), 600)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if conv.TokenAmount != "1" {
		t.Fatalf("token amount = %q, want 1", conv.TokenAmount)
	}
	if conv.PricePerToken != 600 {
		t.Fatalf("price per token = %v, want 600", conv.PricePerToken)
	}
}

func TestConvertUsdNoLiquidity(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.NoLiquidity = true

	f := fakeFlow(t, rpc)
	_, err := f.ConvertUsdToTokenAmount(context.Background(), bscToken(t, "BNB"), 5)
	if err == nil || !strings.Contains(err.Error(), "no liquidity path") {
		t.Fatalf("want no-liquidity error, got %v", err)
	}
}

func TestQuoteEndToEnd(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	// 1:1 pricing keeps the math auditable: $5 → 5 tokens gross.
	f := fakeFlow(t, rpc)

	q, err := f.Quote(context.Background(), bscToken(t, "BNB"), bscToken(t, "USDT"),
		"5", 0.5, "0xA502F4896E1b2B93080EabfFC60018b8D089b872")
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if q.USDAmountFormatted != "$5.00" {
		t.Fatalf("usd formatted = %q", q.USDAmountFormatted)
	}
	// Fee (0.15%) reduces the net input; estimated output mirrors net at 1:1.
	if q.FeePercent == 0 || q.FeeAmount == "0.000000" {
		t.Fatalf("fee missing: %+v", q)
	}
	net, _ := new(big.Float).SetString(q.NetFromTokenAmount)
	gross, _ := new(big.Float).SetString(q.FromTokenAmount)
	if net.Cmp(gross) >= 0 {
		t.Fatalf("net %v must be < gross %v", net, gross)
	}
	if q.MinOutput >= q.EstimatedOutput {
		t.Fatalf("min %q must be < estimated %q at 0.5%% slippage", q.MinOutput, q.EstimatedOutput)
	}
	// Native input never needs approval.
	if q.NeedsApproval {
		t.Fatal("native input must not need approval")
	}
}

func TestQuoteERC20NeedsApproval(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.Allowance = big.NewInt(0) // nothing approved yet

	f := fakeFlow(t, rpc)
	q, err := f.Quote(context.Background(), bscToken(t, "USDT"), bscToken(t, "BNB"),
		"5", 0.5, "0xA502F4896E1b2B93080EabfFC60018b8D089b872")
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if !q.NeedsApproval {
		t.Fatal("zero allowance must flag NeedsApproval")
	}
}

func TestQuoteRejectsBadUSD(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	f := fakeFlow(t, rpc)
	for _, usd := range []string{"0", "-1", "abc"} {
		if _, err := f.Quote(context.Background(), bscToken(t, "BNB"), bscToken(t, "USDT"),
			usd, 0.5, "0xA"); err == nil {
			t.Errorf("usd=%q must error", usd)
		}
	}
}

func TestExecuteRawTxRefusesWrongChain(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	f := fakeFlow(t, rpc)

	_, _, err := f.ExecuteRawTx(context.Background(),
		"4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318",
		RawTx{To: "0x1", ChainID: 1}, // ethereum tx against a bsc flow
		bscToken(t, "BNB"), big.NewInt(1))
	if err == nil || !strings.Contains(err.Error(), "refusing tx for chainId") {
		t.Fatalf("want chain-id refusal, got %v", err)
	}
}
