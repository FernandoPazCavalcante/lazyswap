package wallet

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/tyler-smith/go-bip39"
)

func init() {
	// Standard scrypt costs 256 MB and ~1s per call; the format is identical
	// at light cost, which is all these tests check.
	keystoreScryptN = keystore.LightScryptN
	keystoreScryptP = keystore.LightScryptP
}

func TestGenerateMnemonicIsValidTwelveWords(t *testing.T) {
	m, err := GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Fields(m)); n != 12 {
		t.Fatalf("word count = %d, want 12", n)
	}
	if !bip39.IsMnemonicValid(m) {
		t.Fatalf("generated mnemonic is not valid BIP-39")
	}
}

// insertWithoutMnemonic stores a wallet row that has a key but no phrase —
// the legacy / hand-edited shape Reveal must refuse rather than print "".
func insertWithoutMnemonic(t *testing.T, svc *Service) {
	t.Helper()
	enc, err := svc.crypto.Encrypt(hardhatPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.dao.Insert(&Wallet{Address: hardhatAddress, PrivateKey: enc}); err != nil {
		t.Fatal(err)
	}
}

func TestReveal(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, svc *Service)
		addr    string
		want    string
		wantErr error
	}{
		{
			name:  "returns the stored phrase",
			setup: func(t *testing.T, svc *Service) { mustImport(t, svc) },
			addr:  hardhatAddress,
			want:  hardhatMnemonic,
		},
		{
			name:  "address match is case-insensitive",
			setup: func(t *testing.T, svc *Service) { mustImport(t, svc) },
			addr:  strings.ToLower(hardhatAddress),
			want:  hardhatMnemonic,
		},
		{
			name:    "unknown address",
			setup:   func(t *testing.T, svc *Service) { mustImport(t, svc) },
			addr:    "0x0000000000000000000000000000000000000001",
			wantErr: ErrWalletNotFound,
		},
		{
			name:    "wallet without a stored phrase",
			setup:   insertWithoutMnemonic,
			addr:    hardhatAddress,
			wantErr: ErrNoMnemonic,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newSvc(t)
			tc.setup(t, svc)
			got, err := svc.Reveal(tc.addr)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if got != "" {
					t.Fatalf("phrase leaked alongside error: %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("phrase = %q, want %q", got, tc.want)
			}
		})
	}
}

func mustImport(t *testing.T, svc *Service) {
	t.Helper()
	if _, err := svc.Import(hardhatMnemonic); err != nil {
		t.Fatal(err)
	}
}

// The export is only worth anything if another wallet can read it back to
// the same key — so decrypt it with go-ethereum, not with our own code.
func TestExportKeystoreRoundTrip(t *testing.T) {
	svc := newSvc(t)
	mustImport(t, svc)
	out := filepath.Join(t.TempDir(), "key.json")

	if err := svc.ExportKeystore(hardhatAddress, "correct horse", out); err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	key, err := keystore.DecryptKey(blob, "correct horse")
	if err != nil {
		t.Fatalf("decrypt exported keystore: %v", err)
	}
	if key.Address.Hex() != hardhatAddress {
		t.Fatalf("address = %s, want %s", key.Address.Hex(), hardhatAddress)
	}
	if _, err := keystore.DecryptKey(blob, "wrong passphrase"); err == nil {
		t.Fatal("keystore decrypted with the wrong passphrase")
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
}

func TestExportKeystoreRefusals(t *testing.T) {
	tests := []struct {
		name       string
		addr       string
		passphrase string
		preexist   bool
		wantErr    string
	}{
		{"short passphrase", hardhatAddress, "short", false, "at least 8"},
		{"unknown address", "0x0000000000000000000000000000000000000001", "correct horse", false, "no wallet"},
		{"existing file is not overwritten", hardhatAddress, "correct horse", true, "refusing to overwrite"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newSvc(t)
			mustImport(t, svc)
			out := filepath.Join(t.TempDir(), "key.json")
			if tc.preexist {
				if err := os.WriteFile(out, []byte("keep me"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := svc.ExportKeystore(tc.addr, tc.passphrase, out)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
			if tc.preexist {
				if b, _ := os.ReadFile(out); string(b) != "keep me" {
					t.Fatalf("existing file was modified: %q", b)
				}
				return
			}
			if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
				t.Fatalf("a refused export left a file behind")
			}
		})
	}
}

func TestExportKeystoreRejectsBadKey(t *testing.T) {
	if _, err := encryptKeystore("0xnothex", "correct horse"); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestWriteNewMissingDir(t *testing.T) {
	if err := writeNew(filepath.Join(t.TempDir(), "nope", "k.json"), []byte("x")); err == nil {
		t.Fatal("expected error for missing directory")
	}
}
