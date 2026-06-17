package settings

import (
	"path/filepath"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap-tui/internal/wallet"
)

func newDAO(t *testing.T) *wallet.DAO {
	t.Helper()
	dao, err := wallet.OpenAt(filepath.Join(t.TempDir(), "wallets.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = dao.Close() })
	return dao
}

func TestLoadDefaults(t *testing.T) {
	got, err := Load(newDAO(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Defaults()
	if got != want {
		t.Fatalf("fresh Load = %+v, want defaults %+v", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	dao := newDAO(t)
	if err := SetSlippage(dao, 1.25); err != nil {
		t.Fatal(err)
	}
	if err := SetChain(dao, "ethereum"); err != nil {
		t.Fatal(err)
	}
	if err := SetDefaultWallet(dao, "0xABC"); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dao)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Slippage != 1.25 || got.ChainKey != "ethereum" || got.DefaultWallet != "0xABC" {
		t.Fatalf("round-trip = %+v", got)
	}
}

func TestValidation(t *testing.T) {
	dao := newDAO(t)
	if err := SetSlippage(dao, 150); err == nil {
		t.Fatal("expected slippage > 100 rejected")
	}
	if err := SetChain(dao, "dogecoin"); err == nil {
		t.Fatal("expected unknown chain rejected")
	}
	// An out-of-range value persisted directly is ignored by Load (falls back).
	_ = dao.SetConfig("setting_slippage", "999")
	if got, _ := Load(dao); got.Slippage != DefaultSlippage {
		t.Fatalf("invalid stored slippage should fall back, got %v", got.Slippage)
	}
}
