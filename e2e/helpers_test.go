//go:build e2e

// Package e2e holds end-to-end tests, excluded from the default build via the
// e2e tag: `go test -tags e2e ./e2e/...` (or `make e2e`).
package e2e

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FernandoPazCavalcante/lazyswap/internal/crypto"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

// e2ePassword unlocks every throwaway wallet DB these tests create. It must
// satisfy the login policy: 8+ chars with upper, lower and digits.
const e2ePassword = "E2ePassword1"

// binPath is the lazyswap binary built once in TestMain.
var binPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "lazyswap-e2e-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: mktemp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	binPath = filepath.Join(tmp, "lazyswap")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = ".." // module root
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build failed: %v\n%s", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// seedWallet initialises encryption (salt + sentinel) and creates one wallet
// in a fresh data dir, exactly as the TUI login screen would. It returns the
// data dir and the wallet address. Env for the CLI is set on t.
func seedWallet(t *testing.T) (dataDir, address string) {
	t.Helper()
	dataDir = t.TempDir()
	t.Setenv("LAZYSWAP_DATA_DIR", dataDir)
	t.Setenv("LAZYSWAP_TEST", "1")
	t.Setenv("LAZYSWAP_PASSWORD", e2ePassword)

	dao, err := wallet.OpenAt(filepath.Join(dataDir, "wallets.db"))
	if err != nil {
		t.Fatalf("open dao: %v", err)
	}
	defer func() { _ = dao.Close() }()

	key, salt, err := crypto.DeriveKey(e2ePassword, nil)
	if err != nil {
		t.Fatalf("derive key: %v", err)
	}
	svc, err := crypto.New(key)
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	env, err := svc.Encrypt(crypto.SentinelPlain)
	if err != nil {
		t.Fatalf("sentinel: %v", err)
	}
	if err := dao.SetSalt(hex.EncodeToString(salt)); err != nil {
		t.Fatalf("salt: %v", err)
	}
	if err := dao.SetSentinel(env); err != nil {
		t.Fatalf("sentinel store: %v", err)
	}
	w, err := wallet.NewService(dao, svc).Create()
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	return dataDir, w.Address
}

// runCLI executes the built binary with the ambient (t.Setenv) environment and
// returns combined output + exit code.
func runCLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, out)
	}
	return string(out), code
}

// retry runs fn up to attempts times, passing when any run succeeds. Public
// testnet RPCs flake; genuinely broken code fails all attempts.
func retry(t *testing.T, attempts int, fn func() error) {
	t.Helper()
	var last error
	for i := 0; i < attempts; i++ {
		if last = fn(); last == nil {
			return
		}
		t.Logf("attempt %d/%d failed: %v", i+1, attempts, last)
	}
	t.Fatalf("all %d attempts failed: %v", attempts, last)
}

// mustContain asserts every marker appears in out.
func mustContain(t *testing.T, out string, markers ...string) {
	t.Helper()
	for _, m := range markers {
		if !strings.Contains(out, m) {
			t.Fatalf("output missing %q:\n%s", m, out)
		}
	}
}
