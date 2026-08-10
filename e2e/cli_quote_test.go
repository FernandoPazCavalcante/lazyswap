//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
)

// TestCLIQuoteOnly is the PR-gate e2e: create a wallet, list it, read config,
// and fetch a real quote from the public bsc_testnet RPC — no funds needed,
// nothing signed, nothing broadcast.
func TestCLIQuoteOnly(t *testing.T) {
	_, addr := seedWallet(t)

	out, code := runCLI(t, "wallets")
	if code != 0 {
		t.Fatalf("wallets exited %d:\n%s", code, out)
	}
	mustContain(t, out, addr)

	out, code = runCLI(t, "config", "show")
	if code != 0 {
		t.Fatalf("config show exited %d:\n%s", code, out)
	}
	mustContain(t, out, "chain", "slippage", "swap mode")

	// Quote against the live BSC mainnet router — read-only, no funds, nothing
	// signed. (bsc_testnet is not usable here: its configured router has no
	// stablecoin liquidity path, so USD pricing fails there.) Retried: public
	// RPC flake is the dominant failure mode, not our code.
	retry(t, 3, func() error {
		out, code := runCLI(t, "swap", "0.50", "BNB", "USDT",
			"--chain", "bsc", "--direct", "--quote-only", "--no-safety")
		if code != 0 {
			return fmt.Errorf("exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "receive") || !strings.Contains(out, "min recv") {
			return fmt.Errorf("quote output incomplete:\n%s", out)
		}
		return nil
	})
}

// TestCLIHelpAndVersion pins the top-level command surface.
func TestCLIHelpAndVersion(t *testing.T) {
	out, code := runCLI(t, "help")
	if code != 0 {
		t.Fatalf("help exited %d", code)
	}
	mustContain(t, out, "lazyswap swap", "--quote-only", "lazyswap mcp")

	out, code = runCLI(t, "version")
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("version exited %d with output %q", code, out)
	}
}
