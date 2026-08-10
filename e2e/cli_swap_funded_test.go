//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// TestFundedSwap executes a real tiny swap on bsc_testnet. Nightly-only: it
// needs a funded wallet, imported from the LAZYSWAP_E2E_MNEMONIC secret, and
// spends real testnet gas. Without the env it skips.
func TestFundedSwap(t *testing.T) {
	mnemonic := os.Getenv("LAZYSWAP_E2E_MNEMONIC")
	if mnemonic == "" {
		t.Skip("LAZYSWAP_E2E_MNEMONIC not set — funded e2e runs nightly only")
	}

	dataDir, _ := seedWallet(t)

	// Import the funded wallet next to the throwaway one and make it default.
	dao, err := wallet.OpenAt(filepath.Join(dataDir, "wallets.db"))
	if err != nil {
		t.Fatalf("open dao: %v", err)
	}
	svc, err := wallet.Unlock(dao, e2ePassword)
	if err != nil {
		t.Fatalf("unlock: %v", err)
	}
	funded, err := wallet.NewService(dao, svc).Import(mnemonic)
	if err != nil {
		t.Fatalf("import funded wallet: %v", err)
	}
	_ = dao.Close()

	retry(t, 2, func() error {
		out, code := runCLI(t, "swap", "0.05", "tBNB", "USDT",
			"--chain", "bsc_testnet", "--direct", "--yes", "--no-safety",
			"--wallet", funded.Address)
		if code != 0 {
			return fmt.Errorf("swap exited %d:\n%s", code, out)
		}
		if !strings.Contains(out, "✓ swapped — tx ") {
			return fmt.Errorf("no tx confirmation in output:\n%s", out)
		}
		return nil
	})
}
