package wallet

import (
	"errors"
	"fmt"
	"strings"
)

// Pick resolves which wallet to use: explicit address > configured default >
// the only wallet. Errors when ambiguous or not found.
func Pick(ws []Wallet, explicitAddr, defaultAddr string) (Wallet, error) {
	if len(ws) == 0 {
		return Wallet{}, errors.New("no wallets — create one in the TUI (`lazyswap`)")
	}
	if explicitAddr != "" {
		if w, ok := findByAddress(ws, explicitAddr); ok {
			return w, nil
		}
		return Wallet{}, fmt.Errorf("no wallet with address %s", explicitAddr)
	}
	if defaultAddr != "" {
		if w, ok := findByAddress(ws, defaultAddr); ok {
			return w, nil
		}
		return Wallet{}, fmt.Errorf("default wallet %s not found; set one with `lazyswap config set-wallet`", defaultAddr)
	}
	if len(ws) == 1 {
		return ws[0], nil
	}
	return Wallet{}, errors.New("multiple wallets — set a default (`lazyswap config set-wallet <addr>`) or pass --wallet")
}

func findByAddress(ws []Wallet, addr string) (Wallet, bool) {
	for _, w := range ws {
		if strings.EqualFold(w.Address, addr) {
			return w, true
		}
	}
	return Wallet{}, false
}
