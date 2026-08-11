package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := fn()
	os.Stdout = old
	_ = w.Close()
	out, _ := io.ReadAll(r)
	return string(out), code
}

func TestRunConfigShowDefaults(t *testing.T) {
	tempDAO(t)
	out, code := captureStdout(t, func() int { return runConfig([]string{"show"}) })
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, marker := range []string{"chain", "bsc", "slippage", "0.5%", "swap mode", "direct (default)"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("show output missing %q:\n%s", marker, out)
		}
	}
}

func TestRunConfigSetters(t *testing.T) {
	tempDAO(t)

	cases := []struct {
		name string
		args []string
		code int
	}{
		{"set-chain valid", []string{"set-chain", "ethereum"}, 0},
		{"set-chain invalid", []string{"set-chain", "nope"}, 1},
		{"set-slippage valid", []string{"set-slippage", "1.5"}, 0},
		{"set-slippage NaN", []string{"set-slippage", "abc"}, 1},
		{"set-slippage out of range", []string{"set-slippage", "150"}, 1},
		{"set-swap-mode api", []string{"set-swap-mode", "api"}, 0},
		{"set-swap-mode bogus", []string{"set-swap-mode", "bogus"}, 1},
		{"set-wallet unknown", []string{"set-wallet", "0xNoSuchWallet"}, 1},
		{"unknown subcommand", []string{"frobnicate"}, 1},
		{"missing argument", []string{"set-chain"}, 1},
	}
	for _, c := range cases {
		_, code := captureStdout(t, func() int { return runConfig(c.args) })
		if code != c.code {
			t.Errorf("%s: exit %d, want %d", c.name, code, c.code)
		}
	}

	// The setters actually persisted.
	out, _ := captureStdout(t, func() int { return runConfig([]string{"show"}) })
	if !strings.Contains(out, "ethereum") || !strings.Contains(out, "1.5%") || !strings.Contains(out, "api") {
		t.Fatalf("persisted settings not reflected:\n%s", out)
	}
}

func TestRunWallets(t *testing.T) {
	dao := tempDAO(t)

	out, code := captureStdout(t, func() int { return runWallets(nil) })
	if code != 0 || !strings.Contains(out, "no wallets yet") {
		t.Fatalf("empty list: exit %d out %q", code, out)
	}

	// Addresses are stored plaintext — a bare row insert is enough to list.
	if err := dao.Insert(&wallet.Wallet{ID: "w1", Address: "0xAAA", PrivateKey: "enc"}); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() int { return runConfig([]string{"set-wallet", "0xAAA"}) }); false {
		_ = err
	}
	out, code = captureStdout(t, func() int { return runWallets(nil) })
	if code != 0 || !strings.Contains(out, "* 0xAAA") {
		t.Fatalf("default marker missing: exit %d out %q", code, out)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	if code := Run([]string{"frobnicate"}); code != 2 {
		t.Fatalf("unknown command exit %d, want 2", code)
	}
	if code := Run(nil); code != 2 {
		t.Fatalf("no args exit %d, want 2", code)
	}
	out, code := captureStdout(t, func() int { return Run([]string{"help"}) })
	if code != 0 || !strings.Contains(out, "lazyswap swap") {
		t.Fatalf("help: exit %d", code)
	}
	out, code = captureStdout(t, func() int { return Run([]string{"version"}) })
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("version: exit %d out %q", code, out)
	}
}

func TestRunMcpBadFlags(t *testing.T) {
	tempDAO(t)
	if code := runMcp([]string{"--nope"}); code != 1 {
		t.Fatalf("bad flag exit %d, want 1", code)
	}
	// --allow-trading without --max-usd surfaces mcp.Run's validation error.
	if code := runMcp([]string{"--allow-trading"}); code != 1 {
		t.Fatalf("missing cap exit %d, want 1", code)
	}
}
