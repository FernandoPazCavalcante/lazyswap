package wallet

import "testing"

func TestPick(t *testing.T) {
	a := Wallet{Address: "0xAAA"}
	b := Wallet{Address: "0xBBB"}

	// No wallets → error.
	if _, err := Pick(nil, "", ""); err == nil {
		t.Fatal("expected error with no wallets")
	}

	// Single wallet, no explicit/default → that wallet.
	if w, err := Pick([]Wallet{a}, "", ""); err != nil || w.Address != "0xAAA" {
		t.Fatalf("single: got %+v err %v", w, err)
	}

	// Multiple wallets, no explicit/default → ambiguous error.
	if _, err := Pick([]Wallet{a, b}, "", ""); err == nil {
		t.Fatal("expected ambiguity error")
	}

	// Explicit address wins (case-insensitive).
	if w, err := Pick([]Wallet{a, b}, "0xbbb", "0xAAA"); err != nil || w.Address != "0xBBB" {
		t.Fatalf("explicit: got %+v err %v", w, err)
	}

	// Default used when no explicit address.
	if w, err := Pick([]Wallet{a, b}, "", "0xBBB"); err != nil || w.Address != "0xBBB" {
		t.Fatalf("default: got %+v err %v", w, err)
	}

	// Unknown explicit address → error.
	if _, err := Pick([]Wallet{a, b}, "0xZZZ", ""); err == nil {
		t.Fatal("expected error for unknown --wallet")
	}
}
