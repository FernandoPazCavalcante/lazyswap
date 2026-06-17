package cli

import (
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap-tui/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap-tui/internal/swap"
	"github.com/FernandoPazCavalcante/lazyswap-tui/internal/wallet"
)

func TestResolveToken(t *testing.T) {
	c := chain.Get("bsc") // native BNB, has USDT

	// Native symbol → sentinel address, case-insensitive.
	got, err := resolveToken(c, "bnb")
	if err != nil {
		t.Fatalf("native: %v", err)
	}
	if got.Address != swap.NativeSentinel || got.Symbol != c.NativeSymbol {
		t.Fatalf("native resolved wrong: %+v", got)
	}

	// ERC-20 symbol → its configured address.
	usdt, err := resolveToken(c, "USDT")
	if err != nil {
		t.Fatalf("USDT: %v", err)
	}
	if usdt.Address != c.Tokens["USDT"].Address || usdt.Decimals != c.Tokens["USDT"].Decimals {
		t.Fatalf("USDT resolved wrong: %+v", usdt)
	}

	// Unknown symbol → error.
	if _, err := resolveToken(c, "NOPECOIN"); err == nil {
		t.Fatal("expected error for unknown token")
	}
}

func TestPickWallet(t *testing.T) {
	a := wallet.Wallet{Address: "0xAAA"}
	b := wallet.Wallet{Address: "0xBBB"}

	// No wallets → error.
	if _, err := pickWallet(nil, "", ""); err == nil {
		t.Fatal("expected error with no wallets")
	}

	// Single wallet, no flag/default → that wallet.
	if w, err := pickWallet([]wallet.Wallet{a}, "", ""); err != nil || w.Address != "0xAAA" {
		t.Fatalf("single: got %+v err %v", w, err)
	}

	// Multiple wallets, no flag/default → ambiguous error.
	if _, err := pickWallet([]wallet.Wallet{a, b}, "", ""); err == nil {
		t.Fatal("expected ambiguity error")
	}

	// Flag wins (case-insensitive).
	if w, err := pickWallet([]wallet.Wallet{a, b}, "0xbbb", "0xAAA"); err != nil || w.Address != "0xBBB" {
		t.Fatalf("flag: got %+v err %v", w, err)
	}

	// Default used when no flag.
	if w, err := pickWallet([]wallet.Wallet{a, b}, "", "0xBBB"); err != nil || w.Address != "0xBBB" {
		t.Fatalf("default: got %+v err %v", w, err)
	}

	// Unknown flag address → error.
	if _, err := pickWallet([]wallet.Wallet{a, b}, "0xZZZ", ""); err == nil {
		t.Fatal("expected error for unknown --wallet")
	}
}

func TestTxURL(t *testing.T) {
	// "https://api.bscscan.com/api" → "https://bscscan.com/tx/<hash>"
	got := txURL(chain.Get("bsc"), "0xdeadbeef")
	want := "https://bscscan.com/tx/0xdeadbeef"
	if got != want {
		t.Fatalf("txURL = %q, want %q", got, want)
	}
}
