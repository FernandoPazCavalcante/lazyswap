// Package settings is the single, persisted source of truth for user
// preferences shared by the TUI settings tab and the CLI: slippage tolerance,
// active chain, and default wallet. Stored in the app_config key/value table.
package settings

import (
	"errors"
	"strconv"

	"github.com/FernandoPazCavalcante/lazyswap/internal/chain"
	"github.com/FernandoPazCavalcante/lazyswap/internal/wallet"
)

var (
	errBadSlippage = errors.New("slippage must be between 0 and 100")
	errBadChain    = errors.New("unknown chain")
	errBadSwapMode = errors.New(`swap mode must be "direct" or "api"`)
)

// DefaultSlippage is the slippage percentage applied when none is stored.
const DefaultSlippage = 0.5

const (
	keySlippage      = "setting_slippage"
	keyChain         = "setting_chain"
	keyDefaultWallet = "setting_default_wallet"
	keySwapMode      = "setting_swap_mode"
)

// Swap-mode values. Empty means "auto": api when authenticated and the chain
// supports it, direct otherwise.
const (
	SwapModeDirect = "direct"
	SwapModeAPI    = "api"
)

// Settings mirrors every option in the TUI settings tab and the CLI flags.
type Settings struct {
	Slippage      float64 // percent, 0–100
	ChainKey      string  // a key in chain.CHAINS
	DefaultWallet string  // wallet address; "" when unset
	SwapMode      string  // "direct" | "api" | "" (auto)
}

// Defaults returns the baseline settings used before anything is persisted.
func Defaults() Settings {
	return Settings{Slippage: DefaultSlippage, ChainKey: chain.DefaultKey}
}

// Load reads settings from the config table, falling back to Defaults for any
// missing or invalid value. A nil dao yields Defaults (handy for tests).
func Load(dao *wallet.DAO) (Settings, error) {
	s := Defaults()
	if dao == nil {
		return s, nil
	}
	if v, ok, err := dao.GetConfig(keySlippage); err != nil {
		return s, err
	} else if ok {
		if f, e := strconv.ParseFloat(v, 64); e == nil && f >= 0 && f <= 100 {
			s.Slippage = f
		}
	}
	if v, ok, err := dao.GetConfig(keyChain); err != nil {
		return s, err
	} else if ok && chain.Has(v) {
		s.ChainKey = v
	}
	if v, ok, err := dao.GetConfig(keyDefaultWallet); err != nil {
		return s, err
	} else if ok {
		s.DefaultWallet = v
	}
	if v, ok, err := dao.GetConfig(keySwapMode); err != nil {
		return s, err
	} else if ok && (v == SwapModeDirect || v == SwapModeAPI) {
		s.SwapMode = v
	}
	return s, nil
}

// SetSlippage persists the slippage tolerance (validated 0–100).
func SetSlippage(dao *wallet.DAO, v float64) error {
	if v < 0 || v > 100 {
		return errBadSlippage
	}
	return dao.SetConfig(keySlippage, strconv.FormatFloat(v, 'f', -1, 64))
}

// SetChain persists the active chain (validated against chain.CHAINS).
func SetChain(dao *wallet.DAO, key string) error {
	if !chain.Has(key) {
		return errBadChain
	}
	return dao.SetConfig(keyChain, key)
}

// SetDefaultWallet persists the default wallet address.
func SetDefaultWallet(dao *wallet.DAO, address string) error {
	return dao.SetConfig(keyDefaultWallet, address)
}

// SetSwapMode persists the preferred swap path ("direct" or "api").
func SetSwapMode(dao *wallet.DAO, mode string) error {
	if mode != SwapModeDirect && mode != SwapModeAPI {
		return errBadSwapMode
	}
	return dao.SetConfig(keySwapMode, mode)
}
