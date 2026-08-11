package swap

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
)

const testPrivKey = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"

func TestFlowExecuteNativeBuy(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)

	f := fakeFlow(t, rpc)
	res := f.Execute(context.Background(), testPrivKey,
		bscToken(t, "BNB"), bscToken(t, "USDT"), "5", 0.5)
	if !res.Success {
		t.Fatalf("execute failed: %s", res.Err)
	}
	if res.TxHash == "" || res.GasUsed == "" {
		t.Fatalf("result incomplete: %+v", res)
	}
	if rpc.TxCount != 1 {
		t.Fatalf("broadcasts = %d, want 1 (native buy needs no approval)", rpc.TxCount)
	}
}

func TestFlowExecuteERC20ApprovesFirst(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.Allowance = big.NewInt(0) // force the approval leg

	f := fakeFlow(t, rpc)
	res := f.Execute(context.Background(), testPrivKey,
		bscToken(t, "USDT"), bscToken(t, "BNB"), "5", 0.5)
	if !res.Success {
		t.Fatalf("execute failed: %s", res.Err)
	}
	if rpc.TxCount != 2 {
		t.Fatalf("broadcasts = %d, want 2 (approve + swap)", rpc.TxCount)
	}
}

func TestFlowExecuteBadKey(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	f := fakeFlow(t, rpc)
	res := f.Execute(context.Background(), "zz-not-a-key",
		bscToken(t, "BNB"), bscToken(t, "USDT"), "5", 0.5)
	if res.Success || res.Err == "" {
		t.Fatalf("bad key must fail closed: %+v", res)
	}
}

func TestExecuteRawTxBroadcastsAndWaits(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)

	f := fakeFlow(t, rpc)
	hash, gas, err := f.ExecuteRawTx(context.Background(), testPrivKey, RawTx{
		To:    "0x10ED43C718714eb63d5aA57B78B54704E256024E",
		Value: "0", GasPrice: "1000000000", GasLimit: "100000", Data: "0x00", ChainID: 56,
	}, bscToken(t, "BNB"), big.NewInt(1))
	if err != nil {
		t.Fatalf("raw tx: %v", err)
	}
	if hash == "" || gas == "" || rpc.TxCount != 1 {
		t.Fatalf("hash=%q gas=%q txs=%d", hash, gas, rpc.TxCount)
	}
}

func TestExecuteRawTxRevertedReceipt(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.RevertTx = true

	f := fakeFlow(t, rpc)
	hash, err := f.ExecuteRawTxWrapper(t)
	if err == nil || !strings.Contains(err.Error(), "reverted") {
		t.Fatalf("want reverted error, got hash=%q err=%v", hash, err)
	}
}

// ExecuteRawTxWrapper keeps the revert test readable.
func (f *Flow) ExecuteRawTxWrapper(t *testing.T) (string, error) {
	t.Helper()
	hash, _, err := f.ExecuteRawTx(context.Background(), testPrivKey, RawTx{
		To:    "0x10ED43C718714eb63d5aA57B78B54704E256024E",
		Value: "0", GasPrice: "1000000000", GasLimit: "100000", Data: "0x00", ChainID: 56,
	}, TokenInfo{Symbol: "BNB", Address: NativeSentinel, Decimals: 18}, big.NewInt(1))
	return hash, err
}

func TestEnsureAllowanceSkipsWhenSufficient(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.Allowance = new(big.Int).SetUint64(1 << 60) // plenty pre-approved

	f := fakeFlow(t, rpc)
	_, _, err := f.ExecuteRawTx(context.Background(), testPrivKey, RawTx{
		To:    "0x10ED43C718714eb63d5aA57B78B54704E256024E",
		Value: "0", GasPrice: "1000000000", GasLimit: "100000", Data: "0x00", ChainID: 56,
	}, bscToken(t, "USDT"), big.NewInt(1000))
	if err != nil {
		t.Fatalf("raw tx with sufficient allowance: %v", err)
	}
	if rpc.TxCount != 1 {
		t.Fatalf("broadcasts = %d, want 1 (no approval tx when allowance covers)", rpc.TxCount)
	}
}
