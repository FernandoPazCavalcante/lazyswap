package swap

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
)

const (
	usdtBsc = "0x55d398326f99059fF775485246999027B3197955"
	wbnbBsc = "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"
)

func fakeExecutor(t *testing.T, rpc *testrpc.Server) *Executor {
	t.Helper()
	client, err := ethclient.Dial(rpc.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	e, err := NewExecutor(client, testPrivKey, "bsc")
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestNewExecutorDerivesAddress(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	e := fakeExecutor(t, rpc)
	if e.Address().Hex() != "0x2c7536E3605D9C16a7a3D7b1898e529396a65c23" {
		t.Fatalf("derived address = %s", e.Address().Hex())
	}
	if _, err := NewExecutor(nil, "zz", "bsc"); err == nil {
		t.Fatal("bad key must error")
	}
}

func TestCheckAllowanceFormatsUnits(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.Allowance, _ = new(big.Int).SetString("1500000000000000000", 10) // 1.5 (18 dec)

	e := fakeExecutor(t, rpc)
	got, err := e.CheckAllowance(context.Background(), usdtBsc, "0xspender")
	if err != nil {
		t.Fatalf("allowance: %v", err)
	}
	if got != "1.5" {
		t.Fatalf("allowance = %q, want 1.5", got)
	}
}

func TestApproveTokenBroadcasts(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	e := fakeExecutor(t, rpc)
	hash, err := e.ApproveToken(context.Background(), usdtBsc, "5")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if hash == "" || rpc.TxCount != 1 {
		t.Fatalf("hash=%q txs=%d", hash, rpc.TxCount)
	}
}

func TestExecuteFullSwapRoutesAllShapes(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	// Pre-approved far above any human-readable amount: no approve leg.
	rpc.Allowance, _ = new(big.Int).SetString("1000000000000000000000000", 10)

	e := fakeExecutor(t, rpc)

	// native → token (swapExactETHForTokens)
	res, err := e.ExecuteFullSwap(context.Background(), NativeSentinel, usdtBsc, "0.5", "290", 0.5)
	if err != nil {
		t.Fatalf("buy with native: %v", err)
	}
	if res.FromToken != NativeSentinel || res.TxHash == "" {
		t.Fatalf("buy result: %+v", res)
	}

	// token → native (swapExactTokensForETH)
	res, err = e.ExecuteFullSwap(context.Background(), usdtBsc, NativeSentinel, "5", "0.008", 0.5)
	if err != nil {
		t.Fatalf("sell for native: %v", err)
	}
	if res.ToToken != NativeSentinel {
		t.Fatalf("sell result: %+v", res)
	}

	// token → token (swapExactTokensForTokens)
	res, err = e.ExecuteFullSwap(context.Background(), usdtBsc, wbnbBsc, "5", "0.008", 0.5)
	if err != nil {
		t.Fatalf("token swap: %v", err)
	}
	if res.TxHash == "" {
		t.Fatalf("token swap result: %+v", res)
	}
}

func TestExecuteSwapRefusesInsufficientAllowance(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.Allowance = big.NewInt(0)

	e := fakeExecutor(t, rpc)
	_, err := e.ExecuteSwap(context.Background(), usdtBsc, wbnbBsc, "5", "0.008", 0.5)
	if err == nil || !strings.Contains(err.Error(), "insufficient allowance") {
		t.Fatalf("want allowance refusal, got %v", err)
	}
}
