package balance

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/explorer"
	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
)

const walletAddr = "0xA502F4896E1b2B93080EabfFC60018b8D089b872"

func fakeService(t *testing.T, rpc *testrpc.Server) *Service {
	t.Helper()
	client, err := ethclient.Dial(rpc.URL)
	if err != nil {
		t.Fatalf("dial fake rpc: %v", err)
	}
	t.Cleanup(client.Close)
	return &Service{
		chainKey: "bsc",
		chain:    chain.Get("bsc"),
		client:   client,
		explorer: explorer.NewClient(),
	}
}

func TestFetchAllRejectsBadAddress(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	if _, err := fakeService(t, rpc).FetchAll(context.Background(), "not-an-address", ""); err == nil {
		t.Fatal("invalid address must error")
	}
}

func TestFetchAllReturnsNativeAndTokens(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	rpc.NativeBalance, _ = new(big.Int).SetString("2000000000000000000", 10) // 2 BNB
	rpc.TokenBalance, _ = new(big.Int).SetString("5000000000000000000", 10)  // 5 of each ERC-20

	bs, err := fakeService(t, rpc).FetchAll(context.Background(), walletAddr, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bs) < 2 {
		t.Fatalf("want native + tokens, got %d rows", len(bs))
	}
	if bs[0].Address != NativeAddress || bs[0].Symbol != "BNB" {
		t.Fatalf("first row must be native: %+v", bs[0])
	}
	if bs[0].BalanceRaw.Cmp(rpc.NativeBalance) != 0 {
		t.Fatalf("native raw = %v", bs[0].BalanceRaw)
	}
	// Every non-native row carries the faked token balance and a USD value
	// (1:1 fake pricing → non-empty).
	for _, b := range bs[1:] {
		if b.BalanceRaw.Sign() == 0 {
			t.Fatalf("zero-balance token should have been dropped: %+v", b)
		}
		if b.USDValue == "" {
			t.Fatalf("token %s missing USD value", b.Symbol)
		}
	}
}

func TestFetchAllZeroBalancesKeepOnlyNative(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	// All zeros: ERC-20 rows drop, native stays for the UI.
	bs, err := fakeService(t, rpc).FetchAll(context.Background(), walletAddr, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bs) != 1 || bs[0].Address != NativeAddress {
		t.Fatalf("want only native row, got %+v", bs)
	}
	if bs[0].Balance == "" {
		t.Fatal("native balance must render even at zero")
	}
}
