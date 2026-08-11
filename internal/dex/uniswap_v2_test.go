package dex

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
)

const (
	wbnb = "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"
	usdt = "0x55d398326f99059fF775485246999027B3197955"
)

func provider(t *testing.T, rpc *testrpc.Server) *UniswapV2Provider {
	t.Helper()
	client, err := ethclient.Dial(rpc.URL)
	if err != nil {
		t.Fatalf("dial fake rpc: %v", err)
	}
	t.Cleanup(client.Close)
	return NewUniswapV2(client, "bsc")
}

func TestGetPriceQuotesViaRouter(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	// 1 in → 600 out (both sides 18 decimals on bsc).
	rpc.RateNum = big.NewInt(600)

	p := provider(t, rpc)
	got, err := p.GetPrice(context.Background(), wbnb, usdt, "2")
	if err != nil {
		t.Fatalf("GetPrice: %v", err)
	}
	if got != "1200" {
		t.Fatalf("GetPrice = %q, want 1200", got)
	}
	if rpc.Calls["d06ca61f"] != 1 {
		t.Fatalf("getAmountsOut calls = %d, want 1", rpc.Calls["d06ca61f"])
	}
}

func TestGetPriceRejectsBadAmount(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	p := provider(t, rpc)
	if _, err := p.GetPrice(context.Background(), wbnb, usdt, "not-a-number"); err == nil {
		t.Fatal("bad amount must error before any RPC call")
	}
	if rpc.Calls["d06ca61f"] != 0 {
		t.Fatal("no RPC call expected for a parse error")
	}
}

func TestGetPricePropagatesRevert(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.NoLiquidity = true
	p := provider(t, rpc)
	_, err := p.GetPrice(context.Background(), wbnb, usdt, "1")
	if err == nil || !strings.Contains(err.Error(), "revert") {
		t.Fatalf("want revert error, got %v", err)
	}
}
