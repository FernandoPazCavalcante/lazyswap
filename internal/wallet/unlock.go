package wallet

import (
	"encoding/hex"
	"errors"

	"github.com/FernandoPazCavalcante/lazyswap-tui/internal/crypto"
)

// ErrNotInitialised means no password has been set yet (salt/sentinel missing).
var ErrNotInitialised = errors.New("encryption not initialised — run the TUI once to set a password")

// ErrInvalidPassword means the password failed the sentinel check.
var ErrInvalidPassword = errors.New("invalid password")

// Unlock derives the AES key from password using the stored salt and verifies
// it against the password sentinel, returning a ready crypto.Service.
//
// This is the non-interactive verify path shared by the TUI login screen and
// the CLI. First-time password creation lives in the login screen.
func Unlock(dao *DAO, password string) (*crypto.Service, error) {
	saltHex, ok, err := dao.GetSalt()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotInitialised
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return nil, err
	}
	key, _, err := crypto.DeriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	svc, err := crypto.New(key)
	if err != nil {
		return nil, err
	}
	sentinel, ok, err := dao.GetSentinel()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotInitialised
	}
	pt, err := svc.Decrypt(sentinel)
	if err != nil || pt != crypto.SentinelPlain {
		return nil, ErrInvalidPassword
	}
	return svc, nil
}
