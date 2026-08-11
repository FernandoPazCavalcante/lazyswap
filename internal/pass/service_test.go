package pass

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
)

func TestNewRefusesChainsWithoutPass(t *testing.T) {
	// The pass contract only exists on bsc_testnet — everywhere else the
	// feature must be inert, not half-configured.
	for _, key := range []string{"bsc", "ethereum", "sepolia"} {
		if _, err := New(key); err != ErrNoPass {
			t.Errorf("%s: err = %v, want ErrNoPass", key, err)
		}
	}
}

func TestStatusNoPassHeld(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	client, err := ethclient.Dial(rpc.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)

	s := &Service{
		chain:   chain.Get("bsc_testnet"),
		client:  client,
		address: common.HexToAddress("0x0000000000000000000000000000000000000042"),
	}
	st, err := s.Status(context.Background(), "0xA502F4896E1b2B93080EabfFC60018b8D089b872")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.HasValidPass || !st.ExpiresAt.IsZero() {
		t.Fatalf("zero balance must mean no valid pass: %+v", st)
	}
}

func TestNativeSymbol(t *testing.T) {
	s := &Service{chain: chain.Get("bsc_testnet")}
	if s.NativeSymbol() != "tBNB" {
		t.Fatalf("native symbol = %q", s.NativeSymbol())
	}
}
