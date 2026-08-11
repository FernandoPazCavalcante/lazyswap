package cli

import (
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/crypto"
	"github.com/FernandoPazCavalcante/lazyswap/internal/paths"
	"github.com/FernandoPazCavalcante/lazyswap/internal/testrpc"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

const runTestPassword = "E2ePassword1"

// seedEncryptedWallet initialises salt/sentinel + one wallet, exactly like the
// TUI first run, inside a paths.Override'd temp dir.
func seedEncryptedWallet(t *testing.T) string {
	t.Helper()
	t.Setenv("LAZYSWAP_TEST", "1")
	t.Setenv("LAZYSWAP_PASSWORD", runTestPassword)
	dir := t.TempDir()
	paths.Override(dir)
	t.Cleanup(func() { paths.Override("") })

	dao, err := wallet.OpenAt(filepath.Join(dir, "wallets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dao.Close() }()

	key, salt, err := crypto.DeriveKey(runTestPassword, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := crypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	env, err := svc.Encrypt(crypto.SentinelPlain)
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.SetSalt(hex.EncodeToString(salt)); err != nil {
		t.Fatal(err)
	}
	if err := dao.SetSentinel(env); err != nil {
		t.Fatal(err)
	}
	w, err := wallet.NewService(dao, svc).Create()
	if err != nil {
		t.Fatal(err)
	}
	return w.Address
}

func TestRunSwapQuoteOnlyOverFakeRPC(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	t.Setenv("LAZYSWAP_RPC_URL", rpc.URL)
	seedEncryptedWallet(t)

	out, code := captureStdout(t, func() int {
		return runSwap([]string{"5", "BNB", "USDT", "--direct", "--quote-only", "--no-safety"})
	})
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, marker := range []string{"route     direct", "spend", "receive", "min recv", "fee"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("quote output missing %q:\n%s", marker, out)
		}
	}
}

func TestRunSwapExecuteOverFakeRPC(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	t.Setenv("LAZYSWAP_RPC_URL", rpc.URL)
	seedEncryptedWallet(t)

	out, code := captureStdout(t, func() int {
		return runSwap([]string{"5", "BNB", "USDT", "--direct", "--yes", "--no-safety"})
	})
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "✓ swapped — tx ") {
		t.Fatalf("missing tx confirmation:\n%s", out)
	}
	if rpc.TxCount != 1 {
		t.Fatalf("broadcasts = %d, want 1", rpc.TxCount)
	}
}

func TestRunSwapAPIModeFallsBackToDirect(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	t.Setenv("LAZYSWAP_RPC_URL", rpc.URL)
	t.Setenv("LAZYSWAP_API_URL", "http://127.0.0.1:1") // unreachable backend
	seedEncryptedWallet(t)

	// API chosen by SETTING (not --api): must warn on stderr + fall back.
	_, code := captureStdout(t, func() int { return runConfig([]string{"set-swap-mode", "api"}) })
	if code != 0 {
		t.Fatal("set-swap-mode failed")
	}
	out, code := captureStdout(t, func() int {
		return runSwap([]string{"5", "BNB", "USDT", "--quote-only", "--no-safety"})
	})
	if code != 0 {
		t.Fatalf("fallback should still quote, exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "route     direct") {
		t.Fatalf("expected direct fallback:\n%s", out)
	}

	// Explicit --api must fail loudly instead.
	_, code = captureStdout(t, func() int {
		return runSwap([]string{"5", "BNB", "USDT", "--api", "--quote-only", "--no-safety"})
	})
	if code != 1 {
		t.Fatalf("explicit --api with dead backend: exit %d, want 1", code)
	}
}

func TestRunSwapWrongPassword(t *testing.T) {
	rpc := testrpc.New()
	t.Cleanup(rpc.Close)
	t.Setenv("LAZYSWAP_RPC_URL", rpc.URL)
	seedEncryptedWallet(t)
	t.Setenv("LAZYSWAP_PASSWORD", "Wrong-password-9")

	_, code := captureStdout(t, func() int {
		return runSwap([]string{"5", "BNB", "USDT", "--quote-only"})
	})
	if code != 1 {
		t.Fatalf("wrong password exit %d, want 1", code)
	}
}
