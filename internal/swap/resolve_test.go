package swap

import (
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

func TestResolveToken(t *testing.T) {
	c := chain.Get("bsc") // native BNB, has USDT

	// Native symbol → sentinel address, case-insensitive.
	got, err := ResolveToken(c, "bnb")
	if err != nil {
		t.Fatalf("native: %v", err)
	}
	if got.Address != NativeSentinel || got.Symbol != c.NativeSymbol {
		t.Fatalf("native resolved wrong: %+v", got)
	}

	// ERC-20 symbol → its configured address.
	usdt, err := ResolveToken(c, "USDT")
	if err != nil {
		t.Fatalf("USDT: %v", err)
	}
	if usdt.Address != c.Tokens["USDT"].Address || usdt.Decimals != c.Tokens["USDT"].Decimals {
		t.Fatalf("USDT resolved wrong: %+v", usdt)
	}

	// Unknown symbol → error.
	if _, err := ResolveToken(c, "NOPECOIN"); err == nil {
		t.Fatal("expected error for unknown token")
	}
}
