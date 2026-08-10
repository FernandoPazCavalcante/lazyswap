package cli

import (
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
)

func TestTxURL(t *testing.T) {
	// "https://api.bscscan.com/api" → "https://bscscan.com/tx/<hash>"
	got := txURL(chain.Get("bsc"), "0xdeadbeef")
	want := "https://bscscan.com/tx/0xdeadbeef"
	if got != want {
		t.Fatalf("txURL = %q, want %q", got, want)
	}
}
