package swap

import (
	"context"
	"math/big"
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
