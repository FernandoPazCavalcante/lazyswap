package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/tyler-smith/go-bip39"
)

func fakeTerminal(t *testing.T, isTTY bool) {
	t.Helper()
	old := stdoutIsTerminal
	stdoutIsTerminal = func() bool { return isTTY }
	t.Cleanup(func() { stdoutIsTerminal = old })
}

func TestWalletsListShowsSeededAddress(t *testing.T) {
	addr := seedEncryptedWallet(t)
	out, code := captureStdout(t, func() int { return runWallets(nil) })
	if code != 0 || !strings.Contains(out, addr) {
		t.Fatalf("exit %d, output %q, want address %s", code, out, addr)
	}
}

func TestWalletsRevealPrintsPhrase(t *testing.T) {
	tests := []struct {
		name  string
		isTTY bool
		args  []string
	}{
		{"terminal", true, []string{"reveal"}},
		{"pipe with --allow-pipe", false, []string{"reveal", "--allow-pipe"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr := seedEncryptedWallet(t)
			fakeTerminal(t, tc.isTTY)
			out, code := captureStdout(t, func() int {
				return runWallets(append(tc.args, "--address", addr))
			})
			if code != 0 {
				t.Fatalf("exit %d", code)
			}
			phrase := strings.TrimSpace(out)
			if len(strings.Fields(phrase)) != 12 || !bip39.IsMnemonicValid(phrase) {
				t.Fatalf("stdout is not a valid 12-word phrase: %q", out)
			}
		})
	}
}

func TestWalletsRevealRefusals(t *testing.T) {
	tests := []struct {
		name     string
		isTTY    bool
		password string
		args     []string
	}{
		{"pipe without --allow-pipe", false, runTestPassword, []string{"reveal"}},
		{"wrong password", true, "Wrong-password-9", []string{"reveal"}},
		{"unknown wallet", true, runTestPassword, []string{"reveal", "--address", "0x0000000000000000000000000000000000000001"}},
		{"stray argument", true, runTestPassword, []string{"reveal", "extra"}},
		{"unknown subcommand", true, runTestPassword, []string{"nope"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seedEncryptedWallet(t)
			t.Setenv("LAZYSWAP_PASSWORD", tc.password)
			fakeTerminal(t, tc.isTTY)
			out, code := captureStdout(t, func() int { return runWallets(tc.args) })
			if code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			if out != "" {
				t.Fatalf("a refused reveal wrote to stdout: %q", out)
			}
		})
	}
}

func TestWalletsExportWritesReadableKeystore(t *testing.T) {
	addr := seedEncryptedWallet(t)
	t.Setenv(keystorePassphraseEnv, "correct horse battery")
	out := filepath.Join(t.TempDir(), "key.json")

	stdout, code := captureStdout(t, func() int {
		return runWallets([]string{"export", "--keystore", out})
	})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stdout)
	}
	blob, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	key, err := keystore.DecryptKey(blob, "correct horse battery")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !strings.EqualFold(key.Address.Hex(), addr) {
		t.Fatalf("keystore address %s, want %s", key.Address.Hex(), addr)
	}
	if strings.Contains(stdout, "correct horse") {
		t.Fatal("passphrase echoed to stdout")
	}
}

func TestWalletsExportRefusals(t *testing.T) {
	tests := []struct {
		name       string
		passphrase string
		args       func(dir string) []string
	}{
		{"missing --keystore", "correct horse battery", func(string) []string { return []string{"export"} }},
		{"no passphrase and no terminal", "", func(d string) []string {
			return []string{"export", "--keystore", filepath.Join(d, "k.json")}
		}},
		{"short passphrase", "short", func(d string) []string {
			return []string{"export", "--keystore", filepath.Join(d, "k.json")}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seedEncryptedWallet(t)
			t.Setenv(keystorePassphraseEnv, tc.passphrase)
			dir := t.TempDir()
			_, code := captureStdout(t, func() int { return runWallets(tc.args(dir)) })
			if code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			if _, err := os.Stat(filepath.Join(dir, "k.json")); !os.IsNotExist(err) {
				t.Fatal("a refused export left a file behind")
			}
		})
	}
}
