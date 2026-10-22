package wallet

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

// ErrNoMnemonic means the wallet row has no stored recovery phrase. Every
// wallet created or imported by lazyswap has one; this guards hand-edited or
// legacy rows so a reveal never prints an empty "phrase".
var ErrNoMnemonic = errors.New("this wallet has no stored recovery phrase")

// ErrWalletNotFound means no wallet matches the requested address.
var ErrWalletNotFound = errors.New("no wallet with that address")

// MinKeystorePassphrase is the shortest passphrase ExportKeystore accepts. A
// keystore file is meant to leave this machine, so it can be brute-forced
// offline; scrypt slows that down but cannot save a trivial passphrase.
const MinKeystorePassphrase = 8

// keystoreScryptN / keystoreScryptP are the scrypt cost parameters for exported
// keystores. Standard (256 MB) by default — the file outlives this process and
// may be attacked offline. Tests lower them to keep the suite fast.
var (
	keystoreScryptN = keystore.StandardScryptN
	keystoreScryptP = keystore.StandardScryptP
)

// GenerateMnemonic returns a fresh 12-word BIP-39 phrase without persisting
// anything. The TUI uses it to show the phrase and confirm the user wrote it
// down before the wallet is saved (see Import).
func GenerateMnemonic() (string, error) { return newMnemonic() }

// find returns the decrypted wallet at address (case-insensitive).
func (s *Service) find(address string) (Wallet, error) {
	ws, err := s.FetchAll()
	if err != nil {
		return Wallet{}, err
	}
	for _, w := range ws {
		if strings.EqualFold(w.Address, address) {
			return w, nil
		}
	}
	return Wallet{}, fmt.Errorf("%w: %s", ErrWalletNotFound, address)
}

// Reveal returns the plaintext recovery phrase of the wallet at address. The
// Service already holds an unlocked key, so callers that surface the phrase
// to a person must re-verify the password first (CLI: every invocation
// unlocks; TUI: the reveal overlay re-prompts).
func (s *Service) Reveal(address string) (string, error) {
	w, err := s.find(address)
	if err != nil {
		return "", err
	}
	if w.Mnemonic == "" {
		return "", ErrNoMnemonic
	}
	return w.Mnemonic, nil
}

// ExportKeystore writes the wallet at address to outPath as a Web3 Secret
// Storage (keystore V3) JSON file encrypted with passphrase — the format
// MetaMask, geth, Rabby and Frame import. The file is created 0600 and an
// existing file is never overwritten.
func (s *Service) ExportKeystore(address, passphrase, outPath string) error {
	if len(passphrase) < MinKeystorePassphrase {
		return fmt.Errorf("keystore passphrase must be at least %d characters", MinKeystorePassphrase)
	}
	w, err := s.find(address)
	if err != nil {
		return err
	}
	blob, err := encryptKeystore(w.PrivateKey, passphrase)
	if err != nil {
		return err
	}
	return writeNew(outPath, blob)
}

// encryptKeystore turns a hex private key into keystore V3 JSON.
func encryptKeystore(privHex, passphrase string) ([]byte, error) {
	priv, err := ethcrypto.HexToECDSA(strings.TrimPrefix(privHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	key := &keystore.Key{
		Id:         id,
		Address:    ethcrypto.PubkeyToAddress(*priv.Public().(*ecdsa.PublicKey)),
		PrivateKey: priv,
	}
	return keystore.EncryptKey(key, passphrase, keystoreScryptN, keystoreScryptP)
}

// writeNew creates path exclusively with 0600 permissions and writes data.
func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("refusing to overwrite existing file %s", path)
		}
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	return f.Close()
}
